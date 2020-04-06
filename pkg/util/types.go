package util

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

var floatType = reflect.TypeOf(float64(0))
var intType = reflect.TypeOf(int64(0))
var stringType = reflect.TypeOf("")
var boolType = reflect.TypeOf(false)

// AsFloat64 attempts to convert unk to a float64
func AsFloat64(unk interface{}) (float64, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(floatType) {
		return 0, fmt.Errorf("cannot convert %v (%v) to float64", v.Type(), v)
	}
	fv := v.Convert(floatType)
	return fv.Float(), nil
}

// AsInt64 attempts to convert unk to an int64
func AsInt64(unk interface{}) (int64, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(intType) {
		return 0, fmt.Errorf("cannot convert %v (%v) to int64", v.Type(), v)
	}
	iv := v.Convert(intType)
	return iv.Int(), nil
}

// AsString attempts to convert unk to a string
func AsString(unk interface{}) (string, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(stringType) {
		return "", fmt.Errorf("cannot convert %v (%v) to string", v.Type(), v)
	}
	sv := v.Convert(stringType)
	return sv.String(), nil
}

// AsBool attempts to convert unk to a bool
func AsBool(unk interface{}) (bool, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)
	if !v.Type().ConvertibleTo(boolType) {
		// See if it's a string we can parse
		str, err := AsString(unk)
		if err == nil {
			result, err := ParseBool(str)
			if err != nil {
				return false, errors.Wrapf(err, "cannot parse string %v as bool", str)
			}
			return result, nil
		}
		return false, fmt.Errorf("cannot convert %v (%v) to bool", v.Type(), v)
	}
	bv := v.Convert(boolType)
	return bv.Bool(), nil
}

//AsSliceOfStrings attempts to convert unk to a slice of strings
func AsSliceOfStrings(unk interface{}) ([]string, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)

	result := make([]string, 0)
	for i := 0; i < v.Len(); i++ {
		iv := v.Index(i)
		// TODO Would be nice to type check this, but not sure how
		result = append(result, fmt.Sprintf("%v", iv))
	}
	return result, nil
}

//AsMapOfStringsIntefaces attempts to convert unk to a map[string]interface{}
func AsMapOfStringsIntefaces(unk interface{}) (map[string]interface{}, error) {
	v := reflect.ValueOf(unk)
	v = reflect.Indirect(v)

	if v.Kind() != reflect.Map {
		return make(map[string]interface{}), fmt.Errorf("cannot convert %v (%v) to map[string]interface{}", v.Type(), v)
	}
	result := make(map[string]interface{})
	for _, key := range v.MapKeys() {
		result[key.String()] = v.MapIndex(key).Interface()
	}
	return result, nil
}

// DereferenceInt will return the int value or the empty value for int
func DereferenceInt(i *int) int {
	if i != nil {
		return *i
	}
	return 0
}

// DereferenceInt64 will return the int value or the empty value for i
func DereferenceInt64(i *int64) int64 {
	if i != nil {
		return *i
	}
	return 0
}

// DereferenceString will return the string value or the empty value for string
func DereferenceString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// DereferenceFloat64 will return the float64 value or the empty value for float64
func DereferenceFloat64(f *float64) float64 {
	if f != nil {
		return *f
	}
	return 0
}

// IsZeroOfUnderlyingType checks if the underlying type of the interface is set to it's zero value
func IsZeroOfUnderlyingType(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

// DereferenceBool will return the bool value or the empty value for bool
func DereferenceBool(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}
