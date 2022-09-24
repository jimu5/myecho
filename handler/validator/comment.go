package validator

import (
	"myecho/dal/connect"
	"myecho/handler/errors"
	"myecho/handler/rtype"
	"myecho/model"
)

// 验证评论请求
func ValidateCommentRequest(l *rtype.CommentRequest) error {
	if l.AuthorName == "" {
		return errors.ErrCommentAuthorNameEmpty
	}
	if l.AuthorEmail == "" {
		return errors.ErrCommentAuthorEmailEmpty
	}
	if l.Content == "" {
		return errors.ErrCommentContentEmpty
	}
	err := ValidateParentCommentID(l.ParentID)
	if err != nil {
		return err
	}
	return nil
}

func ValidateArticleID(id uint) error {
	if id == 0 {
		return errors.ErrArticleID
	}
	result := connect.Database.First(&model.Article{}, id)
	if result.Error != nil {
		return errors.ErrArticleID
	}
	return nil
}

func ValidateParentCommentID(id uint) error {
	if id == 0 {
		return nil
	}
	result := connect.Database.First(&model.Comment{}, id)
	if result.Error != nil {
		return errors.ErrParentCommentID
	}
	return nil
}
