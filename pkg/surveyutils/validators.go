package surveyutils

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/iancoleman/orderedmap"

	"gopkg.in/AlecAivazis/survey.v1"
)

// MinLengthValidator validates that val is longer in length than minLength
func MinLengthValidator(minLength *int) survey.Validator {
	if minLength != nil {
		return survey.MinLength(util.DereferenceInt(minLength))
	}
	return NoopValidator()
}

// MaxLengthValidator validates that val is shorter in length than maxLength
func MaxLengthValidator(maxLength *int) survey.Validator {
	if maxLength != nil {
		return survey.MaxLength(util.DereferenceInt(maxLength))
	}
	return NoopValidator()
}

// RequiredValidator applies the RequiredValidator if required is true
func RequiredValidator(required bool) survey.Validator {
	if required {
		return survey.Required
	}
	return NoopValidator()
}

// EnumValidator validates that val appears in the enum
func EnumValidator(enum []interface{}) survey.Validator {
	return func(val interface{}) error {
		if len(enum) > 0 {
			found := false
			for _, e := range enum {
				if e == val {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("%v not found in %v", val, enum)
			}
		}
		return nil
	}
}

// NoopValidator always passes (use instead of nil in a slice of validators)
func NoopValidator() survey.Validator {
	return func(val interface{}) error {
		return nil
	}
}

// DateTimeValidator validates that a string is a RFC 3339 date-time format
func DateTimeValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = time.Parse(time.RFC3339, str)
		if err != nil {
			return fmt.Errorf("%s is not a RFC 3339 date-time formatted string, it should be like %s", str,
				time.RFC3339)
		}
		return nil
	}
}

const (
	rfc3339FullDate = "2006-01-02"
	rfc3339FullTime = "15:04:05Z07:00"
)

// DateValidator validates that a string is a RFC 3339 full-date format
func DateValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = time.Parse(rfc3339FullDate, str)
		if err != nil {
			return fmt.Errorf("%s is not a RFC 3339 full-date formatted string, it should be like %s", str,
				rfc3339FullDate)
		}
		return nil
	}
}

// TimeValidator validates that a string is a RFC3339 full-time format
func TimeValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = time.Parse(rfc3339FullTime, str)
		if err != nil {
			return fmt.Errorf("%s is not a RFC 3339 full-time formatted string, it should be like %s", str,
				rfc3339FullTime)
		}
		return nil
	}
}

// EmailValidator validates that a string is a RFC 5322 email
func EmailValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = mail.ParseAddress(str)
		if err != nil {
			return fmt.Errorf("%s is not a RFC 5322 address, it should be like Barry Gibb <bg@example.com>", str)
		}
		return nil
	}
}

var rfc1034Regex = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

// HostnameValidator validates that a string is a RFC 1034 hostname
func HostnameValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		match := rfc1034Regex.MatchString(str)
		if !match {
			return fmt.Errorf("%s is not a RFC 1034 hostname, it should be like example.com", str)
		}
		return nil
	}
}

// Ipv4Validator validates that a string is a RFC 2673 IPv4 address
func Ipv4Validator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}

		ip := net.ParseIP(str)
		// Check if it didn't parsed, and that it's not a IPv4 address
		if ip == nil && ip.To4() == nil {
			return fmt.Errorf("%s is not a RFC 2673 IPv4 Address, it should be like 127.0.0.1", str)
		}
		return nil
	}
}

// Ipv6Validator validates that a string is a RFC 4291 IPv6 address
func Ipv6Validator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		ip := net.ParseIP(str)
		// Check if it didn't parse and that it's IPv4 address
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("%s is not a RFC 4291 IPv6 address, it should be like ::1", str)
		}
		return nil
	}
}

// URIValidator validates that a string is a valid RFC 3986 URI
func URIValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		u, err := url.Parse(str)
		if err != nil || !u.IsAbs() {
			return fmt.Errorf("%s is not a RFC 3986 URI", str)
		}
		return nil
	}
}

