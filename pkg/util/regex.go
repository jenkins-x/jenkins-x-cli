package util

import "regexp"

// Group is a submatch group
type Group struct {
	Value string
	Start int
	End   int
}

// ReplaceAllStringSubmatchFunc will replace all the submatches found in str by re by calling repl and replacing each
// submatch with the result. Both the argument and the result of repl ignore the entire match (i.e. the item at index 0
// is the first submatch)
func ReplaceAllStringSubmatchFunc(re *regexp.Regexp, str string, repl func(groups []Group) []string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		groups := []Group{}
		for i := 0; i < len(v); i += 2 {
			if i == 0 {
				// Skip the match of the entire string
				continue
			}
			groups = append(groups, Group{
				Value: str[v[i]:v[i+1]],
				Start: v[i],
				End:   v[i+1],
			})
		}
		for i, subst := range repl(groups) {
			group := groups[i]
			result += str[lastIndex:group.Start] + subst
			lastIndex = group.End
		}
	}
	return result + str[lastIndex:]
}
