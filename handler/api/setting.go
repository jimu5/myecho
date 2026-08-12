package api

import (
	"github.com/gofiber/fiber/v2"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api/errors"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
)

func SettingCreate(c *fiber.Ctx) error {
	setting := rtype.SettingCreateReq{}
	if err := c.BodyParser(&setting); err != nil {
		return err
	}
	if err := setting.Validate(); err != nil {
		return err
	}
	err := service.S.Setting.Create(&mysql.SettingModel{
		Key:         setting.Key,
		Value:       setting.Value,
		Type:        setting.Type,
		Description: setting.Description,
		IsPublic:    setting.IsPublic,
	})
	if err != nil {
		return err
	}
	result, err := service.S.Setting.GetByKey(setting.Key)
	if err != nil {
		return err
	}
	return handler.Success(c, rtype.NewSetting(model.Setting(result), service.IsSettingPublic(&result)))
}

func SettingUpdate(c *fiber.Ctx) error {
	reqParam := rtype.SettingUpdateReq{}
	key := c.Params("key")
	if len(key) == 0 {
		return errors.ErrSettingKey
	}
	if err := c.BodyParser(&reqParam); err != nil {
		return err
	}
	result, err := service.S.Setting.UpdateValueDescAndVisibility(key, reqParam.Value, reqParam.Description, reqParam.IsPublic)
	if err != nil {
		return err
	}
	return handler.Success(c, rtype.NewSetting(model.Setting(result), service.IsSettingPublic(&result)))
}

func SettingRetrieve(c *fiber.Ctx) error {
	key := c.Params("key")
	if len(key) == 0 {
		return errors.ErrSettingKey
	}
	result, err := service.S.Setting.GetByKey(key)
	if err != nil {
		return err
	}
	if !service.IsSettingPublic(&result) {
		return errors.ErrSettingKey
	}
	return handler.Success(c, rtype.NewSetting(model.Setting(result), true))
}

func SettingAll(c *fiber.Ctx) error {
	result, err := service.S.Setting.GetAll()
	if err != nil {
		return err
	}
	filtered := make([]rtype.Setting, 0, len(result))
	for _, setting := range result {
		if !service.IsSettingPublic(setting) {
			continue
		}
		filtered = append(filtered, rtype.NewSetting(model.Setting(*setting), true))
	}
	return handler.Success(c, &filtered)
}

func SettingAdminAll(c *fiber.Ctx) error {
	result, err := service.S.Setting.GetAll()
	if err != nil {
		return err
	}
	settings := make([]rtype.Setting, 0, len(result))
	for _, setting := range result {
		if setting == nil || service.IsHiddenSettingKey(setting.Key) {
			continue
		}
		settings = append(settings, rtype.NewSetting(model.Setting(*setting), service.IsSettingPublic(setting)))
	}
	return handler.Success(c, &settings)
}

func SettingDelete(c *fiber.Ctx) error {
	key := c.Params("key")
	if len(key) == 0 {
		return errors.ErrSettingKey
	}
	if err := service.S.Setting.DeleteByKey(key); err != nil {
		return err
	}
	return handler.Success(c, nil)
}
