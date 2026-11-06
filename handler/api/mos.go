package api

import (
	stderrors "errors"
	"log"
	"mime/multipart"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/model"
	"myecho/service"
	"myecho/utils"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
)

func VditorFileUpload(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		log.Printf("解析多部分表单失败: %v", err)
		return ParseErrorResponse(c, err.Error())
	}
	files := form.File["file[]"]
	failedFileName := make([]string, 0)
	successFileMap := make(map[string]string, len(files))
	for _, file := range files {
		log.Printf("处理文件: %s", file.Filename)
		fileModel := mysql.GenFileModel(utils.ParseFileFullName(file.Filename))
		// 如果后面出现相同的 filename
		if _, ok := successFileMap[file.Filename]; ok {
			failedFileName = append(failedFileName, file.Filename)
			continue
		}
		if err := saveFile(c, file, &fileModel); err != nil {
			failedFileName = append(failedFileName, file.Filename)
		} else {
			successFileMap[file.Filename] = fileModel.GetUrlPath()
		}
	}
	resp := &rtype.UploadFileResponse{
		ErrFiles: failedFileName,
		SuccMap:  successFileMap,
	}
	return handler.Success(c, resp)
}

func FileSingleUpload(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	files := form.File["file"]
	if len(files) == 0 {
		return ParseErrorResponse(c, "file is required")
	}
	file := files[0]
	fileModel := mysql.GenFileModel(utils.ParseFileFullName(file.Filename))
	if err = saveFile(c, file, &fileModel); err != nil {
		return err
	}
	fileResp := service.ModelToFile(&fileModel)
	return handler.Success(c, &fileResp)

}

// 保存链接的文件
func FileSaveByLinkUrl(c *fiber.Ctx) error {
	reqBody := new(rtype.SaveLinkFileReqBodyParam)
	if err := c.BodyParser(reqBody); err != nil {
		return err
	}
	filename, extName := utils.ParseFileFullName(path.Base(reqBody.Url))
	fileModel := mysql.GenFileModel(filename, extName)
	setUploaderID(c, &fileModel)
	err := utils.DownloadRemoteFile(reqBody.Url, fileModel.GetTempSavePath(), utils.DefaultMaxRemoteFileBytes)
	if err != nil {
		return err
	}
	err = duplicateSaveTempFileModel(&fileModel)
	if err != nil {
		return err
	}
	res := &rtype.SaveLinkFileResponse{
		OriginalURL: reqBody.Url,
		URL:         fileModel.GetUrlPath(),
	}
	return handler.Success(c, res)
}

func FilePageList(c *fiber.Ctx) error {
	queryParam := service.FilePageListParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	pageParam, err := handler.ParsePageFindParam(c)
	if err != nil {
		return ParseErrorResponse(c, err.Error())
	}
	queryParam.PageFindParam = pageParam
	pageInfo, files, err := service.S.File.PageList(&queryParam, fileUser(c))
	if err != nil {
		return err
	}
	return handler.PaginateData(c, pageInfo, files)
}

func FileInfoUpdate(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &mysql.FileModel{})
	if err != nil {
		return err
	}
	param := service.UpdateFileParam{}
	if err = c.BodyParser(&param); err != nil {
		return err
	}
	file, err := service.S.File.UpdateFile(id, &param, fileUser(c))
	if err != nil {
		return fileAccessError(c, err)
	}
	return handler.Success(c, &file)
}

func FileDelete(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &mysql.FileModel{})
	if err != nil {
		return err
	}
	err = service.S.File.Delete(id, fileUser(c))
	if err != nil {
		return fileAccessError(c, err)
	}
	return handler.SuccessWithStatus(c, fiber.StatusOK, nil)
}

func FileUsages(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &mysql.FileModel{})
	if err != nil {
		return err
	}
	usages, err := service.S.File.Usages(id, fileUser(c))
	if err != nil {
		return fileAccessError(c, err)
	}
	return handler.Success(c, usages)
}

// TODO: 修改这些逻辑代码到 service 层
func saveFile(c *fiber.Ctx, file *multipart.FileHeader, fileModel *mysql.FileModel) error {
	setUploaderID(c, fileModel)
	// 先保存到 temp 文件夹中
	if err := c.SaveFile(file, fileModel.GetTempSavePath()); err != nil {
		return err
	}
	return duplicateSaveTempFileModel(fileModel)
}

func setUploaderID(c *fiber.Ctx, fileModel *mysql.FileModel) {
	user := fileUser(c)
	if user != nil {
		fileModel.UploaderID = user.ID
	}
}

func fileUser(c *fiber.Ctx) *model.User {
	user, ok := c.Locals("user").(*model.User)
	if ok && user != nil {
		return user
	}
	return nil
}

func fileAccessError(c *fiber.Ctx, err error) error {
	if stderrors.Is(err, service.ErrFileForbidden) {
		return ValidateErrorResponse(c, err.Error())
	}
	if stderrors.Is(err, service.ErrFileInUse) {
		return ErrorResponse(c, fiber.StatusConflict, CommonBadError, err.Error())
	}
	return err
}

// 将保存在 temp 中的 file 根据 md5 进行去重
func duplicateSaveTempFileModel(fileModel *mysql.FileModel) (err error) {
	if strings.TrimSpace(fileModel.Name) == "" || utf8.RuneCountInString(fileModel.Name) > 255 ||
		utf8.RuneCountInString(fileModel.ExtensionName) > 32 {
		_ = os.Remove(fileModel.GetTempSavePath())
		return stderrors.New("invalid file name")
	}
	// 检查文件是否存在以及查询是否已经有相同 md5 的文件
	fileMD5, err := fileModel.GetTempSaveFileMD5()
	if err != nil {
		return err
	}
	fileModel.MD5 = fileMD5
	sampleFileModels, err := dal.MySqlDB.File.FindFilesByMD5s([]string{fileMD5}, fileModel.UploaderID)
	if err != nil {
		return err
	}
	if len(sampleFileModels) != 0 {
		delErr := os.Remove(fileModel.GetTempSavePath())
		if delErr != nil {
			log.Printf("FileUpload delete temp file err, err: %+v", delErr)
		}
		*fileModel = *sampleFileModels[0]
		return nil
	}
	defer func() {
		if err == nil {
			return
		}
		if delErr := fileModel.HardDelete(); delErr != nil && !os.IsNotExist(delErr) {
			log.Printf("FileUpload delete stored file err, err: %+v", delErr)
		}
		if delErr := dal.MySqlDB.File.DeleteByUUID(fileModel.UUID); delErr != nil {
			log.Printf("FileUpload del file model err, err: %+v", delErr)
		}
	}()

	err = fileModel.CreateDirIfNotExist()
	if err != nil {
		return err
	}
	if err = os.Rename(fileModel.GetTempSavePath(), fileModel.GetActualSavePath()); err != nil {
		log.Printf("FileUpload rename file err, err: %+v", err)
		return err
	}
	if err = fileModel.PopulateImageMetadata(fileModel.GetActualSavePath()); err != nil {
		return err
	}
	if err = dal.MySqlDB.File.Create(fileModel); err != nil {
		return err
	}
	return nil
}
