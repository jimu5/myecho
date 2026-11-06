package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/handler"
	"myecho/model"
	"myecho/service"
)

// ContentExport sends the portable Markdown content archive.
func ContentExport(c *fiber.Ctx) error {
	includeMedia, err := strconv.ParseBool(c.Query("include_media", "false"))
	if err != nil {
		return ParseErrorResponse(c, "include_media must be true or false")
	}
	archive, err := service.CreateContentExportArchiveWithMedia(includeMedia)
	if errors.Is(err, service.ErrContentArchiveTooLarge) {
		return ErrorResponse(c, fiber.StatusRequestEntityTooLarge, CommonBadError, err.Error())
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(
		`attachment; filename="myecho-content-%s.zip"`,
		time.Now().Format("20060102-150405"),
	))
	return c.Send(archive)
}

// ContentImport previews by default and applies only when dry_run=false.
func ContentImport(c *fiber.Ctx) error {
	dryRun, err := strconv.ParseBool(c.Query("dry_run", "true"))
	if err != nil {
		return ParseErrorResponse(c, "dry_run must be true or false")
	}
	header, err := c.FormFile("file")
	if err != nil {
		return ParseErrorResponse(c, "content ZIP is required")
	}
	if header.Size <= 0 || header.Size > service.MaxContentArchiveBytes {
		return ErrorResponse(c, fiber.StatusRequestEntityTooLarge, CommonBadError, service.ErrContentArchiveTooLarge.Error())
	}
	file, err := header.Open()
	if err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	defer file.Close()

	var result service.ContentImportResult
	if dryRun {
		result, err = service.PreviewContentImportArchive(file, header.Size)
	} else {
		var authorID uint
		var actorID uint
		if user, ok := c.Locals("user").(*model.User); ok && user != nil {
			authorID = user.ID
			actorID = user.ID
		}
		authorMap := map[string]uint{}
		if raw := c.FormValue("author_map"); raw != "" {
			if decodeErr := json.Unmarshal([]byte(raw), &authorMap); decodeErr != nil {
				return ParseErrorResponse(c, "author_map must be a JSON object")
			}
		}
		result, err = service.ApplyContentImportArchiveWithOptions(file, header.Size, service.ContentImportOptions{
			DefaultAuthorID: authorID, AuthorMap: authorMap, ActorID: actorID,
		})
	}
	if err != nil {
		return contentImportErrorResponse(c, err)
	}
	return handler.Success(c, result)
}

func contentImportErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrContentArchiveTooLarge):
		return ErrorResponse(c, fiber.StatusRequestEntityTooLarge, CommonBadError, err.Error())
	case errors.Is(err, service.ErrContentImportConflict):
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
	case errors.Is(err, service.ErrInvalidContentArchive):
		return ParseErrorResponse(c, err.Error())
	default:
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
}
