// +build unit

package create

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateClusterName(t *testing.T) {
	var bigLongName = string("this-name-is-too-long-by-one")
	var capitalName = string("NameWithCapitalLetters")
	var gibberishName = string("l337n@me")
	var goodName = string("good-name-for-cluster")
	t.Parallel()
	tests := []struct {
		name        string
		clusterName string
		want        bool
	}{
		// Negative tests for bad names. Should return false.
		{"Fails when too long", bigLongName, false},
		{"Fails with capital letters", capitalName, false},
		{"Fails with gibberish name", gibberishName, false},
		// Positive tests with good names. Should return true.
		{"Passes with good name", goodName, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClusterName(tt.clusterName)
			nameIsValid := false
			if err == nil {
				nameIsValid = true
			}
			assert.Equal(t, nameIsValid, tt.want)
		})
	}
}

func TestAddLabel(t *testing.T) {
	label := AddLabel("", "created-by", "test.user")
	assert.Equal(t, "created-by=test-user", label, "Label formed incorrectly")
}
