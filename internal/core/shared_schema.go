package core

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
)

//go:embed shared_configuration_schema.json
var sharedConfigurationSchema []byte

type sharedField struct {
	Keys     []string        `json:"keys"`
	Type     string          `json:"type"`
	Ref      string          `json:"ref"`
	Required bool            `json:"required"`
	Nullable bool            `json:"nullable"`
	Default  json.RawMessage `json:"default"`
	Values   []any           `json:"values"`
	Items    *sharedField    `json:"items"`
	MapOf    *sharedField    `json:"mapOf"`
	Minimum  *float64        `json:"minimum"`
	Maximum  *float64        `json:"maximum"`
	MinItems *int            `json:"minItems"`
	MaxItems *int            `json:"maxItems"`
	Pattern  string          `json:"pattern"`
}

var sharedContract = func() struct {
	Definitions map[string]map[string]sharedField `json:"definitions"`
} {
	var contract struct {
		Definitions map[string]map[string]sharedField `json:"definitions"`
	}
	if err := json.Unmarshal(sharedConfigurationSchema, &contract); err != nil {
		panic(err)
	}
	return contract
}()

// Errors deliberately include only a schema path, never configuration values.
var ErrSharedProtocol = errors.New("共享配置协议不兼容")

func sharedSchemaError(path string) error {
	return fmt.Errorf("%w：字段 %s 不支持或类型无效；请升级所有节点后重试，未覆盖数据", ErrSharedProtocol, path)
}
func canonicalConfigurationJSON(data []byte) ([]byte, error) {
	return canonicalSharedJSON(data, "configuration")
}
func canonicalSharedJSON(data []byte, rootType string) ([]byte, error) {
	var root any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, sharedSchemaError(rootType)
	}
	if object, ok := root.(map[string]any); ok {
		migrateSharedWire(object)
	}
	value, err := canonicalSharedValue(root, sharedField{Type: "object", Ref: rootType}, rootType)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
func canonicalSharedValue(value any, rule sharedField, path string) (any, error) {
	if value == nil && rule.Nullable {
		return nil, nil
	}
	switch rule.Type {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return nil, sharedSchemaError(path)
		}
		out := map[string]any{}
		if rule.MapOf != nil {
			for key, item := range object {
				if len(rule.Keys) > 0 {
					known := false
					for _, allowed := range rule.Keys {
						if key == allowed {
							known = true
							break
						}
					}
					if !known {
						return nil, sharedSchemaError(path + ".[unknown]")
					}
				}
				normalized, err := canonicalSharedValue(item, *rule.MapOf, path+".*")
				if err != nil {
					return nil, err
				}
				out[key] = normalized
			}
			return out, nil
		}
		fields, ok := sharedContract.Definitions[rule.Ref]
		if !ok {
			return nil, sharedSchemaError(path)
		}
		for key := range object {
			if _, ok := fields[key]; !ok {
				return nil, sharedSchemaError(path + ".[unknown]")
			}
		}
		marker := rule.Ref == "proxy" && object["kind"] == "shared-reference"
		for key, field := range fields {
			if marker && key != "id" && key != "kind" {
				if _, ok := object[key]; ok {
					return nil, sharedSchemaError(path + "." + key)
				}
				continue
			}
			item, exists := object[key]
			if !exists || item == nil && !field.Nullable {
				if field.Required {
					return nil, sharedSchemaError(path + "." + key)
				}
				if field.Default == nil {
					continue
				}
				dec := json.NewDecoder(bytes.NewReader(field.Default))
				dec.UseNumber()
				if err := dec.Decode(&item); err != nil {
					return nil, err
				}
			}
			if !exists && field.Default == nil {
				continue
			}
			normalized, err := canonicalSharedValue(item, field, path+"."+key)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case "array":
		array, ok := value.([]any)
		if !ok {
			return nil, sharedSchemaError(path)
		}
		if rule.MinItems != nil && len(array) < *rule.MinItems || rule.MaxItems != nil && len(array) > *rule.MaxItems {
			return nil, sharedSchemaError(path)
		}
		out := make([]any, 0, len(array))
		for _, item := range array {
			n, err := canonicalSharedValue(item, *rule.Items, path+"[]")
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
		return out, nil
	case "string":
		text, ok := value.(string)
		if !ok {
			return nil, sharedSchemaError(path)
		}
		if rule.Pattern != "" {
			ok, err := regexp.MatchString(rule.Pattern, text)
			if err != nil || !ok {
				return nil, sharedSchemaError(path)
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return nil, sharedSchemaError(path)
		}
	case "integer", "number":
		number, ok := value.(json.Number)
		if !ok {
			return nil, sharedSchemaError(path)
		}
		n, err := number.Float64()
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, sharedSchemaError(path)
		}
		if rule.Type == "integer" {
			if math.Trunc(n) != n || math.Abs(n) > 9007199254740991 {
				return nil, sharedSchemaError(path)
			}
			value = json.Number(strconv.FormatFloat(n, 'f', 0, 64))
		}
		if rule.Minimum != nil && n < *rule.Minimum || rule.Maximum != nil && n > *rule.Maximum {
			return nil, sharedSchemaError(path)
		}
	default:
		return nil, sharedSchemaError(path)
	}
	if len(rule.Values) > 0 {
		encoded, _ := json.Marshal(value)
		found := false
		for _, allowed := range rule.Values {
			candidate, _ := json.Marshal(allowed)
			if bytes.Equal(encoded, candidate) {
				found = true
				break
			}
		}
		if !found {
			return nil, sharedSchemaError(path)
		}
	}
	return value, nil
}
func migrateSharedWire(root map[string]any) {
	if prefs, ok := root["preferences"].(map[string]any); ok {
		if scope, ok := prefs["quickSendScope"].(string); ok {
			for n, name := range sharedSendScopes {
				if scope == name {
					prefs["quickSendScope"] = json.Number(strconv.Itoa(n))
					break
				}
			}
		}
	}
	if profiles, ok := root["profiles"].([]any); ok {
		for _, item := range profiles {
			p, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if p["titleMode"] == nil {
				if old, ok := p["activeHostProbe"].(bool); ok && old {
					p["titleMode"] = "activeProbe"
				}
			}
			if _, ok := p["activeHostProbe"].(bool); ok {
				delete(p, "activeHostProbe")
			}
			if p["initialDirectory"] == nil && p["kind"] == "ftp" {
				p["initialDirectory"] = "/"
			}
			if p["proxy"] == nil {
				if id, ok := p["id"].(string); ok {
					p["proxy"] = map[string]any{"id": absentProxyID(id), "kind": "none"}
				}
			}
		}
	}
}
