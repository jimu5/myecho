package handler

import "errors"

var (
	// Common
	ErrorIDNotFound       = errors.New("ID not found")
	ErrorInternalNotFound = errors.New("发生了内部错误, 部分内容未找到")
	// Login
	ErrLoginEmailOrNameEmpty = errors.New("登录账号或邮箱为空")
	ErrPasswordEmpty         = errors.New("密码为空")
	ErrNameEmpty             = errors.New("用户名为空")
	ErrEmailEmpty            = errors.New("邮箱为空")
	ErrUserExisted           = errors.New("账号已存在")
	ErrCategoryNotFound      = errors.New("分类不存在")

	// Article
	ErrTitleEmpty   = errors.New("标题为空")
	ErrContentEmpty = errors.New("内容为空")
)
