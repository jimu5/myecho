package mysql

import (
	"gorm.io/gorm"
	"myecho/config"
)

type PageFindParam struct {
	Page     int  `json:"page" query:"page"`
	PageSize int  `json:"page_size" query:"page_size"`
	NoPage   bool `json:"no_page" query:"no_page"`
}

func Paginate(param *PageFindParam) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 如果不需要分页
		if param.NoPage {
			return db
		}
		if param.Page < 1 {
			param.Page = 1
		}
		if param.PageSize < 1 {
			param.PageSize = config.PageSize
		}
		offset := (param.Page - 1) * param.PageSize
		return db.Offset(offset).Limit(param.PageSize)
	}
}
