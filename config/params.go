package config

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"sync"
)

// Params is the parameters of the vertex.
type Params struct {
	raw  json.RawMessage
	data any

	// path -> split parts
	pathCache sync.Map
}

// NewFromRaw creates from json raw message.
func NewFromRaw(raw json.RawMessage) (*Params, error) {
	if len(raw) == 0 {
		return &Params{data: nil, raw: raw}, nil
	}

	// Use json.Number to preserve precision for large numbers.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return &Params{data: v, raw: raw}, nil
}

func newFromAny(data any) *Params {
	if data == nil {
		return &Params{data: nil}
	}
	return &Params{data: data}
}

// getByPath gets the value by path.
// Support "a.b.c" and "tags.0".
func (p *Params) getByPath(path string) any {
	if p.data == nil || path == "" {
		return nil
	}

	var parts []string
	cached, ok := p.pathCache.Load(path)
	if ok {
		parts = cached.([]string)
	} else {
		parts = strings.Split(path, ".")
		p.pathCache.Store(path, parts)
	}

	var current any = p.data
	for _, part := range parts {
		if current == nil {
			return nil
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			current = v[idx]
		default:
			// path not matched
			return nil
		}
	}
	return current
}

// Exists checks if the path exists.
// Exists returns true only if the path exists and the value is not nil.
func (p *Params) Exists(path string) bool {
	return p.getByPath(path) != nil
}

// GetString returns the string value by path.
// If the path does not exist or the value is not a string, it will return the defaultValue.
func (p *Params) GetString(path string, defaultValue string) string {
	v := p.getByPath(path)
	if v == nil {
		return defaultValue
	}
	s, ok := v.(string)
	if !ok {
		return defaultValue
	}
	return s
}

// GetBool returns the bool value by path.
// If the path does not exist or the value is not a bool, it will return the defaultValue.
func (p *Params) GetBool(path string, defaultValue bool) bool {
	v := p.getByPath(path)
	if v == nil {
		return defaultValue
	}
	b, ok := v.(bool)
	if !ok {
		return defaultValue
	}
	return b
}

// GetInt64 returns the int64 value by path.
// If the path does not exist or the value is not a int64, it will return the defaultValue.
func (p *Params) GetInt64(path string, defaultValue int64) int64 {
	v := p.getByPath(path)
	if v == nil {
		return defaultValue
	}

	i, ok := toInt64(v)
	if !ok {
		return defaultValue
	}
	return i
}

// GetInt returns the int value by path.
// If the path does not exist or the value is not a int, it will return the defaultValue.
func (p *Params) GetInt(path string, defaultValue int) int {
	v := p.getByPath(path)
	if v == nil {
		return defaultValue
	}

	i, ok := toInt64(v)
	if !ok {
		return defaultValue
	}
	if i < int64(math.MinInt) || i > int64(math.MaxInt) {
		return defaultValue
	}
	return int(i)
}

// GetFloat64 returns the float64 value by path.
// If the path does not exist or the value is not a float64, it will return the defaultValue.
func (p *Params) GetFloat64(path string, defaultValue float64) float64 {
	v := p.getByPath(path)
	if v == nil {
		return defaultValue
	}
	f, ok := toFloat64(v)
	if !ok {
		return defaultValue
	}
	return f
}

// GetArray returns the array value by path.
// If the path does not exist or the value is not a array, it will return nil.
func (p *Params) GetArray(path string) []*Params {
	v := p.getByPath(path)
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	res := make([]*Params, 0, len(arr))
	for _, v := range arr {
		res = append(res, newFromAny(v))
	}
	return res
}

// GetSubParams returns the sub params by path.
// If the path does not exist or the value is not a map, it will return nil.
func (p *Params) GetSubParams(path string) *Params {
	v := p.getByPath(path)
	if v == nil {
		return nil
	}
	// check if v is a map or array
	if _, ok := v.(map[string]any); ok {
		return newFromAny(v)
	}
	if _, ok := v.([]any); ok {
		return newFromAny(v)
	}
	return nil
}

// GetRaw returns the raw json message.
// It is used for operator to parse params from raw json message.
func (p *Params) GetRaw() json.RawMessage {
	return p.raw
}

// GetArrayInt64 returns the array of int64 value by path.
// If the path does not exist or the value is not a array, it will return nil.
func (p *Params) GetArrayInt64(path string) []int64 {
	v := p.getByPath(path)
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	res := make([]int64, 0, len(arr))
	for _, v := range arr {
		i, ok := toInt64(v)
		if !ok {
			return nil
		}
		res = append(res, i)
	}
	return res
}

// GetArrayFloat64 returns the array of float64 value by path.
// If the path does not exist or the value is not a array, it will return nil.
func (p *Params) GetArrayFloat64(path string) []float64 {
	v := p.getByPath(path)
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	res := make([]float64, 0, len(arr))
	for _, v := range arr {
		f, ok := toFloat64(v)
		if !ok {
			return nil
		}
		res = append(res, f)
	}
	return res
}

// GetArrayString returns the array of string value by path.
// If the path does not exist or the value is not a array, it will return nil.
func (p *Params) GetArrayString(path string) []string {
	v := p.getByPath(path)
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	res := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil
		}
		res = append(res, s)
	}
	return res
}

func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case json.Number:
		i, err := val.Int64()
		return i, err == nil
	case int64:
		return val, true
	case int32:
		return int64(val), true
	case int16:
		return int64(val), true
	case int8:
		return int64(val), true
	case int:
		return int64(val), true
	case uint64:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint8:
		return int64(val), true
	case uint:
		return int64(val), true
	}
	return 0, false
}

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case int:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint:
		return float64(val), true
	}
	return 0, false
}
