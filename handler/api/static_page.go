package api

import (
	"fmt"
	"myecho/handler"
	"myecho/handler/api/errors"
	"myecho/service"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func StaticPageList(c *fiber.Ctx) error {
	pages, err := service.S.StaticPage.ListStaticPages()
	if err != nil {
		return err
	}
	return handler.Success(c, pages)
}

func UploadStaticPage(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(file.Filename), ".zip") {
		return errors.ErrInvalidParams
	}
	if file.Size > service.MaxStaticPagePackageBytes {
		return errors.ErrInvalidParams
	}
	if err := os.MkdirAll("./storage/temp", 0755); err != nil {
		return err
	}

	tmpPath := filepath.Join("./storage/temp", fmt.Sprintf("static-page-%d.zip", time.Now().UnixNano()))
	if err := c.SaveFile(file, tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	page, err := service.S.StaticPage.InstallStaticPagePackage(tmpPath)
	if err != nil {
		return err
	}
	return handler.Success(c, page)
}

func DeleteStaticPage(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.Params("name"))
	if name == "" {
		return errors.ErrInvalidParams
	}
	if err := service.S.StaticPage.DeleteStaticPage(name); err != nil {
		return err
	}
	return handler.Success(c, fiber.Map{"message": "删除成功"})
}
