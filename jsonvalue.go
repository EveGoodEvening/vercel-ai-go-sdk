package gateway

import (
	"math"
	"reflect"
	"sort"
	"strconv"
)

func validationError(path, reason string) *ValidationError {
	return &ValidationError{path: path, reason: reason}
}

func memberPath(path, key string) string      { return path + "[" + strconv.Quote(key) + "]" }
func indexPath(path string, index int) string { return path + "[" + strconv.Itoa(index) + "]" }

type jsonVisit struct {
	typ reflect.Type
	ptr unsafePointer
}

type unsafePointer uintptr

type jsonValidator struct {
	active map[jsonVisit]bool
}

func validateJSONValue(value any, path string) *ValidationError {
	return (&jsonValidator{active: make(map[jsonVisit]bool)}).value(reflect.ValueOf(value), path, false)
}

func validateJSONInput(value any, path string) *ValidationError {
	if value == nil {
		return validationError(path, "required")
	}
	validator := &jsonValidator{active: make(map[jsonVisit]bool)}
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return validationError(path, "required")
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return validationError(path, "required")
	}
	if (v.Kind() == reflect.Map || v.Kind() == reflect.Slice) && v.IsNil() {
		return validationError(path, "required")
	}
	switch v.Kind() {
	case reflect.String, reflect.Map, reflect.Slice, reflect.Array:
		return validator.value(reflect.ValueOf(value), path, false)
	default:
		if err := validator.value(reflect.ValueOf(value), path, false); err != nil {
			return err
		}
		return validationError(path, "must be a string, object, or array")
	}
}

func (v *jsonValidator) value(value reflect.Value, path string, allowNull bool) *ValidationError {
	if !value.IsValid() {
		if allowNull {
			return nil
		}
		return validationError(path, "required")
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			if allowNull {
				return nil
			}
			return validationError(path, "required")
		}
		if value.Kind() == reflect.Pointer {
			visit := jsonVisit{typ: value.Type(), ptr: unsafePointer(uintptr(value.UnsafePointer()))}
			if v.active[visit] {
				return validationError(path, "cycle detected")
			}
			v.active[visit] = true
			defer delete(v.active, visit)
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Bool, reflect.String:
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return nil
	case reflect.Float32, reflect.Float64:
		if math.IsNaN(value.Float()) || math.IsInf(value.Float(), 0) {
			return validationError(path, "number must be finite")
		}
		return nil
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return validationError(path, "map keys must be strings")
		}
		if value.IsNil() {
			return nil
		}
		visit := jsonVisit{typ: value.Type(), ptr: unsafePointer(uintptr(value.UnsafePointer()))}
		if v.active[visit] {
			return validationError(path, "cycle detected")
		}
		v.active[visit] = true
		defer delete(v.active, visit)
		keys := value.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, key := range keys {
			if err := v.value(value.MapIndex(key), memberPath(path, key.String()), true); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		if value.IsNil() {
			return nil
		}
		visit := jsonVisit{typ: value.Type(), ptr: unsafePointer(uintptr(value.UnsafePointer()))}
		if v.active[visit] {
			return validationError(path, "cycle detected")
		}
		v.active[visit] = true
		defer delete(v.active, visit)
		for i := range value.Len() {
			if err := v.value(value.Index(i), indexPath(path, i), true); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		for i := range value.Len() {
			if err := v.value(value.Index(i), indexPath(path, i), true); err != nil {
				return err
			}
		}
		return nil
	default:
		return validationError(path, "must be JSON-compatible")
	}
}

// normalizeJSONValue converts an already validated value into the small set of
// built-in Go values whose encoding/json representation matches the JSON shape
// accepted by jsonValidator. In particular, it strips named types and their
// methods, and treats every slice (including []byte) as a JSON array.
func normalizeJSONValue(value any) any {
	return normalizeJSONReflectValue(reflect.ValueOf(value))
}

func normalizeJSONReflectValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Bool:
		return value.Bool()
	case reflect.String:
		return value.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint()
	case reflect.Float32:
		return float32(value.Float())
	case reflect.Float64:
		return value.Float()
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		normalized := make(map[string]any, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			normalized[iterator.Key().String()] = normalizeJSONReflectValue(iterator.Value())
		}
		return normalized
	case reflect.Slice:
		if value.IsNil() {
			return nil
		}
		fallthrough
	case reflect.Array:
		normalized := make([]any, value.Len())
		for index := range value.Len() {
			normalized[index] = normalizeJSONReflectValue(value.Index(index))
		}
		return normalized
	default:
		panic("normalizeJSONValue called without validation")
	}
}

func isJSONNull(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
