package handler

import (
	"myecho/dal/mysql"
	"myecho/handler/errors"
	"myecho/model"
	"strconv"

	"myecho/config"
	"myecho/handler/validator"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// 分页结构
type Pagination struct {
	Total int64       `json:"total"`
	Data  interface{} `json:"data"`
}

// 分页
func Paginate(c *fiber.Ctx) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		page := c.Query("page")
		pageSize := c.Query("page_size")
		noPage := c.Query("no_page")
		// 如果不需要分页
		if noPage != "" {
			return db
		}
		// string to int
		page_int, err := strconv.Atoi(page)
		if err != nil || page == "" {
			page_int = 1
		}
		pageSizeInt, err := strconv.Atoi(pageSize)
		if err != nil || pageSize == "" {
			pageSizeInt = config.PageSize
		}

		offset := (page_int - 1) * pageSizeInt
		return db.Offset(offset).Limit(pageSizeInt)
	}
}

func PaginateData(c *fiber.Ctx, total int64, data interface{}) error {
	return c.Status(200).JSON(Pagination{Total: total, Data: data})
}

func DetailPreHandle[T any](c *fiber.Ctx, model *T) error {
	id := c.Params("id")
	idInt, err := strconv.Atoi(id)
	if err != nil || idInt <= 0 {
		return errors.ErrorIDNotFound
	}
	// model 实际上是一个模型的指针
	return validator.ValidateID(idInt, model)
}

func ParsePageFindParam(c *fiber.Ctx) (mysql.PageFindParam, error) {
	var pageFindParam mysql.PageFindParam
	err := c.QueryParser(&pageFindParam)
	return pageFindParam, err
}

func PageFind[T any](c *fiber.Ctx, findFunc func(*mysql.PageFindParam) (T, error)) (T, error) {
	var result T
	param, err := ParsePageFindParam(c)
	if err != nil {
		return result, err
	}
	return findFunc(&param)
}

func GetUserFromCtx(c *fiber.Ctx) model.User {
	user := c.Locals("user").(model.User)
	return user
}
