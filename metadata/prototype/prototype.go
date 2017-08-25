package prototype

import (
	"encoding/json"
)

type Snippet struct {
	Prefix string `json:"prefix"`

	// Description describes what the prototype does.
	Description string `json:"description"`

	// Body of the prototype. Follows the TextMate snippets syntax, with several
	// features disallowed.
	Body []string `json:"body"`
}

type Specification struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`

	// Unique identifier of the mixin library. The most reliable way to make a
	// name unique is to embed a domain you own into the name, as is commonly done
	// in the Java community.
	Name string `json:"name"`

	Template Snippet `json:"template"`
}

func Unmarshal(bytes []byte) (*Specification, error) {
	var p Specification
	err := json.Unmarshal(bytes, &p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
