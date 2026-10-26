// Package yamlcodec is the single home of the gopkg.in/yaml.v3 dependency.
package yamlcodec

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

func Marshal(v any) ([]byte, error) {
	return yaml.Marshal(v)
}

func Unmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

func UnmarshalStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}
