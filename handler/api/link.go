package api

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/service"
	"net/url"
	"strings"
)

func LinkCreate(c *fiber.Ctx) error {
	var link mysql.LinkModel
	if err := c.BodyParser(&link); err != nil {
		return err
	}
	if err := normalizeLink(&link); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	if err := service.S.Link.Create(&link); err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusCreated, &link)
}

func LinkUpdate(c *fiber.Ctx) error {
	var link mysql.LinkModel
	if err := c.BodyParser(&link); err != nil {
		return err
	}
	if err := normalizeLink(&link); err != nil {
		return ValidateErrorResponse(c, err.Error())
	}
	id, err := handler.GetIDByParam(c, &mysql.LinkModel{})
	if err != nil {
		return err
	}
	link.ID = id
	err = service.S.Link.UpdateByID(id, &link)
	if err != nil {
		return err
	}
	return handler.Success(c, &link)
}

func LinkDelete(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &mysql.LinkModel{})
	if err != nil {
		return err
	}
	err = service.S.Link.DeleteByID(id)
	if err != nil {
		return err
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func LinkAll(c *fiber.Ctx) error {
	var param rtype.LinkQueryParam
	err := c.QueryParser(&param)
	if err != nil {
		return err
	}
	dalParam := param.ToDALParam()
	result, err := service.S.Link.All(&dalParam)
	if err != nil {
		return err
	}
	return handler.Success(c, &result)
}

func normalizeLink(link *mysql.LinkModel) error {
	link.Name = strings.TrimSpace(link.Name)
	if link.Name == "" {
		return fmt.Errorf("链接名称不能为空")
	}
	normalized, err := normalizeHTTPURL(link.URL, true)
	if err != nil {
		return fmt.Errorf("链接地址无效: %w", err)
	}
	link.URL = normalized
	if strings.TrimSpace(link.IconURL) != "" {
		link.IconURL, err = normalizeHTTPURL(link.IconURL, false)
		if err != nil {
			return fmt.Errorf("图像地址无效: %w", err)
		}
	}
	return nil
}

func normalizeHTTPURL(raw string, required bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return "", fmt.Errorf("不能为空")
		}
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("仅支持有效的 http/https 地址")
	}
	return parsed.String(), nil
}
