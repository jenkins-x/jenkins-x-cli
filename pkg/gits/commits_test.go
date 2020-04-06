// +build unit

package gits_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/stretchr/testify/assert"
)

func TestParseCommits(t *testing.T) {
	t.Parallel()
	assertParseCommit(t, "something regular", &gits.CommitInfo{
		Message: "something regular",
	})
	assertParseCommit(t, "feat: cheese", &gits.CommitInfo{
		Kind:    "feat",
		Message: "cheese",
	})
	assertParseCommit(t, "feat(beer): wine is good too", &gits.CommitInfo{
		Kind:    "feat",
		Feature: "beer",
		Message: "wine is good too",
	})
}

func assertParseCommit(t *testing.T, input string, expected *gits.CommitInfo) {
	info := gits.ParseCommit(input)
	assert.NotNil(t, info)
	assert.Equal(t, expected.Kind, info.Kind, "Kind for Commit %s", info)
	assert.Equal(t, expected.Feature, info.Feature, "Feature for Commit %s", info)
	assert.Equal(t, expected.Message, info.Message, "Message for Commit %s", info)
	assert.Equal(t, expected, info, "CommitInfo for Commit %s", info)
}
