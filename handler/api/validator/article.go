package validator

import (
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
	"time"
)

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
	if err := ValidateCategoryUID(articleRequest.CategoryUID); err != nil {
		return err
	}
	if err := ValidateTagUIDs(articleRequest.TagUIDs); err != nil {
		return err
	}
	return nil
}
