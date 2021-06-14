package utils

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	jsonnet "github.com/google/go-jsonnet"
	log "github.com/sirupsen/logrus"
)

var extVarKindRE = regexp.MustCompile("^<(?:extvar|top-level-arg):.+>$")

/*
MakeUniversalImporter creates an importer that handles resolving imports from the filesystem and HTTP/S.

In addition to the standard importer, supports:
  - URLs in import statements
  - URLs in library search paths
  - Local caching for files retrieved from remote locations

A real-world example:
  - You have https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master in your search URLs.
  - You evaluate a local file which calls `import "ksonnet.beta.2/k.libsonnet"`.
  - If the `ksonnet.beta.2/k.libsonnet`` is not located in the current working directory, an attempt
    will be made to follow the search path, i.e. to download
    https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/ksonnet.beta.2/k.libsonnet.
  - Since the downloaded `k.libsonnet`` file turn in contains `import "k8s.libsonnet"`, the import
    will be resolved as https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/ksonnet.beta.2/k8s.libsonnet
	and downloaded from that location.
*/
func MakeUniversalImporter(searchURLs []*url.URL, cacheDir string) jsonnet.Importer {
	return &universalImporter{
		BaseSearchURLs: searchURLs,
		HTTPCache:      NewHTTPCache(cacheDir),
		cache:          map[string]jsonnet.Contents{},
	}
}

type universalImporter struct {
	BaseSearchURLs []*url.URL
	HTTPCache      *httpCache
	cache          map[string]jsonnet.Contents
}

func (importer *universalImporter) Import(importedFrom, importedPath string) (jsonnet.Contents, string, error) {
	log.Debugf("Importing %q from %q", importedPath, importedFrom)

	candidateURLs, err := importer.expandImportToCandidateURLs(importedFrom, importedPath)
	if err != nil {
		return jsonnet.Contents{}, "", fmt.Errorf("Could not get candidate URLs for when importing %s (imported from %s): %v", importedPath, importedFrom, err)
	}

	var tried []string
	for _, u := range candidateURLs {
		foundAt := u.String()
		if c, ok := importer.cache[foundAt]; ok {
			return c, foundAt, nil
		}

		tried = append(tried, foundAt)
		importedData, err := importer.HTTPCache.Get(foundAt)
		if err == nil {
			importer.cache[foundAt] = importedData
			return importedData, foundAt, nil
		} else if err != errNotFound {
			return jsonnet.Contents{}, "", err
		}
	}

	return jsonnet.Contents{}, "", fmt.Errorf("Couldn't open import %q, no match locally or in library search paths. Tried: %s",
		importedPath,
		strings.Join(tried, ";"),
	)
}

func (importer *universalImporter) expandImportToCandidateURLs(importedFrom, importedPath string) ([]*url.URL, error) {
	importedPathURL, err := url.Parse(importedPath)
	if err != nil {
		return nil, fmt.Errorf("Import path %q is not valid", importedPath)
	}
	if importedPathURL.IsAbs() {
		return []*url.URL{importedPathURL}, nil
	}

	importDirURL, err := url.Parse(importedFrom)
	if err != nil {
		return nil, fmt.Errorf("Invalid import dir %q: %v", importedFrom, err)
	}

	candidateURLs := make([]*url.URL, 1, len(importer.BaseSearchURLs)+1)
	candidateURLs[0] = importDirURL.ResolveReference(importedPathURL)

	for _, u := range importer.BaseSearchURLs {
		candidateURLs = append(candidateURLs, u.ResolveReference(importedPathURL))
	}

	return candidateURLs, nil
}
