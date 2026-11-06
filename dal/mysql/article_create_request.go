package mysql

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myecho/model"
)

var ErrArticleCreateKeyConflict = errors.New("idempotency key was already used for a different article request")
var ErrArticleCreateGone = errors.New("the article for this idempotency key was permanently deleted")

func (a *ArticleDBRepo) ReplayCreateRequest(ownerID uint, key, digest string) ([]byte, error) {
	if key == "" {
		return nil, nil
	}
	var request model.ArticleCreateRequest
	err := db.Where("owner_id = ? AND key = ?", ownerID, key).First(&request).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if request.RequestHash != digest {
		return nil, ErrArticleCreateKeyConflict
	}
	if len(request.Response) == 0 {
		return nil, ErrArticleCreateGone
	}
	return request.Response, nil
}

// Serialize before commit so a lost HTTP response can be replayed verbatim.
func (a *ArticleDBRepo) CreateForRequest(article *ArticleModel, submission *model.ArticleSubmission, review *model.ArticleReview, key, digest string, serialize func() ([]byte, error)) ([]byte, bool, error) {
	var response []byte
	replayed := false
	err := db.Transaction(func(tx *gorm.DB) error {
		request := model.ArticleCreateRequest{
			OwnerID: article.AuthorID, Key: key, RequestHash: digest, Response: []byte{},
		}
		if key != "" {
			insert := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "owner_id"}, {Name: "key"}}, DoNothing: true,
			}).Create(&request)
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected == 0 {
				if err := tx.Where("owner_id = ? AND key = ?", article.AuthorID, key).First(&request).Error; err != nil {
					return err
				}
				if request.RequestHash != digest {
					return ErrArticleCreateKeyConflict
				}
				if len(request.Response) == 0 {
					return ErrArticleCreateGone
				}
				response, replayed = request.Response, true
				return nil
			}
		}
		if err := tx.Model(&ArticleModel{}).Create(article).Error; err != nil {
			return err
		}
		if submission != nil {
			submission.ArticleID = article.ID
			submission.BaseEditVersion = article.EditVersion
			if err := tx.Create(submission).Error; err != nil {
				return err
			}
		}
		if review != nil {
			review.ArticleID = article.ID
			if submission != nil {
				review.SubmissionID = submission.ID
			}
			if err := tx.Create(review).Error; err != nil {
				return err
			}
		}
		var err error
		response, err = serialize()
		if err != nil {
			return err
		}
		if key == "" {
			return nil
		}
		return tx.Model(&request).Updates(map[string]interface{}{
			"article_id": article.ID, "response": response,
		}).Error
	})
	return response, replayed, err
}
