package models

type Resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
		Type    string `json:"type"`
	} `json:"error"`
}
