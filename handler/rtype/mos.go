package rtype

type UploadFileResponse struct {
	ErrFiles []string          `json:"errFiles"`
	SuccMap  map[string]string `json:"succMap"`
}

type SaveLinkFileReqBodyParam struct {
	Url string `json:"url" form:"url"`
}

type SaveLinkFileResponse struct {
	OriginalURL string `json:"originalURL"`
	URL         string `json:"url"`
}
