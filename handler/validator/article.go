package validator

import (
	"time"

	"myecho/config"
	"myecho/handler/errors"
	"myecho/handler/rtype"
	"myecho/model"
)

func ValidateCategoryID(categoryID uint) error {
	if categoryID == 0 {
		return nil
	}
	err := config.Database.Where("id = ?", categoryID).First(&model.Category{}).Error
	if err != nil {
		return errors.ErrCategoryNotFound
	}
	return nil
}

func ValidateArticleRequest(articleRequest *rtype.ArticleRequest) error {
	if len(articleRequest.Title) == 0 {
		return errors.ErrTitleEmpty
	}
	if len(articleRequest.Content) == 0 {
		return errors.ErrContentEmpty
	}
	if articleRequest.PostTime.IsZero() {
		articleRequest.PostTime = time.Now()
	}
	if err := ValidateCategoryID(articleRequest.CategoryID); err != nil {
		return err
	}
	return nil
}
