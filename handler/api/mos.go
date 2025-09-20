package api

import (
	"crypto/tls"
	"io"
	"log"
	"mime/multipart"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"myecho/service"
	"myecho/utils"
	"net/http"
	"os"
	"path"

	"github.com/gofiber/fiber/v2"
)

var httpClient = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

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
	return c.JSON(handler.GetSuccessCommonResp(resp))
}

func FileSingleUpload(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	files := form.File["file"]
	if len(files) == 0 {
		return nil
	}
	file := files[0]
	fileModel := mysql.GenFileModel(utils.ParseFileFullName(file.Filename))
	if err = saveFile(c, file, &fileModel); err != nil {
		return err
	}
	fileResp := service.ModelToFile(&fileModel)
	return c.JSON(&fileResp)

}

// 保存链接的文件
func FileSaveByLinkUrl(c *fiber.Ctx) error {
	reqBody := new(rtype.SaveLinkFileReqBodyParam)
	if err := c.BodyParser(reqBody); err != nil {
		return err
	}
	extName := path.Ext(reqBody.Url)
	filename := path.Base(reqBody.Url)
	fileModel := mysql.GenFileModel(filename, extName)
	out, err := os.Create(fileModel.GetTempSavePath()) // 保存在临时文件
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := httpClient.Get(reqBody.Url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	out.Close()
	err = duplicateSaveTempFileModel(&fileModel)
	if err != nil {
		return err
	}
	res := &rtype.SaveLinkFileResponse{
		OriginalURL: reqBody.Url,
		URL:         fileModel.GetUrlPath(),
	}
	return c.JSON(handler.GetSuccessCommonResp(res))
}

func FilePageList(c *fiber.Ctx) error {
	queryParam := service.FilePageListParam{}
	if err := c.QueryParser(&queryParam); err != nil {
		return err
	}
	pageInfo, files, err := service.S.File.PageList(&queryParam)
	if err != nil {
		return err
	}
	return handler.PaginateData(c, pageInfo.Total, files)
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
	file, err := service.S.File.UpdateFile(id, &param)
	if err != nil {
		return err
	}
	return c.JSON(&file)
}

func FileDelete(c *fiber.Ctx) error {
	id, err := handler.GetIDByParam(c, &mysql.FileModel{})
	if err != nil {
		return err
	}
	err = service.S.File.Delete(id)
	if err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// TODO: 修改这些逻辑代码到 service 层
func saveFile(c *fiber.Ctx, file *multipart.FileHeader, fileModel *mysql.FileModel) error {
	// 先保存到 temp 文件夹中
	if err := c.SaveFile(file, fileModel.GetTempSavePath()); err != nil {
		return err
	}
	return duplicateSaveTempFileModel(fileModel)
}

// 将保存在 temp 中的 file 根据 md5 进行去重
func duplicateSaveTempFileModel(fileModel *mysql.FileModel) error {
	// 检查文件是否存在以及查询是否已经有相同 md5 的文件
	fileMD5, err := fileModel.GetTempSaveFileMD5()
	if err != nil {
		return err
	}
	fileModel.MD5 = fileMD5
	sampleFileModels, err := dal.MySqlDB.File.FindFilesByMD5s([]string{fileMD5})
	if len(sampleFileModels) != 0 {
		delErr := os.Remove(fileModel.GetTempSavePath())
		if delErr != nil {
			log.Printf("FileUpload delete temp file err, err: %+v", delErr)
		}
		fileModel.UUID = sampleFileModels[0].UUID
		fileModel.DirPath = sampleFileModels[0].DirPath
		fileModel.ID = sampleFileModels[0].ID
		return nil
	}
	err = dal.MySqlDB.File.Create(fileModel)
	if err != nil {
		return err
	}

	defer func(err *error) {
		if *err != nil {
			delErr := dal.MySqlDB.File.DeleteByUUID(fileModel.UUID)
			if delErr != nil {
				log.Printf("FileUpload del file model err, err: %+v", delErr)
			}
		}
	}(&err)

	err = fileModel.CreateDirIfNotExist()
	if err != nil {
		return err
	}
	if err = os.Rename(fileModel.GetTempSavePath(), fileModel.GetActualSavePath()); err != nil {
		log.Printf("FileUpload rename file err, err: %+v", err)
		return err
	}
	return nil
}
