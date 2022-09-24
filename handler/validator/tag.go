package validator

import (
	"myecho/dal/connect"
	"myecho/handler/errors"
	"myecho/handler/rtype"
	"myecho/model"
)

func ValidateTagIDs(tagIDs []uint) error {
	if len(tagIDs) == 0 {
		return nil
	}
	var tags []model.Tag
	connect.Database.Where("id in (?)", tagIDs).Find(&tags)
	if len(tags) != len(tagIDs) {
		return errors.ErrTagNotFound
	}
	return nil
}

func ValidateTagRequest(tagRequest *rtype.TagRequest) error {
	if len(tagRequest.Name) == 0 {
		return errors.ErrTagNameEmpty
	}
	// 查看是否有重复的
	var tag model.Tag
	connect.Database.Where("name = ?", tagRequest.Name).First(&tag)
	if tag.ID != 0 {
		return errors.ErrTagNameExist
	}
	return nil
}
