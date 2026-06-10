package api

import (
	"crypto/sha256"
	"encoding/hex"
	"myecho/config/yaml_config"
	"myecho/dal/connect"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"time"

	"myecho/model"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Login(c *fiber.Ctx) error {
	var user model.User
	var res rtype.LoginResponse
	l := new(rtype.LoginRequest)
	if err := c.BodyParser(l); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := validator.ValidateLoginRequest(l); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}

	result := connect.Database
	// 从邮箱登录
	if l.Email != "" {
		result = connect.Database.Where("email = ?", l.Email).First(&user)
	} else {
		// 从用户名登录
		result = connect.Database.Where("name = ?", l.Name).First(&user)
	}
	if result.Error != nil {
		return LoginErrorResponse(c, LoginErrorMsg)
	}
	ok, shouldUpgrade := CheckPassword(user.Password, l.Password)
	if !ok {
		return LoginErrorResponse(c, LoginErrorMsg)
	}
	if shouldUpgrade {
		hashedPassword, err := HashPassword(l.Password)
		if err != nil {
			return InternalErrorResponse(c, InternalSQLError, err.Error())
		}
		user.Password = hashedPassword
	}
	user.LastLogin = time.Now()
	if err := connect.Database.Save(&user).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	res = userToLoginResponse(user)
	return handler.Success(c, res)
}

// 注册
func Register(c *fiber.Ctx) error {
	if !yaml_config.Yaml.APPConfig.AllowRegister {
		return LoginErrorResponse(c, CanNotRegister)
	}
	var r rtype.RegisterRequest
	var res rtype.RegisterResponse
	if err := c.BodyParser(&r); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := validator.ValidateRegisterRequest(&r); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	var user model.User
	structAssign(&user, &r)
	hashedPassword, err := HashPassword(user.Password)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	user.Password = hashedPassword
	user.PermissionType = model.Normal
	// 第一个注册的用户默认为管理员
	if connect.Database.First(&model.User{}).RowsAffected == 0 {
		user.PermissionType = model.Admin
	}
	permissionType := user.PermissionType
	if err := connect.Database.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Select("*").Create(&user).Error; err != nil {
			return err
		}
		return tx.Model(&user).Update("permission_type", permissionType).Error
	}); err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	user.PermissionType = permissionType
	res.LoginResponse = userToLoginResponse(user)
	return handler.Success(c, res)
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashedPassword), err
}

func CheckPassword(storedPassword, password string) (bool, bool) {
	if err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)); err == nil {
		return true, false
	}
	if storedPassword == EncryptPassword(password) {
		return true, true
	}
	return false, false
}

// EncryptPassword returns the legacy broken SHA-256 format kept only for login migration.
func EncryptPassword(password string) string {
	srcByte := []byte(password)
	sha256Cipher := sha256.New()
	sha256Bytes := sha256Cipher.Sum(srcByte)
	sha256String := hex.EncodeToString(sha256Bytes)
	return sha256String
}

func userToLoginResponse(user model.User) rtype.LoginResponse {
	return rtype.LoginResponse{
		Email:          user.Email,
		Name:           user.Name,
		NickName:       user.NickName,
		LastLogin:      user.LastLogin,
		PermissionType: user.PermissionType,
		Token:          user.Token,
	}
}

func LoginErrorResponse(c *fiber.Ctx, msg string) error {
	return ErrorResponse(c, fiber.StatusForbidden, LoginError, msg)
}
