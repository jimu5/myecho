package model

import (
	"encoding/json"
)

// Theme 主题模型
// 用于存储博客系统的主题配置
// 包含主题名称、作者、版本等基本信息
// 以及主题的CSS、JS和配置数据
// 支持主题的启用/禁用状态
// 支持自定义配置项
// 用于实现博客系统的主题切换功能

type Theme struct {
	BaseModel
	Name        string `json:"name" gorm:"size:100;not null;uniqueIndex"`
	DisplayName string `json:"display_name" gorm:"size:100;not null"`
	Author      string `json:"author" gorm:"size:100"`
	Version     string `json:"version" gorm:"size:20"`
	Description string `json:"description" gorm:"type:text"`
	Preview     string `json:"preview" gorm:"type:text"`
	CSS         string `json:"css" gorm:"type:text"`
	JS          string `json:"js" gorm:"type:text"`
	IsDefault   bool   `json:"is_default" gorm:"default:false"`
	IsActive    bool   `json:"is_active" gorm:"default:false"`
	Config      []byte `json:"config" gorm:"type:text"` // 使用 []byte 存储JSON数据
}

// GetConfig 解析 Config 字段为 map[string]interface{}
func (t *Theme) GetConfig() (map[string]interface{}, error) {
	if len(t.Config) == 0 {
		return make(map[string]interface{}), nil
	}
	var config map[string]interface{}
	if err := json.Unmarshal(t.Config, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// SetConfig 将 map[string]interface{} 转换为 JSON 并存储
func (t *Theme) SetConfig(config map[string]interface{}) error {
	if config == nil {
		t.Config = nil
		return nil
	}
	jsonData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	t.Config = jsonData
	return nil
}