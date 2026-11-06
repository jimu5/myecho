package service

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"myecho/dal/connect"
	"myecho/model"
)

func EncodeArticleSubmissionSnapshot(snapshot model.ArticleSubmissionSnapshot) (string, error) {
	data, err := json.Marshal(snapshot)
	return string(data), err
}

func DecodeArticleSubmissionSnapshot(submission *model.ArticleSubmission) (model.ArticleSubmissionSnapshot, error) {
	var snapshot model.ArticleSubmissionSnapshot
	if submission == nil {
		return snapshot, errors.New("submission is required")
	}
	err := json.Unmarshal([]byte(submission.Snapshot), &snapshot)
	return snapshot, err
}

func LatestEditableSubmission(articleID uint) (*model.ArticleSubmission, error) {
	var submission model.ArticleSubmission
	err := connect.Database.Preload("Submitter").Preload("Reviewer").
		Where("article_id = ? AND status IN ?", articleID, []model.ArticleSubmissionStatus{
			model.ArticleSubmissionPending, model.ArticleSubmissionDraft, model.ArticleSubmissionRejected,
		}).Order("id desc").First(&submission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &submission, err
}

func LatestEditableSubmissionForSubmitter(articleID, submitterID uint) (*model.ArticleSubmission, error) {
	var submission model.ArticleSubmission
	err := connect.Database.Preload("Submitter").Preload("Reviewer").
		Where("article_id = ? AND submitter_id = ? AND status IN ?", articleID, submitterID, []model.ArticleSubmissionStatus{
			model.ArticleSubmissionPending, model.ArticleSubmissionDraft, model.ArticleSubmissionRejected,
		}).Order("id desc").First(&submission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &submission, err
}

func PendingArticleSubmission(articleID uint) (*model.ArticleSubmission, error) {
	var submission model.ArticleSubmission
	err := connect.Database.Preload("Submitter").
		Where("article_id = ? AND status = ?", articleID, model.ArticleSubmissionPending).
		Order("id desc").First(&submission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &submission, err
}
