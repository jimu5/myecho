package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"myecho/handler"
	"myecho/service"
)

func Import(c *fiber.Ctx) error {
	dryRun, err := strconv.ParseBool(c.Query("dry_run", "true"))
	if err != nil {
		return ParseErrorResponse(c, "dry_run must be true or false")
	}
	header, err := c.FormFile("file")
	if err != nil {
		return ParseErrorResponse(c, "backup ZIP is required")
	}
	if header.Size <= 0 || header.Size > 512<<20 {
		return ErrorResponse(c, fiber.StatusRequestEntityTooLarge, CommonBadError, service.ErrRestoreTooLarge.Error())
	}
	file, err := header.Open()
	if err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	defer file.Close()

	if dryRun {
		preview, err := service.PreviewRestoreArchive(file, header.Size)
		if err != nil {
			return restoreErrorResponse(c, err)
		}
		return handler.Success(c, preview)
	}
	result, err := service.RestoreArchive(file, header.Size, handler.GetUserFromCtx(c).ID)
	if err != nil {
		return restoreErrorResponse(c, err)
	}
	return handler.Success(c, result)
}

func restoreErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrRestoreTooLarge), errors.Is(err, service.ErrExportTooLarge):
		return ErrorResponse(c, fiber.StatusRequestEntityTooLarge, CommonBadError, err.Error())
	case errors.Is(err, service.ErrInvalidBackup):
		return ParseErrorResponse(c, err.Error())
	default:
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
}
