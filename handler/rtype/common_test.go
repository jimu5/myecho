package rtype

import (
	"errors"
	apierrors "myecho/handler/api/errors"
	"myecho/model"
	"testing"
)

func TestSettingCreateReqValidate(t *testing.T) {
	if err := (&SettingCreateReq{}).Validate(); !errors.Is(err, apierrors.ErrSettingKey) {
		t.Fatalf("Validate() error = %v, want ErrSettingKey", err)
	}
	if err := (&SettingCreateReq{Key: "site_name"}).Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCategoryCreateRequestToMysqlModel(t *testing.T) {
	req := &CategoryCreateRequest{Name: "Go", FatherUID: "parent"}
	got := req.ToMysqlModel()
	if got.Name != req.Name || got.FatherUID != req.FatherUID {
		t.Fatalf("ToMysqlModel() = %+v, want name/father from request", got)
	}
	if err := (&CategoryCreateRequest{}).Validate(); !errors.Is(err, apierrors.ErrCategoryNameEmpty) {
		t.Fatalf("Validate(empty name) error = %v, want ErrCategoryNameEmpty", err)
	}
	if err := (&CategoryCreateRequest{Name: "Go"}).Validate(); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}
}

func TestLinkQueryParamToDALParam(t *testing.T) {
	uid := "category"
	got := (&LinkQueryParam{CategoryUID: &uid}).ToDALParam()
	if got.CategoryUID == nil || *got.CategoryUID != uid {
		t.Fatalf("ToDALParam() = %+v, want category uid", got)
	}
}

func TestNewSettingVisibility_BitsUT(t *testing.T) {
	setting := model.Setting{
		BaseModel:   model.BaseModel{ID: 7},
		Key:         "SiteTitle",
		Value:       "Myecho",
		Type:        model.SettingModelTypeString,
		Description: "site title",
		IsSystem:    true,
	}
	for _, isPublic := range []bool{false, true} {
		got := NewSetting(setting, isPublic)
		if got.ID != setting.ID || got.Key != setting.Key || got.Value != setting.Value || got.Type != setting.Type ||
			got.Description != setting.Description || got.IsSystem != setting.IsSystem || got.IsPublic != isPublic {
			t.Fatalf("NewSetting(%v) = %+v", isPublic, got)
		}
	}
}
