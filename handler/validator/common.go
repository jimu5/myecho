package validator

import (
	"myecho/dal/connect"
	"myecho/handler/errors"
)

func ValidateID[T any](id int, model *T) error {
	result := connect.Database.First(model, id)
	if result.Error != nil {
		return errors.ErrorIDNotFound
	}
	return nil
}
