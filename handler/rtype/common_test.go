package rtype

import (
	"errors"
	apierrors "myecho/handler/api/errors"
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
}

func TestLinkQueryParamToDALParam(t *testing.T) {
	uid := "category"
	got := (&LinkQueryParam{CategoryUID: &uid}).ToDALParam()
	if got.CategoryUID == nil || *got.CategoryUID != uid {
		t.Fatalf("ToDALParam() = %+v, want category uid", got)
	}
}
