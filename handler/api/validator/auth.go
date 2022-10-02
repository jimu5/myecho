package validator

import (
	"myecho/dal/connect"
	"myecho/handler/api/errors"
	"myecho/handler/api/rtype"
	"myecho/model"
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
	if u.Name == "" {
		return errors.ErrNameEmpty
	}
	if u.NickName == "" {
		u.NickName = u.Name
	}
	if u.Email == "" {
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
