package validator

import (
	"myecho/config"
	"myecho/handler/errors"
)

func ValidateID[T any](id int, model *T) error {
	result := config.Database.First(model, id)
	if result.Error != nil {
		return errors.ErrorIDNotFound
	}
	return nil
}
