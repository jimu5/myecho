package model

type Setting struct {
	BaseModel
	Key         string `json:"key" gorm:"size:255"`
	Value       string `json:"value" gorm:"type:text"`
	Type        string `json:"type" gorm:"size:20"`
	Description string `json:"description" gorm:"size:255"`
	IsSystem    bool   `json:"is_system"`
}

const (
	SettingModelTypeInt    = "int"
	SettingModelTypeString = "string"
	SettingModelTypeBool   = "bool"
)
