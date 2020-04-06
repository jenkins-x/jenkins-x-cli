// +build unit

package opts_test

import (
	"bytes"
	"testing"

	expect "github.com/Netflix/go-expect"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
)

func TestExecuteCommand(t *testing.T) {
	t.Parallel()
	o := opts.CommonOptions{}
	err := o.RunCommand("echo", "foo")
	assert.Nil(t, err)
}

func TestCommandError(t *testing.T) {
	t.Parallel()
	o := opts.CommonOptions{}
	err := o.RunCommand("noSuchCommand")
	assert.NotNil(t, err)
}

func TestVerboseOutput(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	t.Parallel()
	buf := new(bytes.Buffer)
	c, err := expect.NewConsole(expect.WithStdout(buf))
	assert.NoError(t, err, "Should not error")
	defer c.Close()
	out := c.Tty()
	o := opts.CommonOptions{Verbose: true, Out: out}
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		c.ExpectEOF()
	}()

	commandResult := o.RunCommand("echo", "foo")

	// Close the slave end of the pty, and read the remaining bytes from the master end.
	out.Close()
	<-donec

	assert.NoError(t, commandResult, "Should not error")
	assert.Equal(t, "foo\r", expect.StripTrailingEmptyLines(buf.String()))
}

func TestNonVerboseOutput(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	t.Parallel()
	console := tests.NewTerminal(t, nil)
	defer console.Close()
	defer console.Cleanup()
	o := opts.CommonOptions{Out: console.Out}
	err := o.RunCommand("echo", "foo")
	assert.NoError(t, err, "Should not error")
	assert.Empty(t, expect.StripTrailingEmptyLines(console.CurrentState()))
}
