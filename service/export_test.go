package service

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"myecho/dal/mysql"
)

func TestIsArticlePubliclyVisible_BitsUT(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		status   int8
		postTime time.Time
		want     bool
	}{
		{name: "公开文章立即可见", status: int8(mysql.ARTILCE_STATUS_PUBLIC), postTime: now, want: true},
		{name: "置顶文章立即可见", status: int8(mysql.ARTICLE_STATUS_TOP), postTime: now, want: true},
		{name: "未来发布的公开文章不可见", status: int8(mysql.ARTILCE_STATUS_PUBLIC), postTime: now.Add(time.Hour), want: false},
		{name: "草稿即使到期也不可见", status: int8(mysql.ARTICLE_STATUS_DRAFT), postTime: now.Add(-time.Hour), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsArticlePubliclyVisible(tt.status, tt.postTime); got != tt.want {
				t.Fatalf("IsArticlePubliclyVisible(%d, %v) = %v, want %v", tt.status, tt.postTime, got, tt.want)
			}
		})
	}
}

func TestIsSensitiveSettingKey_BitsUT(t *testing.T) {
	tests := map[string]bool{
		"SiteTitle":              false,
		"smtp_password":          true,
		"API-Key":                true,
		"oauth.client.secret":    true,
		ArticlePasswordSecretKey: true,
		themePreviewSecretKey:    true,
		"public_key":             false,
	}
	for key, want := range tests {
		t.Run(key, func(t *testing.T) {
			if got := isSensitiveSettingKey(key); got != want {
				t.Fatalf("isSensitiveSettingKey(%q) = %v, want %v", key, got, want)
			}
		})
	}
}

func TestExportLimitWriterWrite_BitsUT(t *testing.T) {
	var output bytes.Buffer
	remaining := int64(4)
	writer := &exportLimitWriter{Writer: &output, Remaining: &remaining}

	n, err := writer.Write([]byte("abc"))
	if err != nil || n != 3 || remaining != 1 || output.String() != "abc" {
		t.Fatalf("Write(within limit) n=%d err=%v remaining=%d output=%q", n, err, remaining, output.String())
	}
	n, err = writer.Write([]byte("de"))
	if !errors.Is(err, ErrExportTooLarge) || n != 0 {
		t.Fatalf("Write(over limit) n=%d err=%v, want 0 and %v", n, err, ErrExportTooLarge)
	}
	if remaining != 1 || output.String() != "abc" {
		t.Fatalf("Write(over limit) mutated state: remaining=%d output=%q", remaining, output.String())
	}
}
