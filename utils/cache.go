package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	jsonnet "github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
)

var errNotFound = errors.New("Not found")

// cache implements a dumb local cache for files fetched remotely.
type httpCache struct {
	// The location of the cache directory
	cacheDir string
	// The http client used for requests
	httpClient *http.Client
}

func NewHTTPCache(cacheDir string) *httpCache {
	// Reconstructed copy of http.DefaultTransport (to avoid
	// modifying the default)
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
	t.RegisterProtocol("internal", http.NewFileTransport(newInternalFS("lib")))

	return &httpCache{
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Transport: t,
		},
	}
}

var httpRegex = regexp.MustCompile("^(https?)://")

func (h *httpCache) getLocalPath(url string) string {
	return filepath.Join(h.cacheDir, httpRegex.ReplaceAllString(url, ""))
}

func (h *httpCache) tryLocalCache(url string) (jsonnet.Contents, error) {
	localPath := h.getLocalPath(url)
	bytes, err := ioutil.ReadFile(localPath)
	if err != nil {
		return jsonnet.Contents{}, err
	}
	log.Debugf("Read %q from local cache at %q", url, localPath)
	return jsonnet.MakeContents(string(bytes)), nil
}

func (h *httpCache) writeToCache(url string, contents []byte) error {
	localPath := h.getLocalPath(url)
	localPathDir := filepath.Dir(localPath)
	finfo, err := os.Stat(localPathDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(localPathDir, 0755); err != nil {
			return err
		}
	}
	if err == nil && !finfo.IsDir() {
		return fmt.Errorf("%q is not a directory, it cannot be used for caching", localPathDir)
	}
	return ioutil.WriteFile(localPath, contents, 0644)
}

func (h *httpCache) Get(url string) (jsonnet.Contents, error) {
	isHTTP := httpRegex.MatchString(url)

	// If this is an http url, try the local cache first
	if isHTTP {
		contents, err := h.tryLocalCache(url)
		if err == nil {
			return contents, nil
		}
		log.Debugf("Error reading %q from local cache: %s", url, err)
	}

	// Attempt a normal GET
	res, err := h.httpClient.Get(url)
	if err != nil {
		return jsonnet.Contents{}, err
	}
	defer res.Body.Close()

	log.Debugf("GET %q -> %s", url, res.Status)
	if res.StatusCode == http.StatusNotFound {
		return jsonnet.Contents{}, errNotFound
	} else if res.StatusCode != http.StatusOK {
		return jsonnet.Contents{}, fmt.Errorf("error reading content: %s", res.Status)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return jsonnet.Contents{}, err
	}

	// If it was an http url, write the contents to the local cache
	if isHTTP {
		if err := h.writeToCache(url, bodyBytes); err != nil {
			log.Debugf("Error writing %q to the local cache: %s", url, err)
		}
	}

	return jsonnet.MakeContents(string(bodyBytes)), nil
}

//go:generate go-bindata -nometadata -ignore .*_test\.|~$DOLLAR -pkg $GOPACKAGE -o bindata.go -prefix ../ ../lib/...
func newInternalFS(prefix string) http.FileSystem {
	// Asset/AssetDir returns `fmt.Errorf("Asset %s not found")`,
	// which does _not_ get mapped to 404 by `http.FileSystem`.
	// Need to convert to `os.ErrNotExist` explicitly ourselves.
	mapNotFound := func(err error) error {
		if err != nil && strings.Contains(err.Error(), "not found") {
			err = os.ErrNotExist
		}
		return err
	}
	return &assetfs.AssetFS{
		Asset: func(path string) ([]byte, error) {
			ret, err := Asset(path)
			return ret, mapNotFound(err)
		},
		AssetDir: func(path string) ([]string, error) {
			ret, err := AssetDir(path)
			return ret, mapNotFound(err)
		},
		Prefix: prefix,
	}
}
