package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"myecho/dal/connect"
	"myecho/model"

	"gorm.io/gorm"
)

const (
	AgentTokenPrefix          = "mye_"
	AgentTokenDefaultExpiry   = 90
	AgentTokenMaxExpiry       = 365
	AgentScopeArticlesRead    = "articles:read"
	AgentScopeArticlesWrite   = "articles:write"
	AgentScopeArticlesPublish = "articles:publish"
	AgentScopeArticlesDelete  = "articles:delete"
	AgentScopeTaxonomyWrite   = "taxonomy:write"
	AgentScopeMediaRead       = "media:read"
	AgentScopeMediaWrite      = "media:write"
	AgentScopeMediaDelete     = "media:delete"
	AgentScopeContentExport   = "content:export"
	AgentScopeContentImport   = "content:import"
	AgentScopeCommentsRead    = "comments:read"
	AgentScopeCommentsWrite   = "comments:write"
	AgentScopeSettingsRead    = "settings:read"
	AgentScopeSettingsWrite   = "settings:write"
)

var (
	ErrInvalidAgentTokenInput = errors.New("invalid agent token input")
	ErrAgentTokenNotFound     = errors.New("agent token not found")
	ErrAgentTokenInvalid      = errors.New("invalid or expired agent token")
)

var agentTokenScopes = []string{
	AgentScopeArticlesRead,
	AgentScopeArticlesWrite,
	AgentScopeArticlesPublish,
	AgentScopeArticlesDelete,
	AgentScopeTaxonomyWrite,
	AgentScopeMediaRead,
	AgentScopeMediaWrite,
	AgentScopeMediaDelete,
	AgentScopeContentExport,
	AgentScopeContentImport,
	AgentScopeCommentsRead,
	AgentScopeCommentsWrite,
	AgentScopeSettingsRead,
	AgentScopeSettingsWrite,
}

func CreateAgentToken(ownerID uint, name string, scopes []string, expiresInDays int) (model.AgentToken, string, error) {
	name = strings.TrimSpace(name)
	if ownerID == 0 || name == "" || utf8.RuneCountInString(name) > 64 || expiresInDays < 0 || expiresInDays > AgentTokenMaxExpiry {
		return model.AgentToken{}, "", ErrInvalidAgentTokenInput
	}
	if expiresInDays == 0 {
		expiresInDays = AgentTokenDefaultExpiry
	}
	validatedScopes, err := normalizeAgentScopes(scopes)
	if err != nil {
		return model.AgentToken{}, "", err
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return model.AgentToken{}, "", err
	}
	rawToken := AgentTokenPrefix + base64.RawURLEncoding.EncodeToString(secret)
	secretHash := sha256.Sum256([]byte(rawToken))
	token := model.AgentToken{
		OwnerID:   ownerID,
		TokenHash: hex.EncodeToString(secretHash[:]),
		Name:      name,
		Scopes:    validatedScopes,
		ExpiresAt: time.Now().AddDate(0, 0, expiresInDays),
	}
	if err := connect.Database.Create(&token).Error; err != nil {
		return model.AgentToken{}, "", err
	}
	return token, rawToken, nil
}

func ListAgentTokens(ownerID uint) ([]model.AgentToken, error) {
	var tokens []model.AgentToken
	if err := connect.Database.Where("owner_id = ?", ownerID).Order("id DESC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

func RevokeAgentToken(ownerID, tokenID uint) (model.AgentToken, error) {
	var token model.AgentToken
	if err := connect.Database.Where("id = ? AND owner_id = ?", tokenID, ownerID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AgentToken{}, ErrAgentTokenNotFound
		}
		return model.AgentToken{}, err
	}
	if token.RevokedAt == nil {
		now := time.Now()
		result := connect.Database.Model(&token).Where("revoked_at IS NULL").Update("revoked_at", now)
		if result.Error != nil {
			return model.AgentToken{}, result.Error
		}
		if result.RowsAffected == 0 {
			if err := connect.Database.First(&token, token.ID).Error; err != nil {
				return model.AgentToken{}, err
			}
		} else {
			token.RevokedAt = &now
		}
	}
	return token, nil
}

func ResolveAgentToken(rawToken string) (model.AgentToken, model.User, error) {
	var token model.AgentToken
	var user model.User
	if !strings.HasPrefix(rawToken, AgentTokenPrefix) || len(rawToken) != len(AgentTokenPrefix)+43 {
		return token, user, ErrAgentTokenInvalid
	}
	secretHash := sha256.Sum256([]byte(rawToken))
	if err := connect.Database.Where("token_hash = ?", hex.EncodeToString(secretHash[:])).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return token, user, ErrAgentTokenInvalid
		}
		return token, user, err
	}
	if token.RevokedAt != nil || !time.Now().Before(token.ExpiresAt) {
		return token, user, ErrAgentTokenInvalid
	}
	if err := connect.Database.First(&user, token.OwnerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return token, user, ErrAgentTokenInvalid
		}
		return token, user, err
	}
	if user.Disabled || !model.IsValidPermissionType(user.PermissionType) {
		return token, user, ErrAgentTokenInvalid
	}
	return token, user, nil
}

func normalizeAgentScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, ErrInvalidAgentTokenInput
	}
	allowed := make(map[string]struct{}, len(agentTokenScopes))
	for _, scope := range agentTokenScopes {
		allowed[scope] = struct{}{}
	}
	seen := make(map[string]struct{}, len(scopes))
	validated := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if _, ok := allowed[scope]; !ok {
			return nil, ErrInvalidAgentTokenInput
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		validated = append(validated, scope)
	}
	return validated, nil
}
