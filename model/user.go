package model

import (
	"math/rand"
	"time"

	"gorm.io/gorm"
)

const (
	Admin = iota
	Normal
)

type User struct {
	gorm.Model
	Name           string `json:"name" gorm:"size:64"`
	Email          string `json:"email" gorm:"size:64"`
	LastLogin      time.Time
	PermissionType int8   `gorm:"default:1" json:"permission_type"`
	Password       string `json:"password" gorm:"size:128"`
	Token          string `json:"token" gorm:"size:32"`
}

// 生成随机字符串
func GenerateRandomString(length int) string {
	var result string
	for i := 0; i < length; i++ {
		result += string(rune(65 + rand.Intn(25)))
	}
	return result
}

// 生成token
func (u *User) GenerateToken() {
	if u.Token == "" {
		u.Token = GenerateRandomString(32)
	}
}

func (u *User) BeforeSave(tx *gorm.DB) error {
	u.GenerateToken()
	return nil
}
