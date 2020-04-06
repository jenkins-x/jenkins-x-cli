package util

import (
	"fmt"
)

func JoinMap(m map[string]string, pairSep string, keyValueSep string) string {
	answer := ""
	for k, v := range m {
		answer = fmt.Sprintf("%s%s%s%s%s", answer, k, keyValueSep, v, pairSep)
	}
	return answer
}
