package validator

import (
	"myecho/dal/connect"
	"myecho/handler/api/errors"
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

func ValidateTagUIDs(tagUIDs []string) error {
	if len(tagUIDs) == 0 {
		return nil
	}
	var counts int64
	err := connect.Database.Model(&model.Tag{}).Where("uid in (?)", tagUIDs).Count(&counts).Error
	if err != nil {
		return err
	}
	if int64(len(tagUIDs)) != counts {
		return errors.ErrTagNotFound
	}
	return nil
}

func ValidateTagRequest(tagRequest *rtype.TagRequest) error {
	if len(tagRequest.Name) == 0 {
		return errors.ErrTagNameEmpty
	}
	// 查看是否有重复的
	var count int64
	if err := connect.Database.Model(&model.Tag{}).Where("name = ?", tagRequest.Name).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.ErrTagNameExist
	}
	return nil
}
