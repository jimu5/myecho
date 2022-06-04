package errors

import errs "github.com/pkg/errors"

var (
	// Common
	ErrorIDNotFound       = errs.New("ID not found")
	ErrorInternalNotFound = errs.New("发生了内部错误, 部分内容未找到")
	// Login
	ErrLoginEmailOrNameEmpty = errs.New("登录账号或邮箱为空")
	ErrPasswordEmpty         = errs.New("密码为空")
	ErrNameEmpty             = errs.New("用户名为空")
	ErrEmailEmpty            = errs.New("邮箱为空")
	ErrUserExisted           = errs.New("账号已存在")

	// Article
	ErrTitleEmpty   = errs.New("标题为空")
	ErrContentEmpty = errs.New("内容为空")

	// Comment
	ErrCommentAuthorNameEmpty  = errs.New("评论者名称为空")
	ErrCommentAuthorEmailEmpty = errs.New("评论者邮箱为空")
	ErrCommentContentEmpty     = errs.New("评论内容为空")
	ErrCommentArticleIDEmpty   = errs.New("文章ID为空")
	ErrArticleID               = errs.New("文章ID不存在")
	ErrParentCommentID         = errs.New("父级评论ID错误")

	// Category
	ErrCategoryNameEmpty = errs.New("category name 为空")
	ErrCategoryNotFound  = errs.New("分类不存在")
)
