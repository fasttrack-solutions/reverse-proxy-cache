package reverseproxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type ReverseProxyCacheItem struct {
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

type ReverseProxy struct {
	proxy          *httputil.ReverseProxy
	cache          ReverseProxyCache
	token          string
	removeFromPath string
}

type ReverseProxyCache interface {
	Set(key string, data []byte) error
	Get(key string) ([]byte, bool)
}

type DebugTransport struct{}

func (DebugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, err := httputil.DumpRequestOut(r, false)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(b))
	return http.DefaultTransport.RoundTrip(r)
}

//New instace of a ReverseProxy
func New(target, bearerToken string, cache ReverseProxyCache, removeFromPath string) *ReverseProxy {
	url, _ := url.Parse(target)

	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = DebugTransport{}

	return &ReverseProxy{
		proxy:          proxy,
		cache:          cache,
		token:          bearerToken,
		removeFromPath: removeFromPath,
	}
}

// HandleRequest will be handle the request via the reverse proxy
func (rp *ReverseProxy) HandleRequest(res http.ResponseWriter, req *http.Request) {
	rp.serveReverseProxy(res, req)
}

func IsSuccess(h *http.Response) bool {
	return h.StatusCode > 199 && h.StatusCode < 300
}

func (rp *ReverseProxy) serveReverseProxy(res http.ResponseWriter, req *http.Request) {
	req.URL.Path = strings.Replace(req.URL.Path, "/proxy", "", -1)
	req.URL.Path = strings.Replace(req.URL.Path, rp.removeFromPath, "", -1)
	fullURL := req.Method + req.URL.Path + "?" + req.URL.RawQuery
	req.Host = req.URL.Host

	log.Printf("getting FullURL: %s", fullURL)

	data, exists := rp.cache.Get(fullURL)

	if exists {

		cacheItem := ReverseProxyCacheItem{}

		err := json.Unmarshal(data, &cacheItem)
		if err != nil {
			panic(err)
		}

		res.Header().Add("Content-Type", cacheItem.ContentType)

		reader := bytes.NewReader([]byte(cacheItem.Body))
		_, err = io.Copy(res, reader)
		if err != nil {
			panic(err)
		}
		return

	}

	// Update the headers to allow for SSL redirection
	//req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	rp.proxy.ModifyResponse = func(h *http.Response) error {

		if !IsSuccess(h) {
			return nil
		}

		buffer := &bytes.Buffer{}

		_, err := io.Copy(buffer, h.Body)
		if err != nil {
			return err
		}

		encoding := h.Header.Get("Content-Encoding")
		data := buffer.Bytes()

		h.Body = ioutil.NopCloser(bytes.NewBuffer(data))

		if encoding == "gzip" {
			data, err = gUnzipData(data)
			if err != nil {
				return err
			}
		}

		cacheItem := ReverseProxyCacheItem{
			ContentType: h.Header.Get("Content-Type"),
			Body:        string(data),
		}

		bytes, err := json.Marshal(cacheItem)

		if err != nil {
			return err
		}

		req := h.Request

		fullURL := req.Method + req.URL.Path + "?" + req.URL.RawQuery
		log.Printf("setting FullURL: %s", fullURL)

		return rp.cache.Set(fullURL, bytes)
	}

	if rp.token != "" {
		req.Header.Set("Authorization", rp.token)
	}

	rp.proxy.ServeHTTP(res, req)

}

func gUnzipData(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}
