package utils

import (
	"fmt"
	jsonnet "github.com/google/go-jsonnet"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

/*
MakeUniversalImporter creates an importer that handles resolving imports from the filesystem and http/s.

In addition to the standard importer, supports:
  - URLs in import statements
  - URLs in library search paths

A real-world example:
  - you have https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master in your search URLs
  - you evaluate a local file which calls `import "ksonnet.beta.2/k.libsonnet"`
  - if the `ksonnet.beta.2/k.libsonnet`` is not located in the current workdir, an attempt
    will be made to follow the search path, i.e. to download
    https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/ksonnet.beta.2/k.libsonnet
  - since the downloaded `k.libsonnet`` file turn in contains `import "k8s.libsonnet"`, the import
    will be resolved as https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/ksonnet.beta.2/k8s.libsonnet
	and downloaded from that location
*/
func MakeUniversalImporter(fsWorkdir string, searchPathsOrURLs []string) (jsonnet.Importer, error) {
	var urls []url.URL
	if !filepath.IsAbs(fsWorkdir) {
		return nil, fmt.Errorf("Given filesystem workdir %s is not an absolute path", fsWorkdir)
	}
	workDirURL := url.URL{Scheme: "file", Path: fsWorkdir}

	for _, p := range searchPathsOrURLs {
		u, err := url.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("Could not parse search path/url %s", p)
		}
		if u.IsAbs() {
			if strings.EqualFold(u.Scheme, "file") && u.Host != "" {
				return nil, fmt.Errorf("Search path given as an URL %s is invalid: Ensure it begins with file:///", p)
			}
			urls = append(urls, *u)
		} else {
			if filepath.IsAbs(p) {
				urls = append(urls, url.URL{Scheme: "file", Path: p})
			} else {
				urls = append(urls, joinURL(workDirURL, p))
			}
		}
	}

	t := &http.Transport{}
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))

	return &universalImporter{
			WorkDirURL:     workDirURL,
			BaseSearchURLs: urls,
			HTTPClient:     &http.Client{Transport: t},
		},
		nil
}

type universalImporter struct {
	WorkDirURL     url.URL
	BaseSearchURLs []url.URL
	HTTPClient     *http.Client
}

func (importer *universalImporter) Import(dir, importedPath string) (*jsonnet.ImportedData, error) {
	candidateURLs, err := importer.expandImportToCandidateURLs(dir, importedPath)
	if err != nil {
		return nil, fmt.Errorf("Could not get candidate URLs for when importing %s (import dir is %s)", importedPath, dir)
	}

	var tried []string
	for _, u := range candidateURLs {
		tried = append(tried, u.String())
		importedData := importer.tryImport(u)
		if importedData != nil {
			return importedData, nil
		}
	}

	return nil, fmt.Errorf("Could't open import '%s', no match locally or in library search paths. Tried: %s",
		importedPath,
		strings.Join(tried[:], ";"),
	)
}

func (importer *universalImporter) tryImport(url url.URL) *jsonnet.ImportedData {
	res, err := importer.HTTPClient.Get(url.String())
	if err == nil {
		defer res.Body.Close()
		if res.StatusCode == http.StatusOK {
			bodyBytes, err := ioutil.ReadAll(res.Body)
			if err == nil {
				return &jsonnet.ImportedData{
					FoundHere: url.String(),
					Content:   string(bodyBytes),
				}
			}
		}
	}
	return nil
}

func (importer *universalImporter) expandImportToCandidateURLs(dir, importedPath string) ([]url.URL, error) {
	importedPathURL, err := url.Parse(importedPath)
	if err != nil {
		return nil, fmt.Errorf("Import path '%s' is not valid", importedPath)
	}
	if importedPathURL.IsAbs() {
		return []url.URL{*importedPathURL}, nil
	} else if filepath.IsAbs(importedPath) {
		return []url.URL{url.URL{Scheme: "file", Path: importedPath}}, nil
	}

	importDirURL, err := url.Parse(dir)
	if err != nil {
		return nil, fmt.Errorf("Invalid import dir '%s'", dir)
	}

	var candidateURLs []url.URL

	if !importDirURL.IsAbs() {
		candidateURLs = append(candidateURLs, joinURL(importer.WorkDirURL, dir, importedPath))
	} else {
		candidateURLs = append(candidateURLs, joinURL(*importDirURL, importedPath))
	}

	for _, baseSearchURL := range importer.BaseSearchURLs {
		candidateURLs = append(candidateURLs, joinURL(baseSearchURL, importedPath))
	}
	return candidateURLs, nil
}

func joinURL(url url.URL, relativePaths ...string) url.URL {
	paths := make([]string, len(relativePaths)+1)
	paths[0] = filepath.ToSlash(url.Path)
	for i, p := range relativePaths {
		paths[i+1] = filepath.ToSlash(p)
	}

	newURL := url
	newURL.Path = path.Join(paths...)
	return newURL
}

/*
	Converts a dir value to URL, while fixing up the dir if necessary.
	The `dir` value that jsonnet calls importer's Import function with is transformed in process - the path
	is cleaned up, which means that consecutive slashes are compacted into one. That poses a problem where
	we, for example, return a resouce path as https://domain.com/path/abc/file.libsonnet, but when resolving
	dependencies the dir comes back as https:/domain.com/path/abc.
*/
func dirURL(dir string) (*url.URL, error) {
	fixupRe := regexp.MustCompile("(?i)(http|https|file):\\/+(.*)")
	fixupMatch := fixupRe.FindStringSubmatch(dir)
	if fixupMatch != nil {
		var slashes = "//"
		if strings.EqualFold(fixupMatch[1], "file") {
			slashes = "///"
		}
		dir = fmt.Sprintf("%s:%s%s", fixupMatch[1], slashes, fixupMatch[2])
	}

	return url.Parse(dir)
}
