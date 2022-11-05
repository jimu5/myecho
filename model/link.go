package model

import "gorm.io/gorm"

type Link struct {
	BaseModel
	Name        string `json:"name" gorm:"size:255"`
	Description string `json:"description" gorm:"size:512"`
	URL         string `json:"url" gorm:"size:1024"`
	AvatarURL   string `json:"avatar" gorm:"size:2048"`
	CategoryUID string `json:"category_uid" gorm:"size: 64"`
}

func (l *Link) AfterCreate(tx *gorm.DB) error {
	if err := l.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (l *Link) BeforeUpdate(tx *gorm.DB) error {
	if err := l.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (l *Link) AfterUpdate(tx *gorm.DB) error {
	if err := l.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (l *Link) AfterDelete(tx *gorm.DB) error {
	if err := l.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (l *Link) AddCategoryCount(tx *gorm.DB) error {
	if len(l.CategoryUID) != 0 {
		return tx.Model(&Category{}).Where("uid = ?", l.CategoryUID).Update("count", gorm.Expr("count + 1")).Error
	}
	return nil
}

func (l *Link) ReduceCategoryCount(tx *gorm.DB) error {
	oldLink, err := getLink(tx, l.ID)
	if err != nil {
		return err
	}
	if len(oldLink.CategoryUID) != 0 {
		return tx.Model(&Category{}).Where("uid = ?", oldLink.CategoryUID).Update("count", gorm.Expr("count - 1")).Error
	}
	return nil
}

func getLink(tx *gorm.DB, id uint) (Link, error) {
	var link Link
	err := tx.Model(&Link{}).Where("id = ?", id).First(&link).Error
	return link, err
}
