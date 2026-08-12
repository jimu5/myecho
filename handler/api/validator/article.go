package validator

import (
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"
	"time"
	"unicode/utf8"
)

func ValidateArticleRequest(articleRequest *rtype.ArticleRequest) error {
	if len(articleRequest.Title) == 0 {
		return errors.ErrTitleEmpty
	}
	if len(articleRequest.Content) == 0 {
		return errors.ErrContentEmpty
	}
	if utf8.RuneCountInString(articleRequest.SEOTitle) > 160 ||
		utf8.RuneCountInString(articleRequest.SEODescription) > 255 ||
		utf8.RuneCountInString(articleRequest.ShareImage) > 512 {
		return errors.ErrInvalidParams
	}
	if articleRequest.PostTime.IsZero() {
		articleRequest.PostTime = time.Now()
	}
	if articleRequest.Type == 0 {
		articleRequest.Type = model.ArticleTypePost
	}
	if !model.IsValidArticleType(articleRequest.Type) {
		return errors.ErrInvalidParams
	}
	articleRequest.ContentFormat = model.NormalizeArticleContentFormat(articleRequest.ContentFormat)
	if !model.IsValidArticleContentFormat(articleRequest.ContentFormat) {
		return errors.ErrInvalidParams
	}
	if err := ValidateCategoryUID(articleRequest.CategoryUID); err != nil {
		return err
	}
	if err := ValidateTagUIDs(articleRequest.TagUIDs); err != nil {
		return err
	}
	return nil
}
