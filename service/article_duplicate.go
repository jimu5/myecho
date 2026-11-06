package service

import (
	"time"

	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/model"
)

// DuplicateAsDraft creates a new draft from an article's live content, or from
// the actor's latest editable draft/rejected snapshot when the actor owns it.
func (a *ArticleService) DuplicateAsDraft(articleID, actorID uint) (mysql.ArticleModel, error) {
	source, err := dal.MySqlDB.Article.FindByID(articleID)
	if err != nil {
		return mysql.ArticleModel{}, err
	}
	if source.Status == int8(mysql.ARTICLE_STATUS_RECYCLE) {
		return mysql.ArticleModel{}, mysql.ErrArticleTrashed
	}

	title := source.Title
	seoTitle := source.SEOTitle
	seoDescription := source.SEODescription
	shareImage := source.ShareImage
	articleType := source.Type
	contentFormat := source.ContentFormat
	summary := source.Summary
	content := ""
	if source.Detail != nil {
		content = source.Detail.Content
	}
	categoryUID := source.CategoryUID
	allowComment := cloneDuplicateBool(source.IsAllowComment)
	tags := source.Tags

	if source.AuthorID == actorID {
		submission, err := dal.MySqlDB.Article.FindLatestDraftOrRejectedSubmission(source.ID, actorID)
		if err != nil {
			return mysql.ArticleModel{}, err
		}
		if submission != nil {
			snapshot, err := DecodeArticleSubmissionSnapshot(submission)
			if err != nil {
				return mysql.ArticleModel{}, err
			}
			title = snapshot.Title
			seoTitle = snapshot.SEOTitle
			seoDescription = snapshot.SEODescription
			shareImage = snapshot.ShareImage
			articleType = snapshot.Type
			contentFormat = snapshot.ContentFormat
			summary = snapshot.Summary
			content = snapshot.Content
			categoryUID = snapshot.CategoryUID
			allowComment = cloneDuplicateBool(snapshot.IsAllowComment)
			tags, err = dal.MySqlDB.Article.FindTagsByUIDs(snapshot.TagUIDs)
			if err != nil {
				return mysql.ArticleModel{}, err
			}
		}
	}

	duplicate := mysql.ArticleModel{
		AuthorID:       actorID,
		Title:          title,
		SEOTitle:       seoTitle,
		SEODescription: seoDescription,
		ShareImage:     shareImage,
		Type:           articleType,
		ContentFormat:  model.NormalizeArticleContentFormat(contentFormat),
		Summary:        summary,
		EditVersion:    1,
		IsAllowComment: allowComment,
		CategoryUID:    categoryUID,
		PostTime:       time.Now(),
		Status:         int8(mysql.ARTICLE_STATUS_DRAFT),
		Detail:         &model.ArticleDetail{Content: content},
		Tags:           tags,
	}
	if err := dal.MySqlDB.Article.Create(&duplicate); err != nil {
		return mysql.ArticleModel{}, err
	}
	return dal.MySqlDB.Article.FindByID(duplicate.ID)
}

func cloneDuplicateBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
