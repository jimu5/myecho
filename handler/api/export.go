package api

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"

	"myecho/service"
)

func Export(c *fiber.Ctx) error {
	archivePath, size, err := service.CreateExportArchive()
	if errors.Is(err, service.ErrExportTooLarge) {
		return ErrorResponse(c, fiber.StatusRequestEntityTooLarge, CommonBadError, err.Error())
	}
	if err != nil {
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	file, err := os.Open(archivePath)
	if err != nil {
		_ = os.Remove(archivePath)
		return InternalErrorResponse(c, InternalSQLError, err.Error())
	}
	stream := &exportArchiveFile{File: file, path: archivePath}
	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(
		`attachment; filename="myecho-export-%s.zip"`,
		time.Now().Format("20060102-150405"),
	))
	if err := c.SendStream(stream, int(size)); err != nil {
		_ = stream.Close()
		return err
	}
	return nil
}

type exportArchiveFile struct {
	*os.File
	path string
}

func (f *exportArchiveFile) Close() error {
	closeErr := f.File.Close()
	removeErr := os.Remove(f.path)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}
