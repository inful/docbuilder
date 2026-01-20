package frontmatter

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"
)

// SerializeYAML serializes a frontmatter map into YAML bytes (without delimiters).
//
// Determinism: keys are sorted (recursively for nested maps) to keep output stable.
// Newlines: the returned bytes use the newline style provided by Style (defaults to \n).
//
// If fields is empty, SerializeYAML returns an empty slice.
func SerializeYAML(fields map[string]any, style Style) ([]byte, error) {
	if len(fields) == 0 {
		return []byte{}, nil
	}

	nl := style.Newline
	if nl == "" {
		nl = "\n"
	}

	node, err := nodeFromStringMap(fields)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	out := buf.Bytes()
	if nl != "\n" {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte(nl))
	}
	return out, nil
}

func nodeFromStringMap(m map[string]any) (*yaml.Node, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
		valNode, err := nodeFromAny(m[k])
		if err != nil {
			return nil, err
		}
		n.Content = append(n.Content, keyNode, valNode)
	}
	return n, nil
}

func nodeFromAny(v any) (*yaml.Node, error) {
	switch vv := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: vv}, nil
	case bool:
		if vv {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}, nil
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}, nil
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(vv)}, nil
	case int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(vv, 10)}, nil
	case float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: fmt.Sprintf("%v", vv)}, nil
	case map[string]any:
		return nodeFromStringMap(vv)
	case map[any]any:
		converted := make(map[string]any, len(vv))
		for k, val := range vv {
			converted[fmt.Sprint(k)] = val
		}
		return nodeFromStringMap(converted)
	case []any:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range vv {
			node, err := nodeFromAny(item)
			if err != nil {
				return nil, err
			}
			seq.Content = append(seq.Content, node)
		}
		return seq, nil
	case []string:
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range vv {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item})
		}
		return seq, nil
	default:
		// Fall back to yaml's own encoding for uncommon scalar types.
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(v); err != nil {
			_ = enc.Close()
			return nil, err
		}
		_ = enc.Close()
		var node yaml.Node
		if err := yaml.Unmarshal(buf.Bytes(), &node); err != nil {
			return nil, err
		}
		// node is a DocumentNode; return its first child.
		if len(node.Content) == 0 {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
		}
		return node.Content[0], nil
	}
}
