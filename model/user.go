package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	Admin = iota
	Normal
)

type User struct {
	gorm.Model
	Name           string `json:"name"`
	Email          string `json:"email"`
	LastLogin      time.Time
	PermissionType int8
	Password       string `json:"password"`
	Token          string `json:"token"`
}
