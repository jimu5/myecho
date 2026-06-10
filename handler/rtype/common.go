package rtype

type CommonResp struct {
	Code int                    `json:"code"`
	Msg  string                 `json:"msg"`
	Data interface{}            `json:"data"`
	Meta map[string]interface{} `json:"meta"`
}
