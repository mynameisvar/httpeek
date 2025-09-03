package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
)

type Proxy struct{}

func NewProxy() *Proxy {
	return &Proxy{}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	if err == nil {
		fmt.Println("====== Incoming Request ======")
		fmt.Println(string(dump))
		fmt.Println("==============================")
	}

	targetURL := r.URL
	if !targetURL.IsAbs() {
		http.Error(w, "Bad Request: URL must be absolute", http.StatusBadRequest)
		return
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	proxyReq.Header = r.Header.Clone()

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respDump, err := httputil.DumpResponse(resp, true)
	if err == nil {
		fmt.Println("===== Response =====")
		fmt.Println(string(respDump))
		fmt.Println("====================")
	}

	// response -> client
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
