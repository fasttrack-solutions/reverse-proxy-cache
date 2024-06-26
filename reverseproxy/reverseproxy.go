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
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

type ReverseProxyCacheItem struct {
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

type ReverseProxy struct {
	proxy           *httputil.ReverseProxy
	cache           ReverseProxyCache
	token           string
	removeFromPath  string
	pathEncodeAfter string
	rateLimit       map[string]time.Time
}

type ReverseProxyCache interface {
	Set(key string, data []byte) error
	Get(key string) ([]byte, bool)
}

type DebugTransport struct {
	pathEncodeAfter string
}

func (d DebugTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, err := httputil.DumpRequestOut(r, false)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(b))

	r.URL.RawPath = d.pathEncode(r.URL.Path)
	fmt.Println("debugtransport path" + r.URL.Path)
	fmt.Println("debugtransport raw path" + r.URL.RawPath)
	fmt.Println("debugtransport full url:" + r.URL.String())

	return http.DefaultTransport.RoundTrip(r)
}

func (d DebugTransport) pathEncode(path string) string {
	if d.pathEncodeAfter == "" {
		return path
	}

	i := strings.LastIndex(path, d.pathEncodeAfter)

	if i == -1 {
		return path
	}

	endingIndex := len(d.pathEncodeAfter) + i

	res := path[:endingIndex] + url.QueryEscape(path[endingIndex:])

	return res
}

// New instace of a ReverseProxy
func New(target, bearerToken string, cache ReverseProxyCache, removeFromPath, pathEncodeAfter string) *ReverseProxy {
	url, _ := url.Parse(target)

	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = DebugTransport{
		pathEncodeAfter: pathEncodeAfter,
	}

	return &ReverseProxy{
		proxy:           proxy,
		cache:           cache,
		token:           bearerToken,
		removeFromPath:  removeFromPath,
		pathEncodeAfter: pathEncodeAfter,
		rateLimit:       make(map[string]time.Time),
	}
}

// HandleRequest will be handle the request via the reverse proxy
func (rp *ReverseProxy) HandleRequest(res http.ResponseWriter, req *http.Request) {
	rp.serveReverseProxy(res, req)
}

func IsSuccess(h *http.Response) bool {
	return (h.StatusCode > 199 && h.StatusCode < 300) || h.StatusCode == 404
}

func (rp *ReverseProxy) serveReverseProxy(res http.ResponseWriter, req *http.Request) {
	req.URL.Path = strings.Replace(req.URL.Path, "/proxy", "", -1)
	req.URL.Path = strings.Replace(req.URL.Path, rp.removeFromPath, "", -1)
	fullURL := req.Method + req.URL.Path + "?" + req.URL.RawQuery
	req.Host = req.URL.Host

	if v, exists := rp.rateLimit[req.URL.Host]; exists {
		if time.Now().Before(v) {
			log.Printf("Rate limit exceeded, resetting at %s", v)
			http.Error(res, "Rate limit exceeded", 429)
			return
		}
	}

	log.Printf("getting FullURL: %s replaced %s path is: %s", fullURL, rp.removeFromPath, req.URL.Path)

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
			if h.StatusCode == 429 {
				resetAt := h.Header.Get("X-Ratelimit-Reset")
				i, err := strconv.ParseInt(resetAt, 10, 64)
				if err != nil {
					panic(err)
				}
				tm := time.Unix(i, 0)
				rp.rateLimit[req.URL.Host] = tm
				log.Printf("Rate limit exceeded, resetting at %s", tm)
			}
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

		if encoding == "br" {
			data, err = decodeBortil(data)
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
		log.Printf("whats the encoding? %s", encoding)

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

func decodeBortil(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r = brotli.NewReader(b)

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}
