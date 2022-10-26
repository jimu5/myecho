package rtype

type CategoryCreateRequest struct {
	Name      string `json:"name" gorm:"size:64"`
	FatherUID string `json:"father_uid" gorm:"default:null"`
}

type CategoryUpdateRequest struct {
	Name     *string `json:"name" gorm:"size:64"`
	FatherID *uint   `json:"father_id" gorm:"default:null"`
}
