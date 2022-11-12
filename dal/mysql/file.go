package mysql

import (
	"myecho/model"
	"path"
	"strconv"
	"time"
)

type FileRepo struct {
}

type FileModel = model.File

func GenFileModel(filename, extName string) FileModel {
	now := time.Now()
	return FileModel{
		Name:          filename,
		ExtensionName: extName,
		DirPath:       path.Join(strconv.FormatInt(int64(now.Year()), 10), strconv.FormatInt(int64(now.Month()), 10)),
	}
}

func (fr *FileRepo) Create(file *FileModel) error {
	return db.Create(file).Error
}

func (fr *FileRepo) DeleteByUUID(uuid string) error {
	return db.Where("uuid = ?", uuid).Delete(&FileModel{}).Error
}

func (fr *FileRepo) Get(id uint) (FileModel, error) {
	var file FileModel
	err := db.Model(&FileModel{}).Where("id = ?", id).First(&file).Error
	return file, err
}

func (fr *FileRepo) Delete(id uint) error {
	m := FileModel{}
	m.ID = id
	return db.Delete(&m).Error
}

func (fr *FileRepo) UpdateBasicInfo(file *FileModel) error {
	err := db.Model(file).Select("name", "extension_name", "note").Updates(file).Error
	return err
}

func (fr *FileRepo) PageQueryByName(param *PageFindParam, name string) ([]*FileModel, error) {
	var err error
	result := make([]*FileModel, 0)
	if len(name) != 0 {
		err = db.Model(&FileModel{}).Scopes(Paginate(param)).Where("name like ?", "%"+name+"%").Order("created_at desc").Find(&result).Error
	} else {
		err = db.Model(&FileModel{}).Scopes(Paginate(param)).Find(&result).Order("created_at desc").Error
	}
	return result, err
}

func (fr *FileRepo) CountByName(name string) (int64, error) {
	var (
		total int64
		err   error
	)
	if len(name) != 0 {
		err = db.Model(&FileModel{}).Where("name like ?", "%"+name+"%").Count(&total).Error
	} else {
		err = db.Model(&FileModel{}).Count(&total).Error
	}
	return total, err
}

func (fr *FileRepo) FindFilesByMD5s(md5 []string) ([]*FileModel, error) {
	result := make([]*FileModel, 0)
	err := db.Model(&FileModel{}).Where("md5 in (?)", md5).Find(&result).Error
	return result, err
}
