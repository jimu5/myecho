package mysql

import (
	"errors"

	"gorm.io/gorm"
	"myecho/model"
)

// FindLatestDraftOrRejectedSubmission returns the latest private snapshot for
// the given submitter. Pending submissions are deliberately excluded so they
// cannot hide an editable draft or rejected snapshot.
func (a *ArticleDBRepo) FindLatestDraftOrRejectedSubmission(articleID, submitterID uint) (*model.ArticleSubmission, error) {
	var submission model.ArticleSubmission
	err := db.Where("article_id = ? AND submitter_id = ? AND status IN ?", articleID, submitterID, []model.ArticleSubmissionStatus{
		model.ArticleSubmissionDraft,
		model.ArticleSubmissionRejected,
	}).Order("id desc").First(&submission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &submission, err
}

// FindTagsByUIDs loads existing tags referenced by a saved article snapshot.
func (a *ArticleDBRepo) FindTagsByUIDs(tagUIDs []string) ([]*model.Tag, error) {
	tags := make([]*model.Tag, 0, len(tagUIDs))
	if len(tagUIDs) == 0 {
		return tags, nil
	}
	err := db.Where("uid IN ?", tagUIDs).Find(&tags).Error
	return tags, err
}
