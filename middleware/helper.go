package middleware

import (
	"myecho/config"
	"myecho/model"
)

// 通过token获取用户
func GetUserByToken(token string) (model.User, error) {
	var user model.User
	err := config.Database.Where("token = ?", token).First(&user).Error
	if err != nil {
		return user, err
	}
	return user, nil
}
