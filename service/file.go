package service

import (
	"errors"
	"fmt"
	"myecho/dal"
	"myecho/dal/connect"
	"myecho/dal/mysql"
	"myecho/model"
	"myecho/utils"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

var ErrFileForbidden = errors.New("file access denied")
var ErrFileInUse = errors.New("file is referenced by content")

type FileUsage struct {
	Type  string `json:"type"`
	ID    uint   `json:"id"`
	Title string `json:"title"`
}

type FilePageListParam struct {
	mysql.PageFindParam
	Name       string `json:"name" query:"name"`
	UploaderID *uint  `json:"uploader_id" query:"uploader_id"`
}

type UpdateFileParam struct {
	FullName string `json:"full_name"`
	Note     string `json:"note"`
	AltText  string `json:"alt_text"`
	Caption  string `json:"caption"`
}

func (ufp *UpdateFileParam) FillModel(fileModel *mysql.FileModel) {
	name, _ := utils.ParseFileFullName(ufp.FullName)
	fileModel.Name = name
	fileModel.Note = ufp.Note
	fileModel.AltText = ufp.AltText
	fileModel.Caption = ufp.Caption
}

type File struct {
	model.BaseModel
	FullName      string `json:"full_name"`
	UUID          string `json:"uuid"`
	ExtensionName string `json:"extension_name"`
	MD5           string `json:"md5"`
	URL           string `json:"url"`
	Note          string `json:"note"`
	UploaderID    uint   `json:"uploader_id"`
	AltText       string `json:"alt_text"`
	Caption       string `json:"caption"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	ThumbnailURL  string `json:"thumbnail_url"`
}

func ModelToFile(model *mysql.FileModel) File {
	return File{
		BaseModel:     model.BaseModel,
		FullName:      model.GetFullName(),
		UUID:          model.UUID,
		ExtensionName: model.ExtensionName,
		MD5:           model.MD5,
		URL:           model.GetUrlPath(),
		Note:          model.Note,
		UploaderID:    model.UploaderID,
		AltText:       model.AltText,
		Caption:       model.Caption,
		Width:         model.Width,
		Height:        model.Height,
		ThumbnailURL:  model.ThumbnailURL,
	}
}
func MModelToFiles(models []*mysql.FileModel) []*File {
	files := make([]*File, len(models))
	for i, m := range models {
		f := ModelToFile(m)
		files[i] = &f
	}
	return files
}

type FileService struct {
}

func (fs *FileService) PageList(param *FilePageListParam, users ...*model.User) (mysql.PageInfo, []*File, error) {
	var (
		pageInfo mysql.PageInfo
		err      error
	)
	uploaderID := param.UploaderID
	if user := fileUser(users); user != nil && !user.CanEditAllContent() {
		uploaderID = &user.ID
	}
	pageInfo.Total, err = dal.MySqlDB.File.CountByName(param.Name, uploaderID)
	if err != nil {
		return mysql.PageInfo{}, nil, err
	}
	pageInfo.FillInfoFromParam(&param.PageFindParam)
	modelFiles, err := dal.MySqlDB.File.PageQueryByName(&param.PageFindParam, param.Name, uploaderID)
	if err != nil {
		return mysql.PageInfo{}, nil, err
	}
	files := MModelToFiles(modelFiles)

	return pageInfo, files, nil
}

func (fs *FileService) Delete(id uint, users ...*model.User) error {
	file, err := dal.MySqlDB.File.Get(id)
	if err != nil {
		return err
	}
	if !canManageFile(&file, fileUser(users)) {
		return ErrFileForbidden
	}
	usages, err := fs.Usages(id, users...)
	if err != nil {
		return err
	}
	if len(usages) > 0 {
		return fmt.Errorf("%w: %d references", ErrFileInUse, len(usages))
	}
	err = file.HardDelete()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return dal.MySqlDB.File.Delete(id)
}

func (fs *FileService) Usages(id uint, users ...*model.User) ([]FileUsage, error) {
	file, err := dal.MySqlDB.File.Get(id)
	if err != nil {
		return nil, err
	}
	if !canManageFile(&file, fileUser(users)) {
		return nil, ErrFileForbidden
	}
	fileURL := file.GetUrlPath()
	like := "%" + fileURL + "%"
	var articles []struct {
		ID    uint
		Title string
	}
	if err := connect.Database.Table("articles").
		Select("DISTINCT articles.id, articles.title").
		Joins("LEFT JOIN article_details ON article_details.uid = articles.detail_uid").
		Where("articles.deleted_at IS NULL AND (articles.share_image LIKE ? OR article_details.content LIKE ?)", like, like).
		Scan(&articles).Error; err != nil {
		return nil, err
	}
	result := make([]FileUsage, 0, len(articles))
	for _, article := range articles {
		result = append(result, FileUsage{Type: "article", ID: article.ID, Title: article.Title})
	}
	var settings []model.Setting
	if err := connect.Database.Select("id", "key").Where("value LIKE ?", like).Find(&settings).Error; err != nil {
		return nil, err
	}
	for _, setting := range settings {
		result = append(result, FileUsage{Type: "setting", ID: setting.ID, Title: setting.Key})
	}
	var submissions []model.ArticleSubmission
	if err := connect.Database.Select("id").Where("status IN ? AND snapshot LIKE ?", []model.ArticleSubmissionStatus{
		model.ArticleSubmissionDraft, model.ArticleSubmissionPending, model.ArticleSubmissionRejected,
	}, like).Find(&submissions).Error; err != nil {
		return nil, err
	}
	for _, submission := range submissions {
		result = append(result, FileUsage{Type: "submission", ID: submission.ID, Title: fmt.Sprintf("投稿 #%d", submission.ID)})
	}
	return result, nil
}

func (fs *FileService) UpdateFile(id uint, param *UpdateFileParam, users ...*model.User) (File, error) {
	file, err := dal.MySqlDB.File.Get(id)
	if err != nil {
		return File{}, err
	}
	if !canManageFile(&file, fileUser(users)) {
		return File{}, ErrFileForbidden
	}
	param.FullName = strings.TrimSpace(param.FullName)
	if param.FullName == "" || path.Base(param.FullName) != param.FullName ||
		utf8.RuneCountInString(param.FullName) > 287 || utf8.RuneCountInString(param.Note) > 256 ||
		utf8.RuneCountInString(param.AltText) > 512 || utf8.RuneCountInString(param.Caption) > 512 {
		return File{}, fmt.Errorf("invalid file metadata")
	}
	_, extName := utils.ParseFileFullName(param.FullName)
	if extName != "" && extName != file.ExtensionName {
		return File{}, fmt.Errorf("file extension cannot be changed")
	}
	param.FillModel(&file)
	err = dal.MySqlDB.File.UpdateBasicInfo(&file)
	if err != nil {
		return File{}, err
	}
	f := ModelToFile(&file)
	return f, nil
}

func (fs *FileService) DeleteByUUID(uuid string) error {
	err := dal.MySqlDB.File.DeleteByUUID(uuid)
	return err
}

func fileUser(users []*model.User) *model.User {
	if len(users) == 0 {
		return nil
	}
	return users[0]
}

func canManageFile(file *mysql.FileModel, user *model.User) bool {
	return user == nil || user.CanEditAllContent() || file.UploaderID == user.ID
}
