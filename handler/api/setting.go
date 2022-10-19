package api

import (
	"github.com/gofiber/fiber/v2"
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
	err := service.S.Setting.Create(&model.Setting{Key: setting.Key, Value: setting.Value, Type: setting.Value})
	if err != nil {
		return err
	}
	result, err := service.S.Setting.GetByKey(setting.Key)
	if err != nil {
		return err
	}
	return c.JSON(&result)
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
	result, err := service.S.Setting.UpdateValue(key, reqParam.Value)
	if err != nil {
		return err
	}
	return c.JSON(&result)
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
	return c.JSON(&result)
}

func SettingAll(c *fiber.Ctx) error {
	result, err := service.S.Setting.GetAll()
	if err != nil {
		return err
	}
	return c.JSON(&result)
}
