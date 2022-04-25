package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"myecho/config"
	"myecho/handler/rtype"
	"myecho/handler/validator"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

func Login(c *fiber.Ctx) error {
	var user model.User
	var res rtype.LoginResponse
	l := new(rtype.LoginRequest)
	if err := c.BodyParser(l); err != nil {
		return nil
	}
	if err := validator.ValidateLoginRequest(l); err != nil {
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
	return c.Status(fiber.StatusOK).JSON(res)
}

// 注册
func Register(c *fiber.Ctx) error {
	var r rtype.RegisterRequest
	var res rtype.RegisterResponse
	if err := c.BodyParser(&r); err != nil {
		return nil
	}
	if err := validator.ValidateRegisterRequest(&r); err != nil {
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
	return c.Status(fiber.StatusOK).JSON(res)
}

// sha256加密密码
func EncryptPassword(password string) string {
	srcByte := []byte(password)
	sha256Cipher := sha256.New()
	sha256Bytes := sha256Cipher.Sum(srcByte)
	sha256String := hex.EncodeToString(sha256Bytes)
	return sha256String
}

func LoginErrorResponse(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusForbidden).JSON(Error{Code: LoginError, Msg: msg})
}
