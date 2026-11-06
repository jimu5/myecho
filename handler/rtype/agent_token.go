package rtype

import (
	"time"

	"myecho/model"
)

type AgentTokenCreateRequest struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays int      `json:"expires_in_days"`
}

type AgentTokenResponse struct {
	ID        uint       `json:"id"`
	OwnerID   uint       `json:"owner_id"`
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	Token     string     `json:"token,omitempty"`
}

func ModelToAgentTokenResponse(token model.AgentToken, rawToken string) AgentTokenResponse {
	return AgentTokenResponse{
		ID:        token.ID,
		OwnerID:   token.OwnerID,
		Name:      token.Name,
		Scopes:    append([]string(nil), token.Scopes...),
		ExpiresAt: token.ExpiresAt,
		RevokedAt: token.RevokedAt,
		CreatedAt: token.CreatedAt,
		Token:     rawToken,
	}
}
