package middleware

import (
	"github.com/Kimiato/myecho/config"
	"github.com/Kimiato/myecho/model"
)

// 通过token获取用户
func GetUserByToken(token string) (*model.User, error) {
	var user model.User
	err := config.Database.Where("token = ?", token).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
