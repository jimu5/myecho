package validator

import (
	"myecho/config"
	"myecho/handler/errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func ValidateID[T any](c *fiber.Ctx, model *T) error {
	id := c.Params("id")
	id_int, err := strconv.Atoi(id)
	if err != nil || id_int <= 0 {
		return errors.ErrorIDNotFound
	}
	result := config.Database.First(model, id_int)
	if result.Error != nil {
		return errors.ErrorIDNotFound
	}
	return nil
}
