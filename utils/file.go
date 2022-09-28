package utils

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

func GetFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		return "", err
	}
	hash := md5.New()
	_, _ = io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func CreateDirIfNotExist(dirPath string) error {
	_, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		createErr := os.MkdirAll(dirPath, os.ModePerm)
		if createErr != nil {
			return createErr
		}
	}
	return err
}
