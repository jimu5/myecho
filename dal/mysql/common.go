package mysql

import (
	"gorm.io/gorm"
)

type PageFindParam struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	NoPage   bool `json:"no_page"`
}

func Paginate(param *PageFindParam) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 如果不需要分页
		if param.NoPage {
			return db
		}
		offset := (param.Page - 1) * param.PageSize
		return db.Offset(offset).Limit(param.PageSize)
	}
}
