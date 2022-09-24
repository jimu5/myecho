package handler

import (
	"github.com/gofiber/fiber/v2"
	"log"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"path"
	"strconv"
	"time"
)

func UploadFile(c *fiber.Ctx) error {
	form, err := c.MultipartForm()
	if err != nil {
		return err
	}
	files := form.File["file[]"]
	now := time.Now()
	failedFileName := make([]string, 0)
	successFileMap := make(map[string]string, len(files))
	for _, file := range files {
		extName := path.Ext(file.Filename)
		modelFileName := file.Filename[0 : len(file.Filename)-len(extName)]
		modelFilePath := path.Join(strconv.FormatInt(int64(now.Year()), 10), strconv.FormatInt(int64(now.Month()), 10))
		fileModel := &mysql.FileModel{
			Name:          modelFileName,
			ExtensionName: extName,
			Path:          modelFilePath,
		}
		err = dal.MySqlDB.File.Create(fileModel)
		if err != nil {
			failedFileName = append(failedFileName, file.Filename)
			continue
		}
		err = fileModel.CreateDirIfNotExist()
		if err != nil {
			return err
		}
		if err = c.SaveFile(file, fileModel.GetActualPath()); err != nil {
			log.Printf("FileUpload  upload file err, err: %+v", err)
			delErr := dal.MySqlDB.File.DeleteByUUID(fileModel.UUID)
			if delErr != nil {
				log.Printf("FileUpload del file model err, err: %+v", delErr)
			}
			failedFileName = append(failedFileName, file.Filename)
			continue
		}
		// 如果后面出现相同的 filename
		if _, ok := successFileMap[file.Filename]; !ok {
			successFileMap[file.Filename] = fileModel.GetUrlPath()
		}
	}
	resp := &rtype.UploadFileResponse{
		ErrFiles: failedFileName,
		SuccMap:  successFileMap,
	}
	return c.JSON(GetSuccessCommonResp(resp))
}
