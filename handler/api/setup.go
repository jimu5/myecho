package api

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"myecho/config"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
)

type setupResponse struct {
	NeedsSetup bool                `json:"needs_setup"`
	User       rtype.LoginResponse `json:"user"`
}

func SetupStatus(c *fiber.Ctx) error {
	needsSetup, err := needsInitialSetup(connect.Database)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	return handler.Success(c, fiber.Map{"needs_setup": needsSetup})
}

func Setup(c *fiber.Ctx) error {
	var req rtype.SetupRequest
	if err := c.BodyParser(&req); err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	if err := validator.ValidateSetupRequest(&req); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}

	setupMu.Lock()
	defer setupMu.Unlock()
	var (
		user     model.User
		settings []mysql.SettingModel
	)
	err = connect.Database.Transaction(func(tx *gorm.DB) error {
		var createErr error
		user, createErr = createInitialAdmin(tx, rtype.RegisterRequest{
			Name:     req.Name,
			NickName: req.Name,
			Email:    req.Email,
			Password: req.Password,
		}, hashedPassword)
		if createErr != nil {
			return createErr
		}
		for key, value := range map[string]string{
			"SiteTitle":       req.SiteTitle,
			"SiteDescription": req.SiteDescription,
		} {
			setting, updateErr := upsertSetupSetting(tx, key, value)
			if updateErr != nil {
				return updateErr
			}
			settings = append(settings, setting)
		}
		return nil
	})
	if errors.Is(err, errSetupCompleted) {
		return LoginErrorResponse(c, CanNotRegister)
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	if config.MySqlSettingModelCache != nil {
		for i := range settings {
			config.MySqlSettingModelCache.Set(settings[i].Key, &settings[i])
		}
	}
	return handler.Success(c, setupResponse{
		NeedsSetup: false,
		User:       userToLoginResponse(user),
	})
}

func upsertSetupSetting(tx *gorm.DB, key, value string) (mysql.SettingModel, error) {
	var setting mysql.SettingModel
	err := tx.Where("key = ?", key).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		setting = mysql.SettingModel{
			Key:      key,
			Value:    value,
			Type:     model.SettingModelTypeString,
			IsSystem: true,
		}
		if err := tx.Create(&setting).Error; err != nil {
			return mysql.SettingModel{}, err
		}
		return setting, nil
	}
	if err != nil {
		return mysql.SettingModel{}, err
	}
	err = tx.Model(&setting).Updates(map[string]interface{}{
		"value":     value,
		"type":      model.SettingModelTypeString,
		"is_system": true,
	}).Error
	if err == nil {
		setting.Value = value
		setting.Type = model.SettingModelTypeString
		setting.IsSystem = true
	}
	return setting, err
}
