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
	err := db.Model(file).Select("name", "extension_name", "note", "alt_text", "caption").Updates(file).Error
	return err
}

func (fr *FileRepo) PageQueryByName(param *PageFindParam, name string, uploaderID ...*uint) ([]*FileModel, error) {
	var err error
	result := make([]*FileModel, 0)
	query := db.Model(&FileModel{}).Scopes(Paginate(param))
	if len(uploaderID) != 0 && uploaderID[0] != nil {
		query = query.Where("uploader_id = ?", *uploaderID[0])
	}
	if len(name) != 0 {
		err = query.Where("name like ?", "%"+name+"%").Order("created_at desc").Find(&result).Error
	} else {
		err = query.Order("created_at desc").Find(&result).Error
	}
	return result, err
}

func (fr *FileRepo) CountByName(name string, uploaderID ...*uint) (int64, error) {
	var (
		total int64
		err   error
	)
	query := db.Model(&FileModel{})
	if len(uploaderID) != 0 && uploaderID[0] != nil {
		query = query.Where("uploader_id = ?", *uploaderID[0])
	}
	if len(name) != 0 {
		err = query.Where("name like ?", "%"+name+"%").Count(&total).Error
	} else {
		err = query.Count(&total).Error
	}
	return total, err
}

func (fr *FileRepo) FindFilesByMD5s(md5 []string, uploaderID ...uint) ([]*FileModel, error) {
	result := make([]*FileModel, 0)
	query := db.Model(&FileModel{}).Where("md5 in (?)", md5)
	if len(uploaderID) > 0 {
		query = query.Where("uploader_id = ?", uploaderID[0])
	}
	err := query.Find(&result).Error
	return result, err
}
