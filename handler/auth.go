package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/Kimiato/myecho/config"
	"github.com/Kimiato/myecho/model"
	"github.com/gofiber/fiber/v2"
)

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

func Login(c *fiber.Ctx) error {
	var user model.User
	var res LoginResponse
	l := new(LoginRequest)
	if err := c.BodyParser(l); err != nil {
		return nil
	}
	if err := validateLoginRequest(l); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	result := config.Database
	// 从邮箱登录
	if l.Email != "" {
		result = config.Database.Where("email = ?", l.Email).First(&user)
	} else {
		// 从用户名登录
		result = config.Database.Where("name = ?", l.Name).First(&user)
	}
	if result.Error != nil {
		return LoginErrorResponse(c, LoginErrorMsg)
	}
	if user.Password != EncryptPassword(l.Password) {
		return LoginErrorResponse(c, LoginErrorMsg)
	}
	user.LastLogin = time.Now()
	config.Database.Save(&user).Scan(&res)
	return c.Status(200).JSON(res)
}

// 注册
func Register(c *fiber.Ctx) error {
	var r RegisterRequest
	var res RegisterResponse
	if err := c.BodyParser(&r); err != nil {
		return nil
	}
	if err := validateRegisterRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	var user model.User
	structAssign(&user, &r)
	user.Password = EncryptPassword(user.Password)
	// 第一个注册的用户默认为管理员
	if config.Database.First(&model.User{}).RowsAffected == 0 {
		user.PermissionType = model.Admin
	}
	config.Database.Model(&model.User{}).Create(&user).Scan(&res)
	return c.Status(200).JSON(res)
}

// sha256加密密码
func EncryptPassword(password string) string {
	srcByte := []byte(password)
	sha256Cipher := sha256.New()
	sha256Bytes := sha256Cipher.Sum(srcByte)
	sha256String := hex.EncodeToString(sha256Bytes)
	return sha256String
}

// 验证请求合法性
func validateLoginRequest(l *LoginRequest) error {
	if l.Email == "" && l.Name == "" {
		return ErrLoginEmailOrNameEmpty
	}
	if l.Password == "" {
		return ErrPasswordEmpty
	}
	return nil
}

func validateRegisterRequest(u *RegisterRequest) error {
	if u.Name == "" {
		return ErrNameEmpty
	}
	if u.NickName == "" {
		u.NickName = u.Name
	}
	if u.Email == "" {
		return ErrEmailEmpty
	}
	if u.Password == "" {
		return ErrPasswordEmpty
	}
	result := config.Database.Where("email = ?", u.Email).Or("name = ?", u.Name).Limit(1).Find(&model.User{})
	if result.RowsAffected > 0 {
		return ErrUserExisted
	}
	return nil
}

func LoginErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(403).JSON(Error{Code: LoginError, Msg: msg})
}
