package api

import (
	"net/mail"
	"strings"
	"unicode/utf8"

	"myecho/dal/connect"
	"myecho/handler"
	apierrors "myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"

	"github.com/gofiber/fiber/v2"
)

func Profile(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return UnauthorizedErrorResponse(c)
	}
	return handler.Success(c, userToLoginResponse(*user))
}

func ProfileUpdate(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return UnauthorizedErrorResponse(c)
	}
	var req rtype.ProfileUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	req.NickName = strings.TrimSpace(req.NickName)
	req.Email = strings.TrimSpace(req.Email)
	address, err := mail.ParseAddress(req.Email)
	if req.NickName == "" ||
		utf8.RuneCountInString(req.NickName) > 64 ||
		utf8.RuneCountInString(req.Email) > 64 ||
		err != nil ||
		address.Address != req.Email {
		return ValidateErrorResponse(c, apierrors.ErrInvalidParams.Error())
	}
	var count int64
	if err := connect.Database.Model(&model.User{}).
		Where("email = ? AND id <> ?", req.Email, user.ID).
		Count(&count).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	if count > 0 {
		return ValidateErrorResponse(c, apierrors.ErrUserExisted.Error())
	}
	user.NickName = req.NickName
	user.Email = req.Email
	if err := connect.Database.Save(user).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, userToLoginResponse(*user))
}

func PasswordUpdate(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return UnauthorizedErrorResponse(c)
	}
	var req rtype.PasswordUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if utf8.RuneCountInString(req.NewPassword) < 8 || len(req.NewPassword) > 72 {
		return ValidateErrorResponse(c, "新密码需为 8 个字符以上且不超过 72 字节")
	}
	if ok, _ := CheckPassword(user.Password, req.OldPassword); !ok {
		return ValidateErrorResponse(c, "旧密码错误")
	}
	hashedPassword, err := HashPassword(req.NewPassword)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	user.Password = hashedPassword
	user.Token = model.GenerateRandomString(32)
	if err := connect.Database.Save(user).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, userToLoginResponse(*user))
}

func Logout(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*model.User)
	if !ok || user == nil {
		return UnauthorizedErrorResponse(c)
	}
	token := model.GenerateRandomString(32)
	if err := connect.Database.Model(user).
		Where("token = ?", user.Token).
		Update("token", token).Error; err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, fiber.Map{"logged_out": true})
}
