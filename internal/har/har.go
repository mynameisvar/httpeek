package har

import (
	"encoding/base64"
	"time"

	"httpeek/internal/types"
)

type Document struct {
	Log Log `json:"log"`
}

type Log struct {
	Version string  `json:"version"`
	Creator Creator `json:"creator"`
	Entries []Entry `json:"entries"`
}

type Creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Entry struct {
	StartedDateTime time.Time `json:"startedDateTime"`
	Time            int64     `json:"time"` // ms
	Request         Req       `json:"request"`
	Response        Resp      `json:"response"`
}
type Header struct{ Name, Value string }
type PostData struct{ MimeType, Text, Encoding string }
type Content struct {
	Size                     int `json:"size"`
	MimeType, Text, Encoding string
}

type Req struct {
	Method      string   `json:"method"`
	URL         string   `json:"url"`
	HTTPVersion string   `json:"httpVersion"`
	Headers     []Header `json:"headers"`
	PostData    PostData `json:"postData"`
}

type Resp struct {
	Status      int      `json:"status"`
	HTTPVersion string   `json:"httpVersion"`
	Headers     []Header `json:"headers"`
	Content     Content  `json:"content"`
}

func FromEntries(in []*types.Entry) Document {
	out := Document{
		Log: Log{
			Version: "1.1",
			Creator: Creator{Name: "HTTPeek", Version: "0.2"},
			Entries: make([]Entry, 0, len(in)),
		},
	}
	for _, e := range in {
		reqBodyB64 := base64.StdEncoding.EncodeToString(e.ReqBody)
		respBodyB64 := base64.StdEncoding.EncodeToString(e.RespBody)
		out.Log.Entries = append(out.Log.Entries, Entry{
			StartedDateTime: e.StartedAt,
			Time:            e.Duration.Milliseconds(),
			Request: Req{
				Method:      e.Method,
				URL:         e.URL,
				HTTPVersion: e.HTTPVersion,
				Headers:     toH(e.ReqHeaders),
				PostData:    PostData{MimeType: "", Text: reqBodyB64, Encoding: "base64"},
			},
			Response: Resp{
				Status:      e.Status,
				HTTPVersion: e.HTTPVersion,
				Headers:     toH(e.RespHeaders),
				Content:     Content{Size: len(e.RespBody), MimeType: "", Text: respBodyB64, Encoding: "base64"},
			},
		})
	}
	return out
}

func toH(in []types.KV) []Header {
	out := make([]Header, 0, len(in))
	for _, kv := range in {
		out = append(out, Header{Name: kv.Key, Value: kv.Value})
	}
	return out
}
