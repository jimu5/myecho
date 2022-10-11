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
	if err := ValidateCategoryUUID(articleRequest.CategoryUUID); err != nil {
		return err
	}
	if err := ValidateTagUUIDs(articleRequest.TagUUIDs); err != nil {
		return err
	}
	return nil
}
