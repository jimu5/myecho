package rtype

import "time"

type LoginRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email          string `json:"email"`
	Name           string `json:"name"`
	Password       string `json:"password"`
	NickName       string `json:"nick_name"`
	PermissionType int8   `json:"-"`
}

type LoginResponse struct {
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	NickName       string    `json:"nick_name"`
	LastLogin      time.Time `json:"last_login"`
	PermissionType int8      `json:"permission_type"`
	Token          string    `json:"token"`
}

type RegisterResponse struct {
	LoginResponse
}
