package mysql

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"myecho/model"
)

// ArticlePreviewDBRepo stores only hashed preview credentials and their frozen
// display snapshots.
type ArticlePreviewDBRepo struct{}

var ErrArticlePreviewNotManageable = errors.New("article preview creator no longer manages article")

type ArticlePreviewCreateSource struct {
	Article    ArticleModel
	Creator    model.User
	Submission *model.ArticleSubmission
}

func (r *ArticlePreviewDBRepo) LoadCreateSource(tx *gorm.DB, articleID, creatorID uint) (*ArticlePreviewCreateSource, error) {
	var source ArticlePreviewCreateSource
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload(clause.Associations).
		Where("id = ?", articleID).First(&source.Article).Error
	if err != nil {
		return nil, err
	}
	if source.Article.Status == int8(ARTICLE_STATUS_RECYCLE) {
		return nil, ErrArticleTrashed
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "permission_type", "disabled").Where("id = ?", creatorID).
		First(&source.Creator).Error; err != nil {
		return nil, err
	}
	if !articlePreviewCreatorCanManage(&source.Creator, &source.Article) {
		return nil, ErrArticlePreviewNotManageable
	}
	var submission model.ArticleSubmission
	err = tx.Where("article_id = ? AND submitter_id = ? AND status IN ?", articleID, creatorID, []model.ArticleSubmissionStatus{
		model.ArticleSubmissionPending,
		model.ArticleSubmissionDraft,
		model.ArticleSubmissionRejected,
	}).Order("id desc").First(&submission).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil {
		source.Submission = &submission
	}
	return &source, nil
}

func (r *ArticlePreviewDBRepo) LoadAccessSource(tx *gorm.DB, articleID, creatorID uint) (ArticleModel, model.User, error) {
	var article ArticleModel
	if err := tx.Select("id", "author_id", "status").Where("id = ?", articleID).First(&article).Error; err != nil {
		return ArticleModel{}, model.User{}, err
	}
	if article.Status == int8(ARTICLE_STATUS_RECYCLE) {
		return ArticleModel{}, model.User{}, ErrArticleTrashed
	}
	var creator model.User
	if err := tx.Select("id", "permission_type", "disabled").Where("id = ?", creatorID).First(&creator).Error; err != nil {
		return ArticleModel{}, model.User{}, err
	}
	if !articlePreviewCreatorCanManage(&creator, &article) {
		return ArticleModel{}, model.User{}, ErrArticlePreviewNotManageable
	}
	return article, creator, nil
}

func (r *ArticlePreviewDBRepo) LoadTaxonomy(tx *gorm.DB, categoryUID string, tagUIDs []string) (string, []model.ArticlePreviewTag, error) {
	categoryName := ""
	if categoryUID != "" {
		var category model.Category
		err := tx.Select("uid", "name").Where("uid = ?", categoryUID).First(&category).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, err
		}
		if err == nil {
			categoryName = category.Name
		}
	}
	tags := make([]model.ArticlePreviewTag, 0, len(tagUIDs))
	if len(tagUIDs) == 0 {
		return categoryName, tags, nil
	}
	var records []model.Tag
	if err := tx.Select("uid", "name").Where("uid IN ?", tagUIDs).Find(&records).Error; err != nil {
		return "", nil, err
	}
	tagNames := make(map[string]string, len(records))
	for _, tag := range records {
		tagNames[tag.UID] = tag.Name
	}
	for _, uid := range tagUIDs {
		if name, ok := tagNames[uid]; ok {
			tags = append(tags, model.ArticlePreviewTag{UID: uid, Name: name})
		}
	}
	return categoryName, tags, nil
}

func (r *ArticlePreviewDBRepo) Create(tx *gorm.DB, preview *model.ArticlePreview) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	return tx.Create(preview).Error
}

func (r *ArticlePreviewDBRepo) FindValidByTokenHash(tokenHash string, now time.Time) (*model.ArticlePreview, error) {
	var preview model.ArticlePreview
	err := db.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, now).
		First(&preview).Error
	if err != nil {
		return nil, err
	}
	// SQLite represents DATETIME values as text in some test/runtime builds, so
	// retain the server-side filter but also compare the decoded value in Go.
	if preview.RevokedAt != nil || !preview.ExpiresAt.After(now) {
		return nil, gorm.ErrRecordNotFound
	}
	return &preview, nil
}

func (r *ArticlePreviewDBRepo) RevokeForArticleCreator(articleID, creatorID uint, now time.Time) error {
	return db.Model(&model.ArticlePreview{}).
		Where("article_id = ? AND creator_id = ? AND revoked_at IS NULL", articleID, creatorID).
		Update("revoked_at", now).Error
}

func articlePreviewCreatorCanManage(creator *model.User, article *ArticleModel) bool {
	if creator == nil || article == nil || creator.Disabled {
		return false
	}
	return creator.CanEditAllContent() || (creator.PermissionType == model.Author && creator.ID == article.AuthorID)
}
