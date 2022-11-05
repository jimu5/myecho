package model

type CategoryType uint

const (
	CategoryTypeArticle CategoryType = 1
	CategoryTypeLink    CategoryType = 2
)

func (ct CategoryType) IsCategoryTypeValid() bool {
	if ct < CategoryTypeArticle || ct > CategoryTypeLink {
		return false
	}
	return true
}

// 分类
type Category struct {
	BaseModel
	Name      string       `json:"name" gorm:"size:64"`
	UID       string       `json:"uid" gorm:"size:20"`
	FatherUID string       `json:"father_uid" gorm:"default:null"`
	Count     uint         `json:"count"`
	Type      CategoryType `json:"type" gorm:"default:1"`
}
