package mysql

import (
	"myecho/model"
)

type FileRepo struct {
}

type FileModel = model.File

func (fr *FileRepo) Create(file *FileModel) error {
	return db.Create(file).Error
}

func (fr *FileRepo) DeleteByUUID(uuid string) error {
	return db.Where("uuid = ?", uuid).Delete(&FileModel{}).Error
}

func (fr *FileRepo) PageQueryByName(param *PageFindParam, name string) ([]*FileModel, error) {
	result := make([]*FileModel, 0)
	err := db.Model(&FileModel{}).Scopes(Paginate(param)).Where("name like ?", "%"+name+"%").Find(&result).Error
	return result, err
}
