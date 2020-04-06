// +build unit

package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_debug_log_is_written_to_output_when_corresponding_level_is_set(t *testing.T) {
	err := SetLevel("info")
	assert.NoError(t, err)

	out := CaptureOutput(func() { Logger().Debug("hello") })
	assert.Empty(t, out)

	err = SetLevel("debug")
	assert.NoError(t, err)

	out = CaptureOutput(func() { Logger().Debug("hello") })
	assert.Equal(t, "DEBUG: hello\n", out)
}

func Test_setting_unknown_log_level_returns_error(t *testing.T) {
	err := SetLevel("foo")
	assert.Error(t, err)
	assert.Equal(t, "Invalid log level 'foo'", err.Error())
}
