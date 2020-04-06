// +build unit

package gits_test

import (
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestDateFormatAndParse(t *testing.T) {
	t.Parallel()
	now := time.Now()

	expected := util.FormatDate(now)

	tests.Debugf("Formatted %s as %s\n", now.String(), expected)
	parsedTime, err := util.ParseDate(expected)
	assert.Nil(t, err)

	actual := util.FormatDate(parsedTime)

	assert.Equal(t, expected, actual, "Formatted dates for %s", now.String())
}
