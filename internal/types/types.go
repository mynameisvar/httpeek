package types

import "time"

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Entry struct {
	ID        string        `json:"id"`
	StartedAt time.Time     `json:"startedAt"`
	Duration  time.Duration `json:"duration"`

	Method      string `json:"method"`
	URL         string `json:"url"`
	HTTPVersion string `json:"httpVersion"`

	ReqHeaders   []KV   `json:"reqHeaders"`
	ReqBody      []byte `json:"reqBody"`
	ReqBodyTrunc bool   `json:"reqBodyTrunc"`

	Status        int    `json:"status"`
	RespHeaders   []KV   `json:"respHeaders"`
	RespBody      []byte `json:"respBody"`
	RespBodyTrunc bool   `json:"respBodyTrunc"`

	Error string `json:"error,omitempty"`

	Host   string `json:"host"`
	Scheme string `json:"scheme"`
}
