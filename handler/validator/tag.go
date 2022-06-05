package validator

import (
	"myecho/config"
	"myecho/handler/errors"
	"myecho/handler/rtype"
	"myecho/model"
)

func ValidateTagIDs(tagIDs []uint) error {
	var tags []model.Tag
	config.Database.Where("id in (?)", tagIDs).Find(&tags)
	if len(tags) != len(tagIDs) {
		return errors.ErrTagNotFound
	}
	return nil
}

func ValidateTagRequest(tagRequest *rtype.TagRequest) error {
	if len(tagRequest.Name) == 0 {
		return errors.ErrTagNameEmpty
	}
	return nil
}
