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
	BaseModel
	Name           string `json:"name" gorm:"size:64"`
	NickName       string `json:"nick_name" gorm:"size:64"`
	Email          string `json:"email" gorm:"size:64"`
	LastLogin      time.Time
	PermissionType int8   `gorm:"default:1" json:"permission_type"`
	Password       string `json:"-" gorm:"size:128"`
	Token          string `json:"-" gorm:"size:32"`
}

// 生成随机字符串
func GenerateRandomString(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte
	for i := 0; i < length; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

// 生成token
func (u *User) generateToken() {
	if u.Token == "" {
		u.Token = GenerateRandomString(32)
	}
}

func (u *User) BeforeSave(tx *gorm.DB) error {
	u.generateToken()
	return nil
}
