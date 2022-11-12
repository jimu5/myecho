package service

import (
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
)

type FilePageListParam struct {
	mysql.PageFindParam
	Name string `json:"name" query:"name"`
}

type UpdateFileParam struct {
	FullName string `json:"full_name"`
	Note     string `json:"note"`
}

func (ufp *UpdateFileParam) FillModel(fileModel *mysql.FileModel) {
	fileModel.Name, fileModel.ExtensionName = utils.ParseFileFullName(ufp.FullName)
	fileModel.Note = ufp.Note
}

type File struct {
	model.BaseModel
	FullName      string `json:"full_name"`
	UUID          string `json:"uuid"`
	ExtensionName string `json:"extension_name"`
	MD5           string `json:"md5"`
	URL           string `json:"url"`
	Note          string `json:"note"`
}

func modelToFile(model *mysql.FileModel) File {
	return File{
		BaseModel:     model.BaseModel,
		FullName:      model.GetFullName(),
		UUID:          model.UUID,
		ExtensionName: model.ExtensionName,
		MD5:           model.MD5,
		URL:           model.GetUrlPath(),
		Note:          model.Note,
	}
}
func mModelToFiles(models []*mysql.FileModel) []*File {
	files := make([]*File, len(models))
	for i, m := range models {
		f := modelToFile(m)
		files[i] = &f
	}
	return files
}

type FileService struct {
}

func (fs *FileService) PageList(param *FilePageListParam) (mysql.PageInfo, []*File, error) {
	var (
		pageInfo mysql.PageInfo
		err      error
	)
	pageInfo.Total, err = dal.MySqlDB.File.CountByName(param.Name)
	if err != nil {
		return mysql.PageInfo{}, nil, err
	}
	pageInfo.FillInfoFromParam(&param.PageFindParam)
	modelFiles, err := dal.MySqlDB.File.PageQueryByName(&param.PageFindParam, param.Name)
	if err != nil {
		return mysql.PageInfo{}, nil, err
	}
	files := mModelToFiles(modelFiles)

	return pageInfo, files, nil
}

func (fs *FileService) Delete(id uint) error {
	file, err := dal.MySqlDB.File.Get(id)
	if err != nil {
		return err
	}
	err = file.HardDelete()
	if err != nil {
		return err
	}
	return dal.MySqlDB.File.Delete(id)
}

func (fs *FileService) UpdateFile(id uint, param *UpdateFileParam) (File, error) {
	file, err := dal.MySqlDB.File.Get(id)
	if err != nil {
		return File{}, nil
	}
	param.FillModel(&file)
	err = dal.MySqlDB.File.UpdateBasicInfo(&file)
	if err != nil {
		return File{}, nil
	}
	f := modelToFile(&file)
	return f, nil
}

func (fs *FileService) DeleteByUUID(uuid string) error {
	err := dal.MySqlDB.File.DeleteByUUID(uuid)
	return err
}