// URIReferenceValidator validates that a string is a valid RFC 3986 URI Reference
func URIReferenceValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = url.Parse(str)
		if err != nil {
			return fmt.Errorf("%s is not a RFC 3986 URI reference", str)
		}
		return nil
	}
}

var rfc6901Regex = regexp.MustCompile(`(/(([^/~])|(~[01]))*)`)

// JSONPointerValidator validates that a string is a JSON Pointer
func JSONPointerValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		match := rfc6901Regex.MatchString(str)
		if !match {
			return fmt.Errorf("%s is not a RFC 6901 JSON pointer", str)
		}
		return nil
	}
}

//FloatValidator validates that val is a float
func FloatValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = strconv.ParseFloat(str, 64)
		if err != nil {
			return fmt.Errorf("unable to convert %s to float64", str)
		}
		return nil
	}
}

//IntegerValidator validates that val is an int
func IntegerValidator() survey.Validator {
	return func(val interface{}) error {
		str, err := util.AsString(val)
		if err != nil {
			return err
		}
		_, err = strconv.Atoi(str)
		if err != nil {
			return fmt.Errorf("unable to convert %s to int", str)
		}
		return nil
	}
}

//BoolValidator validates that val is a bool
func BoolValidator() survey.Validator {
	return func(val interface{}) error {
		_, err := util.AsBool(val)
		if err != nil {
			return errors.Wrapf(err, "unable to convert %v to bool", val)
		}
		return nil
	}
}

// OverrideAnswerValidator will validate the answer supplied as an argument, rather the answer the user provides
// this is useful when you want to validate the value a confirm dialog is confirming, rather than the Y/n
func OverrideAnswerValidator(ans interface{}, validator survey.Validator) survey.Validator {
	return func(val interface{}) error {
		return validator(ans)
	}
}

// MinValidator validates that the val is more than the min, if exclusive then more than or equal to
func MinValidator(min *float64, exclusive bool) survey.Validator {
	return func(val interface{}) error {
		if min != nil {
			minValue := util.DereferenceFloat64(min)
			var value float64
			strVal, err := util.AsString(val)
			if err != nil {
				return errors.Wrapf(err, "unable to convert %v to a string", val)
			}
			// See if val is a float
			if fValue, err := strconv.ParseFloat(strVal, 64); err != nil {
				// See if val is an int
				iValue, err := strconv.ParseInt(strVal, 10, 64)
				value = float64(iValue)
				if err != nil {
					return errors.Wrapf(err, "unable to convert %v to a int64 or a float64", val)
				}
			} else {
				value = fValue
			}
			if exclusive {
				if value <= minValue {
					return fmt.Errorf("%v is not greater than %v", val, *min)
				}
			} else {
				if value < minValue {
					return fmt.Errorf("%v is not greater than or equal to %v", val, *min)
				}
			}
		}
		return nil
	}
}

// MaxValidator validates that the val is less than the max, if exclusive, then less than or equal to
func MaxValidator(max *float64, exclusive bool) survey.Validator {
	return func(val interface{}) error {
		if max != nil {
			maxValue := util.DereferenceFloat64(max)

			var value float64
			strVal, err := util.AsString(val)
			if err != nil {
				return errors.Wrapf(err, "unable to convert %v to a string", val)
			}
			// See if val is a float
			if fValue, err := strconv.ParseFloat(strVal, 64); err != nil {
				// See if val is an int
				iValue, err := strconv.ParseInt(strVal, 10, 64)
				value = float64(iValue)
				if err != nil {
					return errors.Wrapf(err, "unable to convert %v to a int64 or a float64", val)
				}
			} else {
				value = fValue
			}
			if exclusive {
				if value >= maxValue {
					return fmt.Errorf("%v is not less than %v", val, *max)
				}
			} else {
				if value > maxValue {
					return fmt.Errorf("%v is not less than or equal to %v", val, *max)
				}
			}

		}
		return nil
	}
}

