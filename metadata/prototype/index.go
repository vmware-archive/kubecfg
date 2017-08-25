package prototype

import (
	"fmt"
	"strings"
)

type SearchOptions int

const (
	Prefix SearchOptions = iota
	Suffix
	Substring
)

const (
	delimiter = "\x00"
)

type Index interface {
	SearchNames(query string, opts SearchOptions) ([]*Specification, error)
}

type index struct {
	prototypes map[string]*Specification
}

func NewIndex(prototypes []*Specification) Index {
	idx := map[string]*Specification{}

	for _, p := range prototypes {
		idx[p.Name] = p
	}

	return &index{
		prototypes: idx,
	}
}

func (idx *index) SearchNames(query string, opts SearchOptions) ([]*Specification, error) {
	// TODO(hausdorff): This is the world's worst search algorithm. Improve it at
	// some point.

	prototypes := []*Specification{}

	for name, prototype := range idx.prototypes {
		isSearchResult := false
		switch opts {
		case Prefix:
			isSearchResult = strings.HasPrefix(name, query)
		case Suffix:
			isSearchResult = strings.HasSuffix(name, query)
		case Substring:
			isSearchResult = strings.Contains(name, query)
		default:
			return nil, fmt.Errorf("Unrecognized search option '%d'", opts)
		}

		if isSearchResult {
			prototypes = append(prototypes, prototype)
		}
	}

	return prototypes, nil
}
