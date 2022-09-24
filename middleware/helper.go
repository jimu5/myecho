package middleware

import (
	"myecho/dal/connect"
	"myecho/model"
)

// 通过token获取用户
func GetUserByToken(token string) (model.User, error) {
	var user model.User
	err := connect.Database.Where("token = ?", token).First(&user).Error
	if err != nil {
		return user, err
	}
	return user, nil
}
