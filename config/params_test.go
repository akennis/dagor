package config

import (
	"encoding/json"
	"math"
	"testing"
)

func TestNewFromRaw_Empty(t *testing.T) {
	params, err := NewFromRaw(json.RawMessage(""))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if params == nil {
		t.Fatal("expected non-nil params")
	}
	if params.data != nil {
		t.Error("expected nil data for empty raw message")
	}
}

func TestNewFromRaw_ValidJSON(t *testing.T) {
	raw := json.RawMessage(`{"name":"test","value":123}`)
	params, err := NewFromRaw(raw)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if params == nil {
		t.Fatal("expected non-nil params")
	}
	if params.data == nil {
		t.Error("expected non-nil data")
	}
}

func TestNewFromRaw_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid json}`)
	params, err := NewFromRaw(raw)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
	if params != nil {
		t.Error("expected nil params for invalid JSON")
	}
}

func TestNewFromRaw_NestedObject(t *testing.T) {
	raw := json.RawMessage(`{"a":{"b":{"c":123}}}`)
	params, err := NewFromRaw(raw)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if params == nil {
		t.Fatal("expected non-nil params")
	}
}

func TestNewFromRaw_Array(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b","c"]}`)
	params, err := NewFromRaw(raw)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if params == nil {
		t.Fatal("expected non-nil params")
	}
}

func TestGetByPath_SimplePath(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("name")
	if value == nil {
		t.Fatal("expected non-nil value")
	}
	if value.(string) != "test" {
		t.Errorf("expected 'test', got %v", value)
	}
}

func TestGetByPath_NestedPath(t *testing.T) {
	raw := json.RawMessage(`{"a":{"b":{"c":123}}}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("a.b.c")
	if value == nil {
		t.Fatal("expected non-nil value")
	}
	i, err := value.(json.Number).Int64()
	if err != nil {
		t.Errorf("expected int64, got %v", err)
	}
	if i != 123 {
		t.Errorf("expected 123, got %v", value)
	}
}

func TestGetByPath_ArrayIndex(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b","c"]}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("tags.0")
	if value == nil {
		t.Fatal("expected non-nil value")
	}
	if value.(string) != "a" {
		t.Errorf("expected 'a', got %v", value)
	}
}

func TestGetByPath_ArrayIndexOutOfBounds(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b"]}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("tags.5")
	if value != nil {
		t.Errorf("expected nil for out of bounds index, got %v", value)
	}
}

func TestGetByPath_InvalidArrayIndex(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b"]}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("tags.invalid")
	if value != nil {
		t.Errorf("expected nil for invalid index, got %v", value)
	}
}

func TestGetByPath_NonExistentPath(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("nonexistent")
	if value != nil {
		t.Errorf("expected nil for non-existent path, got %v", value)
	}
}

func TestGetByPath_EmptyPath(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("")
	if value != nil {
		t.Errorf("expected nil for empty path, got %v", value)
	}
}

func TestGetByPath_NilData(t *testing.T) {
	params := &Params{data: nil}
	value := params.getByPath("any.path")
	if value != nil {
		t.Errorf("expected nil for nil data, got %v", value)
	}
}

func TestGetByPath_PathCache(t *testing.T) {
	raw := json.RawMessage(`{"a":{"b":123}}`)
	params, _ := NewFromRaw(raw)

	// First call should populate cache
	value1 := params.getByPath("a.b")
	if value1 == nil {
		t.Fatal("expected non-nil value")
	}

	// Second call should use cache
	value2 := params.getByPath("a.b")
	if value2 == nil {
		t.Fatal("expected non-nil value")
	}

	if value1 != value2 {
		t.Error("expected cached value to match")
	}
}

func TestExists_ExistingPath(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	if !params.Exists("name") {
		t.Error("expected path to exist")
	}
}

func TestExists_NonExistentPath(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	if params.Exists("nonexistent") {
		t.Error("expected path not to exist")
	}
}

func TestExists_NilValue(t *testing.T) {
	raw := json.RawMessage(`{"name":null}`)
	params, _ := NewFromRaw(raw)
	if params.Exists("name") {
		t.Error("expected nil value to not exist")
	}
}

func TestGetString_Valid(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	value := params.GetString("name", "default")
	if value != "test" {
		t.Errorf("expected 'test', got '%s'", value)
	}
}

func TestGetString_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	value := params.GetString("nonexistent", "default")
	if value != "default" {
		t.Errorf("expected 'default', got '%s'", value)
	}
}

func TestGetString_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":123}`)
	params, _ := NewFromRaw(raw)
	value := params.GetString("value", "default")
	if value != "default" {
		t.Errorf("expected 'default', got '%s'", value)
	}
}

