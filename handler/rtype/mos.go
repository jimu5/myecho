package rtype

type UploadFileResponse struct {
	Code     int               `json:"code"`
	Msg      string            `json:"msg"`
	ErrFiles []string          `json:"errFiles"`
	SuccMap  map[string]string `json:"succMap"`
}
