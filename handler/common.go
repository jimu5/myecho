package handler

import (
	"strconv"

	"github.com/Kimiato/myecho/config"
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

func ValidateID(c *fiber.Ctx, model interface{}) error {
	id := c.Params("id")
	id_int, err := strconv.Atoi(id)
	if err != nil || id_int <= 0 {
		return ErrorIDNotFound
	}
	result := config.Database.First(&model, id_int)
	if result.Error != nil {
		return ErrorIDNotFound
	}
	return nil
}