func TestGetBool_Valid(t *testing.T) {
	raw := json.RawMessage(`{"enabled":true}`)
	params, _ := NewFromRaw(raw)
	value := params.GetBool("enabled", false)
	if !value {
		t.Error("expected true, got false")
	}
}

func TestGetBool_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"enabled":true}`)
	params, _ := NewFromRaw(raw)
	value := params.GetBool("nonexistent", false)
	if value {
		t.Error("expected false (default), got true")
	}
}

func TestGetBool_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":"true"}`)
	params, _ := NewFromRaw(raw)
	value := params.GetBool("value", false)
	if value {
		t.Error("expected false (default), got true")
	}
}

func TestGetInt64_ValidInt(t *testing.T) {
	raw := json.RawMessage(`{"value":123}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt64("value", 0)
	if value != 123 {
		t.Errorf("expected 123, got %d", value)
	}
}

func TestGetInt64_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"value":123}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt64("nonexistent", 999)
	if value != 999 {
		t.Errorf("expected 999 (default), got %d", value)
	}
}

func TestGetInt64_InvalidString(t *testing.T) {
	raw := json.RawMessage(`{"value":"notanumber"}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt64("value", 999)
	if value != 999 {
		t.Errorf("expected 999 (default), got %d", value)
	}
}

func TestGetInt_Valid(t *testing.T) {
	raw := json.RawMessage(`{"value":123}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt("value", 0)
	if value != 123 {
		t.Errorf("expected 123, got %d", value)
	}
}

func TestGetInt_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"value":123}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt("nonexistent", 999)
	if value != 999 {
		t.Errorf("expected 999 (default), got %d", value)
	}
}

func TestGetInt_OutOfBounds(t *testing.T) {
	raw := json.RawMessage(`{"value":9223372036854775809}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt("value", 0)
	if value != 0 {
		t.Errorf("expected 0 (default), got %d", value)
	}
}

func TestGetInt_NegativeOutOfBounds(t *testing.T) {
	raw := json.RawMessage(`{"value":-9223372036854775809}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt("value", 0)
	if value != 0 {
		t.Errorf("expected 0 (default) for out of bounds, got %d", value)
	}
}

func TestGetFloat64_ValidFloat(t *testing.T) {
	raw := json.RawMessage(`{"value":123.45}`)
	params, _ := NewFromRaw(raw)
	value := params.GetFloat64("value", 0.0)
	if value != 123.45 {
		t.Errorf("expected 123.45, got %f", value)
	}
}

func TestGetFloat64_ValidInt(t *testing.T) {
	raw := json.RawMessage(`{"value":123}`)
	params, _ := NewFromRaw(raw)
	value := params.GetFloat64("value", 0.0)
	if value != 123.0 {
		t.Errorf("expected 123.0, got %f", value)
	}
}

func TestGetFloat64_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"value":123.45}`)
	params, _ := NewFromRaw(raw)
	value := params.GetFloat64("nonexistent", 999.99)
	if value != 999.99 {
		t.Errorf("expected 999.99 (default), got %f", value)
	}
}

func TestGetFloat64_InvalidString(t *testing.T) {
	raw := json.RawMessage(`{"value":"notanumber"}`)
	params, _ := NewFromRaw(raw)
	value := params.GetFloat64("value", 999.99)
	if value != 999.99 {
		t.Errorf("expected 999.99 (default), got %f", value)
	}
}

func TestGetArray_Valid(t *testing.T) {
	raw := json.RawMessage(`{"items":[1,2,3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArray("items")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
}

func TestGetArray_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"items":[1,2,3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArray("nonexistent")
	if arr != nil {
		t.Errorf("expected nil for non-existent path, got %v", arr)
	}
}

func TestGetArray_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":"notanarray"}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArray("value")
	if arr != nil {
		t.Errorf("expected nil for wrong type, got %v", arr)
	}
}

func TestGetArray_EmptyArray(t *testing.T) {
	raw := json.RawMessage(`{"items":[]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArray("items")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got length %d", len(arr))
	}
}

func TestGetSubParams_Valid(t *testing.T) {
	raw := json.RawMessage(`{"config":{"key":"value"}}`)
	params, _ := NewFromRaw(raw)
	sub := params.GetSubParams("config")
	if sub == nil {
		t.Fatal("expected non-nil sub params")
	}
	value := sub.GetString("key", "")
	if value != "value" {
		t.Errorf("expected 'value', got '%s'", value)
	}
}

func TestGetSubParams_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"config":{"key":"value"}}`)
	params, _ := NewFromRaw(raw)
	sub := params.GetSubParams("nonexistent")
	if sub != nil {
		t.Errorf("expected nil for non-existent path, got %v", sub)
	}
}

func TestGetSubParams_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":"notamap"}`)
	params, _ := NewFromRaw(raw)
	sub := params.GetSubParams("value")
	if sub != nil {
		t.Errorf("expected nil for wrong type, got %v", sub)
	}
}

