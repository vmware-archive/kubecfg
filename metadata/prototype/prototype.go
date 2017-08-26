package prototype

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Specification struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`

	// Unique identifier of the mixin library. The most reliable way to make a
	// name unique is to embed a domain you own into the name, as is commonly done
	// in the Java community.
	Name     string  `json:"name"`
	Params   Params  `json:"params"`
	Template Snippet `json:"template"`
}

func (s *Specification) RequiredParams() Params {
	reqd := Params{}
	for _, p := range s.Params {
		if p.Default == nil {
			reqd = append(reqd, p)
		}
	}

	return reqd
}

func (s *Specification) OptionalParams() Params {
	opt := Params{}
	for _, p := range s.Params {
		if p.Default != nil {
			opt = append(opt, p)
		}
	}

	return opt
}

type Snippet struct {
	Prefix string `json:"prefix"`

	// Description describes what the prototype does.
	Description string `json:"description"`

	// Body of the prototype. Follows the TextMate snippets syntax, with several
	// features disallowed.
	Body []string `json:"body"`
}

type Param struct {
	Name        string  `json:"name"`
	Alias       *string `json:"alias"`
	Description string  `json:"description"`
	Default     *string `json:"default"`
}

func RequiredParam(name, alias, description string) *Param {
	return &Param{
		Name:        name,
		Alias:       &alias,
		Description: description,
		Default:     nil,
	}
}

func OptionalParam(name, alias, description, defaultVal string) *Param {
	return &Param{
		Name:        name,
		Alias:       &alias,
		Description: description,
		Default:     &defaultVal,
	}
}

type Params []*Param

func (ps Params) PrettyString(prefix string) string {
	if len(ps) == 0 {
		return "  [none]"
	}

	flags := []string{}
	for _, p := range ps {
		alias := p.Name
		if p.Alias != nil {
			alias = *p.Alias
		}
		flags = append(flags, fmt.Sprintf("--%s=<%s>", p.Name, alias))
	}

	max := 0
	for _, flag := range flags {
		if flagLen := len(flag); max < flagLen {
			max = flagLen
		}
	}

	prettyFlags := []string{}
	for i := range flags {
		p := ps[i]
		flag := flags[i]

		defaultVal := ""
		if p.Default != nil {
			defaultVal = fmt.Sprintf(" [default: %s]", *p.Default)
		}

		// NOTE: If we don't add 1 here, the longest line will look like:
		// `--flag=<flag>Description is here.`
		space := strings.Repeat(" ", max-len(flag)+1)
		pretty := fmt.Sprintf(prefix + flag + space + p.Description + defaultVal)
		prettyFlags = append(prettyFlags, pretty)
	}

	return strings.Join(prettyFlags, "\n")
}

func Unmarshal(bytes []byte) (*Specification, error) {
	var p Specification
	err := json.Unmarshal(bytes, &p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}
