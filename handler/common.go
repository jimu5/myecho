package handler

import (
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/handler/api/errors"
	"myecho/handler/api/validator"
	"myecho/handler/rtype"
	"myecho/model"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// 分页结构
type Pagination struct {
	Total int64       `json:"total"`
	Data  interface{} `json:"data"`
}

func Success(c *fiber.Ctx, data interface{}) error {
	return SuccessWithStatus(c, fiber.StatusOK, data)
}

func SuccessWithStatus(c *fiber.Ctx, status int, data interface{}) error {
	return c.Status(status).JSON(rtype.CommonResp{
		Code: 0,
		Msg:  "ok",
		Data: data,
		Meta: map[string]interface{}{},
	})
}

func PaginateData(c *fiber.Ctx, pageInfo mysql.PageInfo, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(rtype.CommonResp{
		Code: 0,
		Msg:  "ok",
		Data: data,
		Meta: map[string]interface{}{
			"total":     pageInfo.Total,
			"page":      pageInfo.Page,
			"page_size": pageInfo.PageSize,
		},
	})
}

func DetailPreHandleByParam[T any](c *fiber.Ctx, model *T) error {
	id := c.Params("id")
	idInt, err := strconv.Atoi(id)
	if err != nil || idInt <= 0 {
		return errors.ErrorIDNotFound
	}
	// model 实际上是一个模型的指针
	return validator.ValidateID(idInt, model)
}

func GetIDByParam[T any](c *fiber.Ctx, model *T) (uint, error) {
	id := c.Params("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil || idInt <= 0 {
		return 0, errors.ErrorIDNotFound
	}
	var count int64
	err = connect.Database.Model(model).Where("id = ?", idInt).Count(&count).Error
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return uint(idInt), errors.ErrorIDNotFound
	}
	return uint(idInt), nil
}

func ParsePageFindParam(c *fiber.Ctx) (mysql.PageFindParam, error) {
	var pageFindParam mysql.PageFindParam
	err := c.QueryParser(&pageFindParam)
	return pageFindParam, err
}

func PageFind[T any, P any](c *fiber.Ctx, findFunc func(*mysql.PageFindParam, P) (T, error), extraParam P) (T, mysql.PageFindParam, error) {
	var result T
	param, err := ParsePageFindParam(c)
	if err != nil {
		return result, param, err
	}
	result, err = findFunc(&param, extraParam)
	return result, param, err
}

func GetUserFromCtx(c *fiber.Ctx) *model.User {
	user := c.Locals("user").(*model.User)
	return user
}

func GetSuccessCommonResp(data interface{}) rtype.CommonResp {
	return rtype.CommonResp{
		Code: 0,
		Msg:  "ok",
		Data: data,
		Meta: map[string]interface{}{},
	}
}