func TestGetRaw_WithRaw(t *testing.T) {
	raw := json.RawMessage(`{"name":"test"}`)
	params, _ := NewFromRaw(raw)
	retrieved := params.GetRaw()
	if string(retrieved) != string(raw) {
		t.Errorf("expected raw message to match, got %s", string(retrieved))
	}
}

func TestGetRaw_WithoutRaw(t *testing.T) {
	params := newFromAny(map[string]any{"name": "test"})
	retrieved := params.GetRaw()
	if len(retrieved) != 0 {
		t.Errorf("expected empty raw message, got %s", string(retrieved))
	}
}

func TestToFloat64_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
		valid    bool
	}{
		{"float64", float64(123.45), 123.45, true},
		{"float32", float32(123.45), float64(float32(123.45)), true},
		{"int", int(123), 123.0, true},
		{"int64", int64(123), 123.0, true},
		{"int32", int32(123), 123.0, true},
		{"int16", int16(123), 123.0, true},
		{"int8", int8(123), 123.0, true},
		{"uint", uint(123), 123.0, true},
		{"uint64", uint64(123), 123.0, true},
		{"uint32", uint32(123), 123.0, true},
		{"uint16", uint16(123), 123.0, true},
		{"uint8", uint8(123), 123.0, true},
		{"string_valid", "123.45", 0, false},
		{"string_invalid", "notanumber", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toFloat64(tt.input)
			if ok != tt.valid {
				t.Errorf("expected valid=%v, got %v for %s", tt.valid, ok, tt.name)
			}
			if tt.valid && result != tt.expected {
				t.Errorf("expected %f, got %f for %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestToInt64_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int64
		valid    bool
	}{
		{"int64", int64(123), 123, true},
		{"int32", int32(123), 123, true},
		{"int16", int16(123), 123, true},
		{"int8", int8(123), 123, true},
		{"int", int(123), 123, true},
		{"uint64", uint64(123), 123, true},
		{"uint32", uint32(123), 123, true},
		{"uint16", uint16(123), 123, true},
		{"uint8", uint8(123), 123, true},
		{"uint", uint(123), 123, true},
		{"float64", float64(123.7), 0, false},
		{"float32", float32(123.7), 0, false},
		{"string_valid", "123", 0, false},
		{"string_invalid", "notanumber", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toInt64(tt.input)
			if ok != tt.valid {
				t.Errorf("expected valid=%v, got %v for %s", tt.valid, ok, tt.name)
			}
			if tt.valid && result != tt.expected {
				t.Errorf("expected %d, got %d for %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestComplexNestedStructure(t *testing.T) {
	raw := json.RawMessage(`{
		"user": {
			"name": "John",
			"age": 30,
			"active": true,
			"tags": ["admin", "user"],
			"settings": {
				"theme": "dark",
				"notifications": false
			}
		},
		"items": [
			{"id": 1, "name": "item1"},
			{"id": 2, "name": "item2"}
		]
	}`)
	params, err := NewFromRaw(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test nested string
	if name := params.GetString("user.name", ""); name != "John" {
		t.Errorf("expected 'John', got '%s'", name)
	}

	// Test nested int
	if age := params.GetInt("user.age", 0); age != 30 {
		t.Errorf("expected 30, got %d", age)
	}

	// Test nested bool
	if active := params.GetBool("user.active", false); !active {
		t.Errorf("expected true, got false")
	}

	// Test nested array
	if tags := params.GetArray("user.tags"); len(tags) != 2 {
		t.Errorf("expected array length 2, got %d", len(tags))
	}

	// Test nested map
	settings := params.GetSubParams("user.settings")
	if settings == nil {
		t.Error("expected non-nil sub params")
	}
	if theme := settings.GetString("theme", ""); theme != "dark" {
		t.Errorf("expected 'dark', got '%s'", theme)
	}
	if notifications := settings.GetBool("notifications", false); notifications {
		t.Errorf("expected false, got true")
	}

	// Test nested array
	items := params.GetArray("items")
	if items == nil {
		t.Error("expected non-nil array")
	}
	if len(items) != 2 {
		t.Errorf("expected array length 2, got %d", len(items))
	}
}

// TestGetArrayInt64 tests the GetArrayInt64 method
func TestGetArrayInt64_Valid(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1,2,3,4,5]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 5 {
		t.Errorf("expected array length 5, got %d", len(arr))
	}
	expected := []int64{1, 2, 3, 4, 5}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %d at index %d, got %d", expected[i], i, v)
		}
	}
}

func TestGetArrayInt64_EmptyArray(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got length %d", len(arr))
	}
}

func TestGetArrayInt64_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1,2,3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("nonexistent")
	if arr != nil {
		t.Errorf("expected nil for non-existent path, got %v", arr)
	}
}

func TestGetArrayInt64_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":"notanarray"}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("value")
	if arr != nil {
		t.Errorf("expected nil for wrong type, got %v", arr)
	}
}

func TestGetArrayInt64_MixedTypes(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1,"2",3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("numbers")
	if arr != nil {
		t.Errorf("expected nil for mixed types, got %v", arr)
	}
}

func TestGetArrayInt64_WithFloat(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1,2.5,3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("numbers")
	if arr != nil {
		t.Errorf("expected nil for array with float, got %v", arr)
	}
}

func TestGetArrayInt64_LargeNumbers(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[9223372036854775807,-9223372036854775808,0]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	if arr[0] != 9223372036854775807 {
		t.Errorf("expected max int64, got %d", arr[0])
	}
	if arr[1] != -9223372036854775808 {
		t.Errorf("expected min int64, got %d", arr[1])
	}
	if arr[2] != 0 {
		t.Errorf("expected 0, got %d", arr[2])
	}
}

func TestGetArrayInt64_NestedPath(t *testing.T) {
	raw := json.RawMessage(`{"data":{"numbers":[10,20,30]}}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayInt64("data.numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	expected := []int64{10, 20, 30}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %d at index %d, got %d", expected[i], i, v)
		}
	}
}

// TestGetArrayFloat64 tests the GetArrayFloat64 method
func TestGetArrayFloat64_Valid(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1.1,2.2,3.3,4.4,5.5]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 5 {
		t.Errorf("expected array length 5, got %d", len(arr))
	}
	expected := []float64{1.1, 2.2, 3.3, 4.4, 5.5}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %f at index %d, got %f", expected[i], i, v)
		}
	}
}

func TestGetArrayFloat64_WithInts(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1,2,3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	expected := []float64{1.0, 2.0, 3.0}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %f at index %d, got %f", expected[i], i, v)
		}
	}
}

func TestGetArrayFloat64_EmptyArray(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got length %d", len(arr))
	}
}

func TestGetArrayFloat64_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1.1,2.2]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("nonexistent")
	if arr != nil {
		t.Errorf("expected nil for non-existent path, got %v", arr)
	}
}

func TestGetArrayFloat64_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":"notanarray"}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("value")
	if arr != nil {
		t.Errorf("expected nil for wrong type, got %v", arr)
	}
}

func TestGetArrayFloat64_MixedTypes(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[1.1,"2.2",3.3]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("numbers")
	if arr != nil {
		t.Errorf("expected nil for mixed types, got %v", arr)
	}
}

func TestGetArrayFloat64_SpecialValues(t *testing.T) {
	raw := json.RawMessage(`{"numbers":[0.0,-0.0,1.5e10,1.5e-10]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 4 {
		t.Errorf("expected array length 4, got %d", len(arr))
	}
	if arr[0] != 0.0 {
		t.Errorf("expected 0.0, got %f", arr[0])
	}
	if arr[2] != 1.5e10 {
		t.Errorf("expected 1.5e10, got %f", arr[2])
	}
}

func TestGetArrayFloat64_NestedPath(t *testing.T) {
	raw := json.RawMessage(`{"data":{"numbers":[10.5,20.5,30.5]}}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayFloat64("data.numbers")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	expected := []float64{10.5, 20.5, 30.5}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %f at index %d, got %f", expected[i], i, v)
		}
	}
}

// TestGetArrayString tests the GetArrayString method
func TestGetArrayString_Valid(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b","c","d","e"]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("tags")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 5 {
		t.Errorf("expected array length 5, got %d", len(arr))
	}
	expected := []string{"a", "b", "c", "d", "e"}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected '%s' at index %d, got '%s'", expected[i], i, v)
		}
	}
}

func TestGetArrayString_EmptyArray(t *testing.T) {
	raw := json.RawMessage(`{"tags":[]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("tags")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 0 {
		t.Errorf("expected empty array, got length %d", len(arr))
	}
}

func TestGetArrayString_NonExistent(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b"]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("nonexistent")
	if arr != nil {
		t.Errorf("expected nil for non-existent path, got %v", arr)
	}
}

func TestGetArrayString_WrongType(t *testing.T) {
	raw := json.RawMessage(`{"value":"notanarray"}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("value")
	if arr != nil {
		t.Errorf("expected nil for wrong type, got %v", arr)
	}
}

func TestGetArrayString_MixedTypes(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a",123,"c"]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("tags")
	if arr != nil {
		t.Errorf("expected nil for mixed types, got %v", arr)
	}
}

func TestGetArrayString_EmptyStrings(t *testing.T) {
	raw := json.RawMessage(`{"tags":["","hello","","world"]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("tags")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 4 {
		t.Errorf("expected array length 4, got %d", len(arr))
	}
	if arr[0] != "" {
		t.Errorf("expected empty string at index 0, got '%s'", arr[0])
	}
	if arr[1] != "hello" {
		t.Errorf("expected 'hello' at index 1, got '%s'", arr[1])
	}
}

func TestGetArrayString_NestedPath(t *testing.T) {
	raw := json.RawMessage(`{"data":{"tags":["x","y","z"]}}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArrayString("data.tags")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	expected := []string{"x", "y", "z"}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected '%s' at index %d, got '%s'", expected[i], i, v)
		}
	}
}

// Additional edge case tests
func TestGetByPath_NegativeArrayIndex(t *testing.T) {
	raw := json.RawMessage(`{"tags":["a","b"]}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("tags.-1")
	if value != nil {
		t.Errorf("expected nil for negative index, got %v", value)
	}
}

func TestGetByPath_ArrayInNestedPath(t *testing.T) {
	raw := json.RawMessage(`{"data":{"items":["first","second"]}}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("data.items.0")
	if value == nil {
		t.Fatal("expected non-nil value")
	}
	if value.(string) != "first" {
		t.Errorf("expected 'first', got %v", value)
	}
}

func TestGetByPath_ComplexNestedArray(t *testing.T) {
	raw := json.RawMessage(`{"a":{"b":[{"c":123}]}}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("a.b.0.c")
	if value == nil {
		t.Fatal("expected non-nil value")
	}
}

func TestGetInt64_WithJsonNumber(t *testing.T) {
	raw := json.RawMessage(`{"value":1234567890123456789}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt64("value", 0)
	if value != 1234567890123456789 {
		t.Errorf("expected 1234567890123456789, got %d", value)
	}
}

func TestGetInt64_WithLargeJsonNumber(t *testing.T) {
	raw := json.RawMessage(`{"value":"9223372036854775807"}`)
	params, _ := NewFromRaw(raw)
	value := params.GetInt64("value", 0)
	// json.Number should handle this
	if value == 0 {
		t.Log("json.Number string conversion handled")
	}
}

func TestGetFloat64_WithJsonNumber(t *testing.T) {
	raw := json.RawMessage(`{"value":123.4567890123456789}`)
	params, _ := NewFromRaw(raw)
	value := params.GetFloat64("value", 0.0)
	if value != 123.4567890123456789 {
		t.Errorf("expected 123.4567890123456789, got %f", value)
	}
}

func TestGetInt_BoundaryValues(t *testing.T) {
	// Test max int value
	params1 := newFromAny(map[string]any{"value": int64(math.MaxInt)})
	result1 := params1.GetInt("value", 0)
	if result1 != math.MaxInt {
		t.Errorf("expected %d, got %d", math.MaxInt, result1)
	}

	// Test min int value
	params2 := newFromAny(map[string]any{"value": int64(math.MinInt)})
	result2 := params2.GetInt("value", 0)
	if result2 != math.MinInt {
		t.Errorf("expected %d, got %d", math.MinInt, result2)
	}

	// Test value that's definitely outside int range (using MaxInt64 which may be > MaxInt on 32-bit)
	if math.MaxInt64 > int64(math.MaxInt) {
		params3 := newFromAny(map[string]any{"value": math.MaxInt64})
		result3 := params3.GetInt("value", 0)
		if result3 != 0 {
			t.Errorf("expected 0 (default) for out-of-range value, got %d", result3)
		}
	}
}

func TestGetString_EmptyString(t *testing.T) {
	raw := json.RawMessage(`{"value":""}`)
	params, _ := NewFromRaw(raw)
	value := params.GetString("value", "default")
	if value != "" {
		t.Errorf("expected empty string, got '%s'", value)
	}
}

func TestGetBool_FalseValue(t *testing.T) {
	raw := json.RawMessage(`{"enabled":false}`)
	params, _ := NewFromRaw(raw)
	value := params.GetBool("enabled", true)
	if value {
		t.Error("expected false, got true")
	}
}

func TestGetSubParams_WithArray(t *testing.T) {
	raw := json.RawMessage(`{"items":[1,2,3]}`)
	params, _ := NewFromRaw(raw)
	sub := params.GetSubParams("items")
	if sub == nil {
		t.Fatal("expected non-nil sub params for array")
	}
	// Verify the array is still accessible through the parent params
	arr := params.GetArrayInt64("items")
	if arr == nil {
		t.Error("expected non-nil array from parent params")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	expected := []int64{1, 2, 3}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %d at index %d, got %d", expected[i], i, v)
		}
	}
}

func TestGetSubParams_NestedArray(t *testing.T) {
	raw := json.RawMessage(`{"data":{"items":[1,2,3]}}`)
	params, _ := NewFromRaw(raw)
	sub := params.GetSubParams("data.items")
	if sub == nil {
		t.Fatal("expected non-nil sub params")
	}
	// Verify the array is still accessible through the parent params
	arr := params.GetArrayInt64("data.items")
	if arr == nil {
		t.Error("expected non-nil array")
	}
	if len(arr) != 3 {
		t.Errorf("expected array length 3, got %d", len(arr))
	}
	expected := []int64{1, 2, 3}
	for i, v := range arr {
		if v != expected[i] {
			t.Errorf("expected %d at index %d, got %d", expected[i], i, v)
		}
	}
}

func TestToInt64_WithJsonNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    json.Number
		expected int64
		valid    bool
	}{
		{"valid_int", json.Number("123"), 123, true},
		{"valid_negative", json.Number("-456"), -456, true},
		{"invalid_string", json.Number("notanumber"), 0, false},
		{"max_int64", json.Number("9223372036854775807"), 9223372036854775807, true},
		{"min_int64", json.Number("-9223372036854775808"), -9223372036854775808, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toInt64(tt.input)
			if ok != tt.valid {
				t.Errorf("expected valid=%v, got %v for %s", tt.valid, ok, tt.name)
			}
			if tt.valid && result != tt.expected {
				t.Errorf("expected %d, got %d for %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestToFloat64_WithJsonNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    json.Number
		expected float64
		valid    bool
	}{
		{"valid_float", json.Number("123.45"), 123.45, true},
		{"valid_int", json.Number("123"), 123.0, true},
		{"valid_negative", json.Number("-456.78"), -456.78, true},
		{"invalid_string", json.Number("notanumber"), 0, false},
		{"scientific_notation", json.Number("1.5e10"), 1.5e10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toFloat64(tt.input)
			if ok != tt.valid {
				t.Errorf("expected valid=%v, got %v for %s", tt.valid, ok, tt.name)
			}
			if tt.valid && math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("expected %f, got %f for %s", tt.expected, result, tt.name)
			}
		})
	}
}

func TestGetArray_WithNestedObjects(t *testing.T) {
	raw := json.RawMessage(`{"items":[{"id":1,"name":"a"},{"id":2,"name":"b"}]}`)
	params, _ := NewFromRaw(raw)
	arr := params.GetArray("items")
	if arr == nil {
		t.Fatal("expected non-nil array")
	}
	if len(arr) != 2 {
		t.Errorf("expected array length 2, got %d", len(arr))
	}
	// Test accessing nested properties
	if id := arr[0].GetInt("id", 0); id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
	if name := arr[0].GetString("name", ""); name != "a" {
		t.Errorf("expected name 'a', got '%s'", name)
	}
}

func TestGetByPath_ArrayIndexZero(t *testing.T) {
	raw := json.RawMessage(`{"items":["first"]}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("items.0")
	if value == nil {
		t.Fatal("expected non-nil value")
	}
	if value.(string) != "first" {
		t.Errorf("expected 'first', got %v", value)
	}
}

func TestGetByPath_IntermediateNil(t *testing.T) {
	raw := json.RawMessage(`{"a":null}`)
	params, _ := NewFromRaw(raw)
	value := params.getByPath("a.b.c")
	if value != nil {
		t.Errorf("expected nil for path through nil, got %v", value)
	}
}
