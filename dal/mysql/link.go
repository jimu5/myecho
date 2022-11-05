package mysql

import (
	"gorm.io/gorm"
	"myecho/model"
	"strings"
)

type LinkModel model.Link

func (lm *LinkModel) BeforeCreate(tx *gorm.DB) error {
	if err := categoryRepo.ValidateUIDExist(lm.CategoryUID); err != nil {
		return err
	}
	return nil
}

func (lm *LinkModel) AfterCreate(tx *gorm.DB) error {
	if err := lm.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (lm *LinkModel) BeforeUpdate(tx *gorm.DB) error {
	if err := categoryRepo.ValidateUIDExist(lm.CategoryUID); err != nil {
		return err
	}
	if err := lm.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (lm *LinkModel) AfterUpdate(tx *gorm.DB) error {
	if err := lm.AddCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (lm *LinkModel) AfterDelete(tx *gorm.DB) error {
	if err := lm.ReduceCategoryCount(tx); err != nil {
		return err
	}
	return nil
}

func (lm *LinkModel) AddCategoryCount(tx *gorm.DB) error {
	if len(lm.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", lm.CategoryUID).Update("count", gorm.Expr("count + 1")).Error
	}
	return nil
}

func (lm *LinkModel) ReduceCategoryCount(tx *gorm.DB) error {
	oldLink, err := linkRepo.TxGet(tx, lm.ID)
	if err != nil {
		return err
	}
	if len(oldLink.CategoryUID) != 0 {
		return tx.Model(&CategoryModel{}).Where("uid = ?", oldLink.CategoryUID).Update("count", gorm.Expr("count - 1")).Error
	}
	return nil
}

type LinkRepo struct {
}

type LinkCommonQueryParam struct {
	CategoryUID *string
}

func (lcp *LinkCommonQueryParam) GenQuerySQL(db *gorm.DB) (*gorm.DB, error) {
	sqlPrefix := make([]string, 0)
	sqlValue := make([]interface{}, 0)
	if lcp.CategoryUID != nil {
		sql := "category_uid in (?)"
		allUID := make([]string, 0)
		allUID = append(allUID, *lcp.CategoryUID)
		fatherUIDs, err := categoryRepo.GetAllChildrenUID(*lcp.CategoryUID)
		if err != nil {
			return nil, err
		}
		allUID = append(allUID, fatherUIDs...)
		sqlPrefix = append(sqlPrefix, sql)
		sqlValue = append(sqlValue, allUID)
	}
	return db.Where(strings.Join(sqlPrefix, queryAND), sqlValue...), nil
}

func (l *LinkRepo) Create(linkModel *LinkModel) error {
	return db.Create(linkModel).Error
}

func (l *LinkRepo) UpdateByID(id uint, linkModel *LinkModel) error {
	return db.Model(&LinkModel{}).Where("id = ?", id).Save(linkModel).Error
}

func (l *LinkRepo) DeleteByID(id uint) error {
	return db.Model(&LinkModel{}).Where("id = ?", id).Error
}

func (l *LinkRepo) All(param *LinkCommonQueryParam) ([]*LinkModel, error) {
	d, err := param.GenQuerySQL(db.Model(&LinkModel{}))
	if err != nil {
		return nil, err
	}
	result := make([]*LinkModel, 0)
	err = d.Find(&result).Error
	return result, err
}

func (l *LinkRepo) TxGet(tx *gorm.DB, id uint) (LinkModel, error) {
	var link LinkModel
	err := tx.Model(&LinkModel{}).Where("id = ?", id).First(&link).Error
	return link, err
}
