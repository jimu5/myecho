package model

import "time"

type AgentToken struct {
	BaseModel
	OwnerID   uint       `json:"owner_id" gorm:"not null;index"`
	TokenHash string     `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Name      string     `json:"name" gorm:"size:64;not null"`
	Scopes    []string   `json:"scopes" gorm:"serializer:json;type:text"`
	ExpiresAt time.Time  `json:"expires_at" gorm:"not null;index"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" gorm:"index"`
}
