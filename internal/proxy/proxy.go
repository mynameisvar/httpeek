package proxy

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elazarl/goproxy"

	"httpeek/internal/storage"
	"httpeek/internal/types"
)

const (
	maxBodyCapture = 512 * 1024 // 512KB
)

func New(store *storage.Store, dataDir string) (http.Handler, string) {
	caCertPath := filepath.Join(dataDir, "ca.pem")
	caKeyPath := filepath.Join(dataDir, "ca.key")

	cert, key, created, err := ensureCA(caCertPath, caKeyPath)
	if err != nil {
		log.Println("CA error:", err)
	}
	if created {
		log.Println("Generated new root CA at:", caCertPath)
	}

	err = mitmConfig(cert, key)
	if err != nil {
		log.Println("mitm config error:", err)
	}

	p := goproxy.NewProxyHttpServer()
	p.Verbose = false

	if goproxy.GoproxyCa.Certificate != nil && len(goproxy.GoproxyCa.Certificate) > 0 {
		p.OnRequest().HandleConnect(goproxy.AlwaysMitm)
	} else {
		p.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
			return goproxy.OkConnect, host
		})
	}

	p.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		ctx.UserData = &capture{
			start:  time.Now(),
			method: req.Method,
			url:    req.URL.String(),
			host:   req.Host,
			scheme: schemeOf(req),
			ver:    req.Proto,
			reqH:   cloneHeader(req.Header),
			reqB:   readLimited(req.Body, maxBodyCapture),
		}

		if c, ok := ctx.UserData.(*capture); ok && c.reqB != nil {
			req.Body = io.NopCloser(bytes.NewReader(c.reqB))
		}
		return req, nil
	})

	p.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		c, _ := ctx.UserData.(*capture)
		if c == nil {
			return resp
		}
		var status int
		if resp != nil {
			status = resp.StatusCode
		}
		var respBody []byte
		if resp != nil && resp.Body != nil {
			respBody = readLimited(resp.Body, maxBodyCapture)
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
		}

		entry := &types.Entry{
			StartedAt:     c.start,
			Duration:      time.Since(c.start),
			Method:        c.method,
			URL:           c.url,
			HTTPVersion:   c.ver,
			ReqHeaders:    toKV(c.reqH),
			ReqBody:       c.reqB,
			ReqBodyTrunc:  c.reqTrunc,
			Status:        status,
			RespHeaders:   toKV(headerOf(resp)),
			RespBody:      respBody,
			RespBodyTrunc: len(respBody) >= maxBodyCapture,
			Error:         "",
			Host:          c.host,
			Scheme:        c.scheme,
		}
		if resp == nil {
			entry.Error = "no response (connection error)"
		}

		if _, err := store.Put(entry); err != nil {
			log.Println("store put:", err)
		}

		notifySSE(entry) // push live update
		return resp
	})

	p.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		return goproxy.OkConnect, host
	})

	return p, caCertPath
}

type capture struct {
	start            time.Time
	method, url, ver string
	host, scheme     string
	reqH             http.Header
	reqB             []byte
	reqTrunc         bool
}

func schemeOf(r *http.Request) string {
	if r.URL != nil && r.URL.Scheme != "" {
		return r.URL.Scheme
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func readLimited(rc io.ReadCloser, limit int) []byte {
	if rc == nil {
		return nil
	}
	defer rc.Close()
	var buf bytes.Buffer
	n, _ := io.CopyN(&buf, rc, int64(limit))
	_ = n
	b := buf.Bytes()
	return b
}

func cloneHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, v := range h {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func toKV(h http.Header) []types.KV {
	if h == nil {
		return nil
	}
	res := make([]types.KV, len(h))
	for k, vs := range h {
		res = append(res, types.KV{Key: k, Value: strings.Join(vs, ", ")})
	}
	return res
}

func headerOf(resp *http.Response) http.Header {
	if resp == nil {
		return nil
	}
	return resp.Header
}

func ensureCA(certPath, keyPath string) (certPEM, keyPEM []byte, created bool, err error) {
	// if cert exists -> read
	if _, e := os.Stat(certPath); e == nil {
		certPEM, err = os.ReadFile(certPath)
		if err != nil {
			return nil, nil, false, err
		}
		keyPEM, err = os.ReadFile(keyPath)
		if err != nil {
			return nil, nil, false, err
		}
		return certPEM, keyPEM, false, nil
	}

	// generate new cert
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, false, err
	}

	serialNumber, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "HTTPeek Root CA",
			Organization: []string{"HTTPeek"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(5, 0, 0), // 5 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, false, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, nil, false, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, nil, false, err
	}

	return certPEM, keyPEM, true, nil
}

func mitmConfig(caCertPEM, caKeyPEM []byte) error {
	cert, err := tls.X509KeyPair(caCertPEM, caKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to load CA key pair: %w", err)
	}

	block, _ := pem.Decode(caCertPEM)
	if block == nil {
		return fmt.Errorf("failed to decode CA certificate PEM")
	}
	cert.Leaf, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	goproxy.GoproxyCa = cert
	return nil
}

/**
 * sse broadcaster
 */
var subscribers = make(map[chan *types.Entry]struct{})

func Subscribe() chan *types.Entry {
	ch := make(chan *types.Entry, 32)
	subscribers[ch] = struct{}{}
	return ch
}

func Unsubscribe(ch chan *types.Entry) {
	delete(subscribers, ch)
	close(ch)
}

func notifySSE(e *types.Entry) {
	for ch := range subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}
