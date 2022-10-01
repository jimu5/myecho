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
		Name:          filename[0 : len(filename)-len(extName)],
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

func (fr *FileRepo) PageQueryByName(param *PageFindParam, name string) ([]*FileModel, error) {
	result := make([]*FileModel, 0)
	err := db.Model(&FileModel{}).Scopes(Paginate(param)).Where("name like ?", "%"+name+"%").Find(&result).Error
	return result, err
}

func (fr *FileRepo) FindFilesByMD5s(md5 []string) ([]*FileModel, error) {
	result := make([]*FileModel, 0)
	err := db.Model(&FileModel{}).Where("md5 in (?)", md5).Find(&result).Error
	return result, err
}
