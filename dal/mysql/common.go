package mysql

import (
	"gorm.io/gorm"
	"myecho/config/static_config"
)

const queryAND = " AND "

type PageFindParam struct {
	Page        int  `json:"page" query:"page"`
	PageSize    int  `json:"page_size" query:"page_size"`
	NoPage      bool `json:"no_page" query:"no_page"`
	ForceOffset int  // 强制 offset, 当这个属性不等于 0 时, 通过这个属性值来设定 offset
}

type PageInfo struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
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
			param.PageSize = static_config.PageSize
		}
		offset := param.ForceOffset
		if offset == 0 {
			offset = (param.Page - 1) * param.PageSize
		}
		return db.Offset(offset).Limit(param.PageSize)
	}
}

func (p *PageInfo) FillInfoFromParam(param *PageFindParam) {
	if param.Page != 0 {
		p.Page = param.Page
	} else {
		p.Page = 1
	}
	if param.PageSize != 0 {
		p.PageSize = param.PageSize
	} else {
		p.PageSize = static_config.PageSize
	}
}
