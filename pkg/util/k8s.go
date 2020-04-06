package util

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EncodeKubernetesName takes a string and turns it into a form suitable for use as a Kubernetes name.
// Note there is no decode functionality provided atm (and things like escaping escape codes aren't handled), so
// round-tripping is not possible yet.
// K8S names are lower case, numbers, '-', and '.'. Invalid characters are percent-encoded in a _similar_ way to URL
// encoding, with a period ('.') followed by the numerical character code. Conversion is:
//   Upper case letters -> lower case.
//   'a-z', '0-9', '-', '.' -> left as-is.
//   Any other characters: percent-encoded ('.' + rune hex code).
func EncodeKubernetesName(name string) string {
	encodedName := strings.Builder{}

	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '.' {
			//allowed in name.
			encodedName.WriteRune(ch)
		} else if ch >= 'A' && ch <= 'Z' {
			//Apply toLower encoding.
			encodedName.WriteString(fmt.Sprintf(".%c", unicode.ToLower(ch)))
		} else {
			//Apply character-code encoding.
			encodedName.WriteString(fmt.Sprintf(".%X", int(ch)))
		}
	}
	return encodedName.String()
}

// DurationString returns the duration between start and end time as string
func DurationString(start *metav1.Time, end *metav1.Time) string {
	if start == nil || end == nil {
		return ""
	}
	return end.Sub(start.Time).Round(time.Second).String()
}