// MultipleOfValidator validates that the val is a multiple of multipleOf
func MultipleOfValidator(multipleOf *float64) survey.Validator {
	return func(val interface{}) error {
		if multipleOf != nil {
			multipleOfValue := util.DereferenceFloat64(multipleOf)
			var fValue float64
			strVal, err := util.AsString(val)
			if err != nil {
				return errors.Wrapf(err, "unable to convert %v to a string", val)
			}
			// See if val is a float
			if fValue, err = strconv.ParseFloat(strVal, 64); err != nil {
				// See if val is an int
				value, err := strconv.ParseInt(strVal, 10, 64)
				fValue = float64(value)
				if err != nil {
					return errors.Wrapf(err, "unable to convert %v to a int64 or a float64", val)
				}
			}
			res := fValue / multipleOfValue
			if res != float64(int64(res)) {
				return fmt.Errorf("%v cannot be divided by %v", val, *multipleOf)
			}
		}
		return nil
	}
}

// MaxItemsValidator validates that at most the maxItems number of items exist in a slice
func MaxItemsValidator(maxItems *int, value []interface{}) survey.Validator {
	return func(val interface{}) error {
		if maxItems != nil {
			maxItemsValue := util.DereferenceInt(maxItems)
			if len(value) > maxItemsValue {
				return fmt.Errorf("%d has more than %d items", value, maxItemsValue)
			}
		}
		return nil
	}
}

// MaxPropertiesValidator validates that at most the maxItems number of key-value pairs exist in a map
func MaxPropertiesValidator(maxItems *int, value *orderedmap.OrderedMap, key string) survey.Validator {
	return func(val interface{}) error {
		if maxItems != nil {
			maxItemsValue := util.DereferenceInt(maxItems)
			if len(value.Keys()) > maxItemsValue {
				return fmt.Errorf("%v has more than %d items, has %v", key, maxItemsValue, value.Keys())
			}
		}
		return nil
	}
}

// MinItemsValidator validates that at least the minItems number of items exist in a slice
func MinItemsValidator(minItems *int, value []interface{}) survey.Validator {
	return func(val interface{}) error {
		if minItems != nil {
			minItemsValue := util.DereferenceInt(minItems)
			if len(value) < minItemsValue {
				return fmt.Errorf("%d has less than %d items", value, minItemsValue)
			}
		}
		return nil
	}
}

// MinPropertiesValidator validates that at least the minItems number of key-value pairs exist in a map
func MinPropertiesValidator(minItems *int, value *orderedmap.OrderedMap, key string) survey.Validator {
	return func(val interface{}) error {
		if minItems != nil {
			minItemsValue := util.DereferenceInt(minItems)
			if len(value.Keys()) < minItemsValue {
				return fmt.Errorf("%v has less than %d items, has %v", key, minItemsValue, value.Keys())
			}
		}
		return nil
	}
}

//UniqueItemsValidator validates that the val is unique in a slice
func UniqueItemsValidator(value []interface{}) survey.Validator {
	return func(val interface{}) error {
		set := make(map[interface{}]bool)
		for _, item := range value {
			if v, ok := set[item]; ok {
				return fmt.Errorf("%v is not unique in %v", v, value)
			}
			set[item] = true
		}
		return nil
	}
}

//NoWhiteSpaceValidator is an input validator for the survey package that disallows any whitespace in the val
func NoWhiteSpaceValidator() survey.Validator {
	// return a validator that ensures the given string does not contain any whitespace
	return func(val interface{}) error {
		if str, ok := val.(string); ok {
			if strings.ContainsAny(str, " ") {
				// yell loudly
				return fmt.Errorf("supplied value \"%v\" must not contain any whitespace", str)
			}
		}
		// the input is fine
		return nil
	}
}

//PatternValidator validates that the val matches the regex pattern
func PatternValidator(pattern *string) survey.Validator {
	return func(val interface{}) error {
		if pattern != nil {
			str, err := util.AsString(val)
			if err != nil {
				return err
			}
			regexp, err := regexp.Compile(*pattern)
			if err != nil {
				return err
			}
			if !regexp.MatchString(str) {
				return fmt.Errorf("%v does not match %s", val, *pattern)
			}
		}
		return nil
	}
}
