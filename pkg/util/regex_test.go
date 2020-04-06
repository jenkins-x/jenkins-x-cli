// +build unit

package util_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/util"
)

func TestReplaceAllStringSubmatchFunc(t *testing.T) {
	incrementVersionRepl := func(groups []util.Group) []string {
		answer := make([]string, 0)
		for _, group := range groups {
			i, err := strconv.Atoi(group.Value)
			assert.NoError(t, err)
			answer = append(answer, fmt.Sprintf("%d", i+1))
		}
		return answer
	}
	brewVersionRepl := func(groups []util.Group) []string {
		answer := make([]string, 0)
		answer = append(answer, "1.0.20")
		return answer
	}
	brewShaRepl := func(groups []util.Group) []string {
		answer := make([]string, 0)
		answer = append(answer, "ef7a95c23bc5858cff6fd2825836af7e8342a9f6821d91ddb0b5b5f87f0f4e85")
		return answer
	}
	type args struct {
		re   *regexp.Regexp
		str  string
		repl func([]util.Group) []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "JX_VERSION",
			args: args{
				re:   regexp.MustCompile(`JX_VERSION=(.*)`),
				str:  "JX_VERSION=10",
				repl: incrementVersionRepl,
			},
			want: "JX_VERSION=11",
		},
		{
			name: "release",
			args: args{
				re:   regexp.MustCompile(`\s*release = \"(.*)\"`),
				str:  "    release = \"10\"",
				repl: incrementVersionRepl,
			},
			want: "    release = \"11\"",
		},
		{
			name: "twice",
			args: args{
				re:   regexp.MustCompile(`\s*release = \"(.*)\"`),
				str:  "    release = \"10\"\n    release = \"8\"",
				repl: incrementVersionRepl,
			},
			want: "    release = \"11\"\n    release = \"9\"",
		},
		{
			name: "brew-version",
			args: args{
				re:   regexp.MustCompile(`\s*version \"(.*)\"`),
				str:  "  version \"1.0.1\"",
				repl: brewVersionRepl,
			},
			want: "  version \"1.0.20\"",
		},
		{
			name: "brew-sha",
			args: args{
				re:   regexp.MustCompile(`\s*sha256 \"(.*)\"`),
				str:  "  sha256 \"7d7d380c5f0760027ae73f1663a1e1b340548fd93f68956e6b0e2a0d984774fa\"",
				repl: brewShaRepl,
			},
			want: "  sha256 \"ef7a95c23bc5858cff6fd2825836af7e8342a9f6821d91ddb0b5b5f87f0f4e85\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.ReplaceAllStringSubmatchFunc(tt.args.re, tt.args.str, tt.args.repl); got != tt.want {
				t.Errorf("ReplaceAllStringSubmatchFunc() = %v, want %v", got, tt.want)
			}
		})
	}
}
