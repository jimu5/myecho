package model

import (
	"encoding/json"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestCategoryTypeValidity(t *testing.T) {
	if !CategoryTypeArticle.IsCategoryTypeValid() {
		t.Fatal("article category type should be valid")
	}
	if !CategoryTypeLink.IsCategoryTypeValid() {
		t.Fatal("link category type should be valid")
	}
	if CategoryType(0).IsCategoryTypeValid() || CategoryType(3).IsCategoryTypeValid() {
		t.Fatal("out-of-range category type should be invalid")
	}
}

func TestThemeConfigRoundTrip(t *testing.T) {
	theme := &Theme{}
	empty, err := theme.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() empty error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty config = %#v, want empty map", empty)
	}

	want := map[string]interface{}{"primary": "#123456", "enabled": true}
	if err := theme.SetConfig(want); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	got, err := theme.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}
	if got["primary"] != want["primary"] || got["enabled"] != want["enabled"] {
		t.Fatalf("GetConfig() = %#v, want %#v", got, want)
	}

	if err := theme.SetConfig(nil); err != nil {
		t.Fatalf("SetConfig(nil) error = %v", err)
	}
	if theme.Config != nil {
		t.Fatalf("SetConfig(nil) left config = %s", string(theme.Config))
	}
}

func TestThemeGetConfigRejectsInvalidJSON(t *testing.T) {
	theme := &Theme{Config: []byte("{")}
	if _, err := theme.GetConfig(); err == nil {
		t.Fatal("GetConfig() expected invalid JSON error")
	}
}

func TestGenerateRandomStringAndUserToken(t *testing.T) {
	token := GenerateRandomString(32)
	if len(token) != 32 {
		t.Fatalf("GenerateRandomString() length = %d, want 32", len(token))
	}
	if strings.Trim(token, "0123456789abcdefghijklmnopqrstuvwxyz") != "" {
		t.Fatalf("GenerateRandomString() returned unexpected characters: %q", token)
	}

	user := &User{}
	if err := user.BeforeSave(&gorm.DB{}); err != nil {
		t.Fatalf("BeforeSave() error = %v", err)
	}
	if len(user.Token) != 32 {
		t.Fatalf("generated token length = %d, want 32", len(user.Token))
	}
	existing := user.Token
	user.generateToken()
	if user.Token != existing {
		t.Fatalf("generateToken() replaced existing token")
	}
}

func TestArticleDetailBeforeCreateKeepsExistingUID(t *testing.T) {
	detail := &ArticleDetail{UID: "existing"}
	if err := detail.BeforeCreate(&gorm.DB{}); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if detail.UID != "existing" {
		t.Fatalf("BeforeCreate() UID = %q, want existing", detail.UID)
	}

	detail = &ArticleDetail{}
	if err := detail.BeforeCreate(&gorm.DB{}); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if len(detail.UID) != 20 {
		t.Fatalf("generated UID length = %d, want 20", len(detail.UID))
	}
}

func TestThemeSetConfigProducesJSON(t *testing.T) {
	theme := &Theme{}
	if err := theme.SetConfig(map[string]interface{}{"count": float64(2)}); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(theme.Config, &decoded); err != nil {
		t.Fatalf("stored config is not valid JSON: %v", err)
	}
	if decoded["count"] != float64(2) {
		t.Fatalf("decoded config = %#v", decoded)
	}
}
