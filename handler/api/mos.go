package api

import (
	"github.com/gofiber/fiber/v2"
	"io"
	"log"
	"mime/multipart"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler"
	"myecho/handler/rtype"
	"net/http"
	"os"
	"path"
)

func UploadFile(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	files := form.File["file[]"]
	failedFileName := make([]string, 0)
	successFileMap := make(map[string]string, len(files))
	for _, file := range files {
		extName := path.Ext(file.Filename)
		fileModel := mysql.GenFileModel(file.Filename[0:len(file.Filename)-len(extName)], extName)
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

// 保存链接的文件
func SaveLinkUrlFile(c *fiber.Ctx) error {
	reqBody := new(rtype.SaveLinkFileReqBodyParam)
	if err := c.BodyParser(reqBody); err != nil {
		return err
	}
	extName := path.Ext(reqBody.Url)
	filename := path.Base(reqBody.Url)
	fileModel := mysql.GenFileModel(filename[0:len(filename)-len(extName)], extName)
	out, err := os.Create(fileModel.GetTempSavePath()) // 保存在临时文件
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(reqBody.Url)
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
