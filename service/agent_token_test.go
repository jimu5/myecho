package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/model"
)

func setupAgentTokenServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "agent-token.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.AgentToken{}); err != nil {
		t.Fatalf("migrate agent token tables: %v", err)
	}
	previousDB := connect.Database
	connect.Database = db
	t.Cleanup(func() {
		connect.Database = previousDB
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createAgentTokenTestUser(t *testing.T, db *gorm.DB, name string, role int8) *model.User {
	t.Helper()
	user := &model.User{Name: name, Email: name + "@example.com", Token: "login-" + name, PermissionType: role}
	if err := db.Select("*").Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Model(user).Update("permission_type", role).Error; err != nil {
		t.Fatalf("set user role: %v", err)
	}
	return user
}

func TestAgentTokenStoresOnlyHashAndSurvivesLoginRotation(t *testing.T) {
	db := setupAgentTokenServiceDB(t)
	owner := createAgentTokenTestUser(t, db, "owner", model.Editor)
	createdAfter := time.Now()
	token, raw, err := CreateAgentToken(owner.ID, "nightly import", []string{AgentScopeArticlesWrite}, 0)
	if err != nil {
		t.Fatalf("CreateAgentToken() error = %v", err)
	}
	if len(raw) != len(AgentTokenPrefix)+43 || raw[:len(AgentTokenPrefix)] != AgentTokenPrefix {
		t.Fatalf("raw token = %q, want a %s token with 32-byte base64url secret", raw, AgentTokenPrefix)
	}
	if token.TokenHash == raw {
		t.Fatal("stored token hash contains the raw credential")
	}
	hash := sha256.Sum256([]byte(raw))
	if token.TokenHash != hex.EncodeToString(hash[:]) {
		t.Fatalf("stored hash = %q, want SHA-256 hash", token.TokenHash)
	}
	if !token.ExpiresAt.After(createdAfter.AddDate(0, 0, 89)) || token.ExpiresAt.After(time.Now().AddDate(0, 0, 91)) {
		t.Fatalf("default expiration = %s, want about 90 days from now", token.ExpiresAt)
	}

	var stored model.AgentToken
	if err := db.First(&stored, token.ID).Error; err != nil {
		t.Fatalf("load stored token: %v", err)
	}
	if len(stored.Scopes) != 1 || stored.Scopes[0] != AgentScopeArticlesWrite {
		t.Fatalf("stored scopes = %#v", stored.Scopes)
	}
	if err := db.Model(owner).Update("token", "new-login-token").Error; err != nil {
		t.Fatalf("rotate login token: %v", err)
	}
	resolved, currentOwner, err := ResolveAgentToken(raw)
	if err != nil {
		t.Fatalf("ResolveAgentToken() after login rotation: %v", err)
	}
	if resolved.ID != token.ID || currentOwner.ID != owner.ID || currentOwner.Token != "new-login-token" {
		t.Fatalf("resolved token/user = (%d, %d, %q)", resolved.ID, currentOwner.ID, currentOwner.Token)
	}
}

func TestAgentTokenRejectsUnknownScopesAndInvalidExpiry(t *testing.T) {
	db := setupAgentTokenServiceDB(t)
	owner := createAgentTokenTestUser(t, db, "owner", model.Admin)
	for _, expiresInDays := range []int{-1, AgentTokenMaxExpiry + 1} {
		if _, _, err := CreateAgentToken(owner.ID, "bad", []string{AgentScopeArticlesRead}, expiresInDays); !errors.Is(err, ErrInvalidAgentTokenInput) {
			t.Errorf("CreateAgentToken(expiry=%d) error = %v, want invalid input", expiresInDays, err)
		}
	}
	if _, _, err := CreateAgentToken(owner.ID, "bad", []string{"admin:*"}, 90); !errors.Is(err, ErrInvalidAgentTokenInput) {
		t.Fatalf("CreateAgentToken(unknown scope) error = %v, want invalid input", err)
	}
	var count int64
	if err := db.Model(&model.AgentToken{}).Count(&count).Error; err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("stored %d invalid credentials", count)
	}
}

func TestAgentTokenRevocationIsOwnerScopedAndRechecked(t *testing.T) {
	db := setupAgentTokenServiceDB(t)
	owner := createAgentTokenTestUser(t, db, "owner", model.Author)
	other := createAgentTokenTestUser(t, db, "other", model.Admin)
	token, raw, err := CreateAgentToken(owner.ID, "automation", []string{AgentScopeArticlesRead}, 3)
	if err != nil {
		t.Fatalf("CreateAgentToken() error = %v", err)
	}
	if _, err := RevokeAgentToken(other.ID, token.ID); !errors.Is(err, ErrAgentTokenNotFound) {
		t.Fatalf("RevokeAgentToken(other owner) error = %v, want not found", err)
	}
	revoked, err := RevokeAgentToken(owner.ID, token.ID)
	if err != nil {
		t.Fatalf("RevokeAgentToken(owner) error = %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("revoked credential has no revoked_at")
	}
	if _, _, err := ResolveAgentToken(raw); !errors.Is(err, ErrAgentTokenInvalid) {
		t.Fatalf("ResolveAgentToken(revoked) error = %v, want invalid", err)
	}
	repeated, err := RevokeAgentToken(owner.ID, token.ID)
	if err != nil || repeated.RevokedAt == nil || !repeated.RevokedAt.Equal(*revoked.RevokedAt) {
		t.Fatalf("repeated revoke = (%v, %v), want idempotent original revocation", repeated.RevokedAt, err)
	}

	if err := db.Model(owner).Update("disabled", true).Error; err != nil {
		t.Fatalf("disable owner: %v", err)
	}
	if _, _, err := ResolveAgentToken(raw); !errors.Is(err, ErrAgentTokenInvalid) {
		t.Fatalf("ResolveAgentToken(disabled owner) error = %v, want invalid", err)
	}
}

func TestAgentTokenExpiryAndRoleAreResolvedFresh(t *testing.T) {
	db := setupAgentTokenServiceDB(t)
	owner := createAgentTokenTestUser(t, db, "owner", model.Editor)
	token, raw, err := CreateAgentToken(owner.ID, "automation", []string{AgentScopeArticlesRead}, 90)
	if err != nil {
		t.Fatalf("CreateAgentToken() error = %v", err)
	}
	if err := db.Model(&model.AgentToken{}).Where("id = ?", token.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expire credential: %v", err)
	}
	if _, _, err := ResolveAgentToken(raw); !errors.Is(err, ErrAgentTokenInvalid) {
		t.Fatalf("ResolveAgentToken(expired) error = %v, want invalid", err)
	}
	if err := db.Model(&model.AgentToken{}).Where("id = ?", token.ID).Update("expires_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("restore credential expiry: %v", err)
	}
	if err := db.Model(owner).Update("permission_type", model.Author).Error; err != nil {
		t.Fatalf("change owner role: %v", err)
	}
	_, resolvedOwner, err := ResolveAgentToken(raw)
	if err != nil {
		t.Fatalf("ResolveAgentToken(after role change) error = %v", err)
	}
	if resolvedOwner.PermissionType != model.Author {
		t.Fatalf("resolved role = %d, want fresh author role", resolvedOwner.PermissionType)
	}
	if err := db.Model(owner).Update("disabled", true).Error; err != nil {
		t.Fatalf("disable owner: %v", err)
	}
	if _, _, err := ResolveAgentToken(raw); !errors.Is(err, ErrAgentTokenInvalid) {
		t.Fatalf("ResolveAgentToken(disabled owner) error = %v, want invalid", err)
	}
}
