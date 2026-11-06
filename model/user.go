package model

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

const (
	Admin int8 = iota
	Author
	Editor
)

// Normal is kept for API and database compatibility with earlier releases.
const Normal = Author

const AuthTokenTTL = 30 * 24 * time.Hour

type User struct {
	BaseModel
	Name           string `json:"name" gorm:"size:64"`
	NickName       string `json:"nick_name" gorm:"size:64"`
	Email          string `json:"email" gorm:"size:64"`
	Slug           string `json:"slug" gorm:"size:160;index"`
	AvatarURL      string `json:"avatar_url" gorm:"size:512"`
	LastLogin      time.Time
	PermissionType int8      `gorm:"default:1" json:"permission_type"`
	CanPublish     bool      `json:"can_publish" gorm:"default:false"`
	Password       string    `json:"-" gorm:"size:128"`
	Token          string    `json:"-" gorm:"size:32"`
	TokenExpiresAt time.Time `json:"token_expires_at"`
	Disabled       bool      `json:"disabled" gorm:"default:false"`
}

func IsValidPermissionType(permissionType int8) bool {
	return permissionType == Admin || permissionType == Author || permissionType == Editor
}

func (u *User) CanEditAllContent() bool {
	return u != nil && (u.PermissionType == Admin || u.PermissionType == Editor)
}

func (u *User) CanPublishContent() bool {
	return u != nil && (u.CanEditAllContent() || (u.PermissionType == Author && u.CanPublish))
}

func (u *User) RotateToken(now time.Time) {
	u.Token = GenerateRandomString(32)
	u.TokenExpiresAt = now.Add(AuthTokenTTL)
}

// 生成随机字符串
func GenerateRandomString(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte
	for len(result) < length {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(bytes))))
		if err != nil {
			return string(result)
		}
		result = append(result, bytes[index.Int64()])
	}
	return string(result)
}

// 生成token
func (u *User) generateToken() {
	if u.Token == "" {
		u.RotateToken(time.Now())
	}
}

func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.Slug == "" {
		u.Slug = NormalizeUserSlug(u.Name)
	}
	u.generateToken()
	return nil
}

func NormalizeUserSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	lastDash := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			result.WriteRune(r)
			lastDash = false
		} else if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}

func (u *User) PublicSlug() string {
	if u == nil {
		return ""
	}
	if slug := NormalizeUserSlug(u.Slug); slug != "" {
		return slug
	}
	return NormalizeUserSlug(u.Name)
}
