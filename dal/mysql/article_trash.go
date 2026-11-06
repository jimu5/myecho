package mysql

import (
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
)

var ErrArticleNotTrashed = errors.New("article is not in the trash")
var ErrInvalidArticleIDs = errors.New("article IDs must be non-empty and non-zero")

// TrashByID moves an article out of public visibility while preserving its
// content and related records. Repeating the operation is harmless.
func (a *ArticleDBRepo) TrashByID(id uint, actorIDs ...uint) error {
	return a.BatchTrash([]uint{id}, actorIDs...)
}

// BatchTrash validates all articles and trashes them in one transaction.
func (a *ArticleDBRepo) BatchTrash(ids []uint, actorIDs ...uint) error {
	ids, err := normalizeArticleIDs(ids)
	if err != nil {
		return err
	}
	actorID := firstActorID(actorIDs)
	return db.Transaction(func(tx *gorm.DB) error {
		articles, err := loadArticlesForTrashMutation(tx, ids)
		if err != nil {
			return err
		}
		for i := range articles {
			if articles[i].Status == int8(ARTICLE_STATUS_RECYCLE) {
				continue
			}
			if !validTrashSourceStatus(articles[i].Status) {
				return ErrArticleStatusChanged
			}
		}
		categoryUIDs := make([]string, 0, len(articles))
		for i := range articles {
			article := &articles[i]
			if isCategoryCountedArticle(article.Status, article.Type) && article.CategoryUID != "" {
				categoryUIDs = append(categoryUIDs, article.CategoryUID)
			}
		}
		if err := lockArticleCategories(tx, categoryUIDs...); err != nil {
			return err
		}
		for i := range articles {
			article := &articles[i]
			if article.Status == int8(ARTICLE_STATUS_RECYCLE) {
				continue
			}
			newVersion, err := claimArticleEditVersion(tx, article.ID, &article.EditVersion)
			if err != nil {
				return err
			}
			result := tx.Model(&ArticleModel{}).
				Where("id = ? AND status = ? AND edit_version = ?", article.ID, article.Status, newVersion).
				Update("status", int8(ARTICLE_STATUS_RECYCLE))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrArticleStatusChanged
			}
			if isCategoryCountedArticle(article.Status, article.Type) && article.CategoryUID != "" {
				if err := tx.Model(&CategoryModel{}).Where("uid = ?", article.CategoryUID).
					Update("count", gorm.Expr("CASE WHEN count > 0 THEN count - 1 ELSE 0 END")).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&model.ArticleSubmission{}).
				Where("article_id = ? AND status = ?", article.ID, model.ArticleSubmissionPending).
				Updates(map[string]interface{}{
					"status":      model.ArticleSubmissionWithdrawn,
					"reviewer_id": actorID,
					"reviewed_at": time.Now(),
					"note":        "article moved to trash",
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RestoreFromTrash restores only trashed articles, always as drafts.
func (a *ArticleDBRepo) RestoreFromTrash(id uint, actorIDs ...uint) error {
	return a.BatchRestoreFromTrash([]uint{id}, actorIDs...)
}

// BatchRestoreFromTrash validates all articles before restoring any of them.
func (a *ArticleDBRepo) BatchRestoreFromTrash(ids []uint, actorIDs ...uint) error {
	ids, err := normalizeArticleIDs(ids)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		articles, err := loadArticlesForTrashMutation(tx, ids)
		if err != nil {
			return err
		}
		for i := range articles {
			if articles[i].Status != int8(ARTICLE_STATUS_RECYCLE) {
				return ErrArticleNotTrashed
			}
		}
		for i := range articles {
			article := &articles[i]
			newVersion, err := claimArticleEditVersion(tx, article.ID, &article.EditVersion)
			if err != nil {
				return err
			}
			result := tx.Model(&ArticleModel{}).
				Where("id = ? AND status = ? AND edit_version = ?", article.ID, int8(ARTICLE_STATUS_RECYCLE), newVersion).
				Update("status", int8(ARTICLE_STATUS_DRAFT))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrArticleStatusChanged
			}
		}
		return nil
	})
}

// PermanentlyDelete removes an article only after it has been moved to trash.
func (a *ArticleDBRepo) PermanentlyDelete(id uint) error {
	return a.BatchPermanentlyDelete([]uint{id})
}

// BatchPermanentlyDelete validates all articles and deletes them atomically.
func (a *ArticleDBRepo) BatchPermanentlyDelete(ids []uint) error {
	ids, err := normalizeArticleIDs(ids)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		articles, err := loadArticlesForTrashMutation(tx, ids)
		if err != nil {
			return err
		}
		for i := range articles {
			if articles[i].Status != int8(ARTICLE_STATUS_RECYCLE) {
				return ErrArticleNotTrashed
			}
		}
		for i := range articles {
			article := &articles[i]
			if _, err := claimArticleEditVersion(tx, article.ID, &article.EditVersion); err != nil {
				return err
			}
			if err := deleteArticleByID(tx, article.ID); err != nil {
				return err
			}
			// Keep the key reserved, but purge cached copies of deleted content.
			if err := tx.Model(&model.ArticleCreateRequest{}).Where("article_id = ?", article.ID).
				UpdateColumn("response", []byte{}).Error; err != nil {
				return err
			}
			// DeleteByID's shared cleanup preserves its historical soft-delete
			// behavior. A permanent purge must remove the trashed article and
			// submissions physically in the same transaction.
			if err := tx.Unscoped().Where("article_id = ?", article.ID).Delete(&model.ArticleSubmission{}).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM articles WHERE id = ?", article.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func loadArticlesForTrashMutation(tx *gorm.DB, ids []uint) ([]ArticleModel, error) {
	articles := make([]ArticleModel, 0, len(ids))
	for _, id := range ids {
		var article ArticleModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&article, id).Error; err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	return articles, nil
}

func normalizeArticleIDs(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, ErrInvalidArticleIDs
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			return nil, ErrInvalidArticleIDs
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func validTrashSourceStatus(status int8) bool {
	return status >= int8(ARTILCE_STATUS_PUBLIC) && status <= int8(ARTICLE_STATUS_WAIT_REVIEW)
}
