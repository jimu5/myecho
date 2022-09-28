package handler

import (
	"github.com/gofiber/fiber/v2"
	"log"
	"mime/multipart"
	"myecho/dal"
	"myecho/dal/mysql"
	"myecho/handler/rtype"
	"os"
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
		fileModel := &mysql.FileModel{
			Name:          file.Filename[0 : len(file.Filename)-len(extName)],
			ExtensionName: extName,
			DirPath:       path.Join(strconv.FormatInt(int64(now.Year()), 10), strconv.FormatInt(int64(now.Month()), 10)),
		}
		// 如果后面出现相同的 filename
		if _, ok := successFileMap[file.Filename]; ok {
			failedFileName = append(failedFileName, file.Filename)
			continue
		}
		if err := saveFile(c, file, fileModel); err != nil {
			failedFileName = append(failedFileName, file.Filename)
		} else {
			successFileMap[file.Filename] = fileModel.GetUrlPath()
		}
	}
	resp := &rtype.UploadFileResponse{
		ErrFiles: failedFileName,
		SuccMap:  successFileMap,
	}
	return c.JSON(GetSuccessCommonResp(resp))
}

func SaveLinkImgUrl(c *fiber.Ctx) error {
	return nil
}

func saveFile(c *fiber.Ctx, file *multipart.FileHeader, fileModel *mysql.FileModel) error {
	// 先保存到 temp 文件夹中
	err := fileModel.CreateTempIfNotExist()
	if err != nil {
		return err
	}
	if err = c.SaveFile(file, fileModel.GetTempSavePath()); err != nil {
		return err
	}
	// 查询是否已经有相同 md5 的文件
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
