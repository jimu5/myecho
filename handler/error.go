package handler

import "errors"

var (
	ErrLoginEmailOrNameEmpty = errors.New("登录账号或邮箱为空")
	ErrPasswordEmpty         = errors.New("密码为空")
	ErrNameEmpty             = errors.New("用户名为空")
	ErrEmailEmpty            = errors.New("邮箱为空")
	ErrUserExisted           = errors.New("账号已存在")
)
