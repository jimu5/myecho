package rtype

import "time"

type ArticlePreviewCreateRequest struct {
	ExpectedEditVersion *uint64 `json:"expected_edit_version"`
}

type ArticlePreviewResponse struct {
	PreviewURL string    `json:"preview_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}
