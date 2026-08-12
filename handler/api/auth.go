package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"myecho/dal/connect"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"sync"
	"time"

	"myecho/model"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	errSetupCompleted = errors.New("setup already completed")
	// ponytail: process-wide lock is enough for the single-instance deployment; add a DB-level lock if clustered setup is supported.
	setupMu sync.Mutex
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

// Register is kept for API compatibility; initial account creation only happens through Setup.
func Register(c *fiber.Ctx) error {
	return LoginErrorResponse(c, CanNotRegister)
}

func needsInitialSetup(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func createInitialAdmin(tx *gorm.DB, req rtype.RegisterRequest, hashedPassword string) (model.User, error) {
	needsSetup, err := needsInitialSetup(tx)
	if err != nil {
		return model.User{}, err
	}
	if !needsSetup {
		return model.User{}, errSetupCompleted
	}
	user := model.User{
		Name:           req.Name,
		NickName:       req.NickName,
		Email:          req.Email,
		Password:       hashedPassword,
		PermissionType: model.Admin,
	}
	if user.NickName == "" {
		user.NickName = user.Name
	}
	if err := tx.Select("*").Create(&user).Error; err != nil {
		return model.User{}, err
	}
	if err := tx.Model(&user).Update("permission_type", model.Admin).Error; err != nil {
		return model.User{}, err
	}
	user.PermissionType = model.Admin
	return user, nil
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
