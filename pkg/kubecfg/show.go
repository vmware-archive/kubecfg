package kubecfg

import (
	"encoding/json"
	"fmt"
	"io"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ShowCmd struct {
	Format string

	Objs []*unstructured.Unstructured
}

func (c ShowCmd) Run(out io.Writer) error {
	switch c.Format {
	case "yaml":
		for _, obj := range c.Objs {
			fmt.Fprintln(out, "---")
			// Urgh.  Go via json because we need
			// to trigger the custom scheme
			// encoding.
			buf, err := json.Marshal(obj)
			if err != nil {
				return err
			}
			o := map[string]interface{}{}
			if err := json.Unmarshal(buf, &o); err != nil {
				return err
			}
			buf, err = yaml.Marshal(o)
			if err != nil {
				return err
			}
			out.Write(buf)
		}
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		for _, obj := range c.Objs {
			// TODO: this is not valid framing for JSON
			if len(c.Objs) > 1 {
				fmt.Fprintln(out, "---")
			}
			if err := enc.Encode(obj); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("Unknown --format: %s", c.Format)
	}

	return nil
}
