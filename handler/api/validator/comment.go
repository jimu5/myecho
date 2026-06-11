package validator

import (
	"myecho/dal/connect"
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"
	"net/mail"
	"strings"
)

// 验证评论请求
func ValidateCommentRequest(l *rtype.CommentRequest) error {
	if l.AuthorName == "" {
		return errors.ErrCommentAuthorNameEmpty
	}
	if l.AuthorEmail == "" {
		return errors.ErrCommentAuthorEmailEmpty
	}
	if _, err := mail.ParseAddress(l.AuthorEmail); err != nil {
		return errors.ErrCommentAuthorEmailEmpty
	}
	if l.Content == "" {
		return errors.ErrCommentContentEmpty
	}
	if len([]rune(strings.TrimSpace(l.AuthorName))) > 64 || len([]rune(strings.TrimSpace(l.AuthorEmail))) > 64 || len([]rune(strings.TrimSpace(l.AuthorUrl))) > 256 || len([]rune(strings.TrimSpace(l.Content))) > 2000 {
		return errors.ErrInvalidParams
	}
	err := ValidateParentCommentID(l.ParentID)
	if err != nil {
		return err
	}
	return nil
}

func ValidateParentCommentForArticle(parentID uint, articleUID string) error {
	if parentID == 0 {
		return nil
	}
	var comment model.Comment
	if err := connect.Database.First(&comment, parentID).Error; err != nil {
		return errors.ErrParentCommentID
	}
	if comment.ArticleUID != articleUID {
		return errors.ErrParentCommentID
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

func ValidateCommentStatus(status model.CommentStatus) error {
	if !model.IsValidCommentStatus(status) {
		return errors.ErrCommentStatus
	}
	return nil
}
