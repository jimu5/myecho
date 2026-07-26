package validator

import (
	"myecho/dal/connect"
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// 验证请求合法性
func ValidateLoginRequest(l *rtype.LoginRequest) error {
	if l.Email == "" && l.Name == "" {
		return errors.ErrLoginEmailOrNameEmpty
	}
	if l.Password == "" {
		return errors.ErrPasswordEmpty
	}
	return nil
}

func ValidateRegisterRequest(u *rtype.RegisterRequest) error {
	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.TrimSpace(u.Email)
	if u.Name == "" {
		return errors.ErrNameEmpty
	}
	if u.NickName == "" {
		u.NickName = u.Name
	}
	if u.Email == "" {
		return errors.ErrEmailEmpty
	}
	address, err := mail.ParseAddress(u.Email)
	if err != nil || address.Address != u.Email {
		return errors.ErrEmailEmpty
	}
	if u.Password == "" {
		return errors.ErrPasswordEmpty
	}
	result := connect.Database.Where("email = ?", u.Email).Or("name = ?", u.Name).Limit(1).Find(&model.User{})
	if result.RowsAffected > 0 {
		return errors.ErrUserExisted
	}
	return nil
}

func ValidateSetupRequest(req *rtype.SetupRequest) error {
	req.SiteTitle = strings.TrimSpace(req.SiteTitle)
	req.SiteDescription = strings.TrimSpace(req.SiteDescription)
	if req.SiteTitle == "" {
		return errors.ErrInvalidParams
	}
	registerReq := rtype.RegisterRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	}
	if err := ValidateRegisterRequest(&registerReq); err != nil {
		return err
	}
	if utf8.RuneCountInString(req.Password) < 8 {
		return errors.ErrInvalidParams
	}
	req.Name = registerReq.Name
	req.Email = registerReq.Email
	return nil
}
