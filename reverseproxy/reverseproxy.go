package reverseproxy

import (
	"bytes"
	"encoding/json"
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
	proxy *httputil.ReverseProxy
	cache ReverseProxyCache
	token string
}

type ReverseProxyCache interface {
	Set(key string, data []byte) error
	Get(key string) ([]byte, bool)
}

//New instace of a ReverseProxy
func New(target, bearerToken string, cache ReverseProxyCache) *ReverseProxy {
	url, _ := url.Parse(target)
	return &ReverseProxy{
		proxy: httputil.NewSingleHostReverseProxy(url),
		cache: cache,
		token: bearerToken,
	}
}

// HandleRequest will be handle the request via the reverse proxy
func (rp *ReverseProxy) HandleRequest(res http.ResponseWriter, req *http.Request) {
	rp.serveReverseProxy(res, req)
}

func (rp *ReverseProxy) serveReverseProxy(res http.ResponseWriter, req *http.Request) {
	req.URL.Path = strings.Replace(req.URL.Path, "/proxy", "", -1)
	fullURL := req.Method + req.URL.Path + "?" + req.URL.RawQuery

	log.Print(fullURL)

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
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	rp.proxy.ModifyResponse = func(h *http.Response) error {
		buffer := &bytes.Buffer{}
		_, err := io.Copy(buffer, h.Body)
		if err != nil {
			return err
		}

		data := buffer.Bytes()

		h.Body = ioutil.NopCloser(bytes.NewBuffer(data))

		cacheItem := ReverseProxyCacheItem{
			ContentType: h.Header.Get("Content-Type"),
			Body:        string(data),
		}

		bytes, err := json.Marshal(cacheItem)

		if err != nil {
			return err
		}

		return rp.cache.Set(fullURL, bytes)
	}

	if rp.token != "" {
		req.Header.Set("Authorization", rp.token)
	}

	rp.proxy.ServeHTTP(res, req)

}
