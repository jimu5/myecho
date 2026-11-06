package mysql

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
	"myecho/dal/connect"
	"myecho/model"
)

func TestArticlePreviewRepoLookupAndRevokeScope(t *testing.T) {
	setupMysqlRepoTestDB(t)
	if err := connect.Database.AutoMigrate(&model.ArticlePreview{}); err != nil {
		t.Fatalf("migrate preview grants: %v", err)
	}
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Minute)
	grants := []model.ArticlePreview{
		{ArticleID: 41, CreatorID: 7, TokenHash: "active-current-creator", Snapshot: `{}`, ExpiresAt: now.Add(time.Hour)},
		{ArticleID: 41, CreatorID: 7, TokenHash: "expired-current-creator", Snapshot: `{}`, ExpiresAt: now.Add(-time.Second)},
		{ArticleID: 41, CreatorID: 7, TokenHash: "revoked-current-creator", Snapshot: `{}`, ExpiresAt: now.Add(time.Hour), RevokedAt: &revokedAt},
		{ArticleID: 42, CreatorID: 7, TokenHash: "active-other-article", Snapshot: `{}`, ExpiresAt: now.Add(time.Hour)},
		{ArticleID: 41, CreatorID: 8, TokenHash: "active-other-creator", Snapshot: `{}`, ExpiresAt: now.Add(time.Hour)},
	}
	if err := connect.Database.Create(&grants).Error; err != nil {
		t.Fatalf("create preview grants: %v", err)
	}
	repo := &ArticlePreviewDBRepo{}
	active, err := repo.FindValidByTokenHash("active-current-creator", now)
	if err != nil || active == nil || active.TokenHash != grants[0].TokenHash {
		t.Fatalf("FindValidByTokenHash(active) = %+v, %v", active, err)
	}
	for _, hash := range []string{"expired-current-creator", "revoked-current-creator", "missing"} {
		if _, err := repo.FindValidByTokenHash(hash, now); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("FindValidByTokenHash(%q) error = %v, want not found", hash, err)
		}
	}

	if err := repo.RevokeForArticleCreator(41, 7, now); err != nil {
		t.Fatalf("RevokeForArticleCreator() error = %v", err)
	}
	for i, wantRevoked := range []bool{true, true, true, false, false} {
		var stored model.ArticlePreview
		if err := connect.Database.First(&stored, grants[i].ID).Error; err != nil {
			t.Fatalf("reload grant %q: %v", grants[i].TokenHash, err)
		}
		if (stored.RevokedAt != nil) != wantRevoked {
			t.Errorf("grant %q revoked = %v, want %v", stored.TokenHash, stored.RevokedAt != nil, wantRevoked)
		}
	}
	if _, err := repo.FindValidByTokenHash("active-current-creator", now); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("revoked token lookup error = %v, want not found", err)
	}
	for _, hash := range []string{"active-other-article", "active-other-creator"} {
		if _, err := repo.FindValidByTokenHash(hash, now); err != nil {
			t.Errorf("scoped revoke invalidated %q: %v", hash, err)
		}
	}
}
