package tests

import (
	"bytes"
	"strings"

	expect "github.com/Netflix/go-expect"
	"github.com/acarl005/stripansi"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

// ConsoleWrapper is a wrapper around the go-expect Console that takes a test object and will report failures
// This prevents users having to manually detect and report errors during the tests
type ConsoleWrapper struct {
	tester  assert.TestingT
	console *expect.Console
	state   *vt10x.State
	terminal.Stdio
}

// Analogous to https://github.com/Netflix/go-expect/blob/9d1f4485533ba0665fa4ca2ef6f1210198f7eaae/expect_opt.go#L158-L177 but ignoring ANSI escapes.
type strippedStringMatcher struct {
	str string
}

func (sm *strippedStringMatcher) Match(v interface{}) bool {
	buf, ok := v.(*bytes.Buffer)
	if !ok {
		return false
	}
	if strings.Contains(stripansi.Strip(buf.String()), sm.str) {
		return true
	}
	return false
}

func (sm *strippedStringMatcher) Criteria() interface{} {
	return sm.str
}

// ExpectString expects a string to be present on the console and fails the test if it is not
func (c *ConsoleWrapper) ExpectString(s string) {
	out, err := c.console.Expect(func(opts *expect.ExpectOpts) error {
		// analogous to https://github.com/Netflix/go-expect/blob/9d1f4485533ba0665fa4ca2ef6f1210198f7eaae/expect_opt.go#L245-L251
		opts.Matchers = append(opts.Matchers, &strippedStringMatcher{
			str: s,
		})
		return nil
	})
	assert.NoError(c.tester, err, "Expected string: %q\nActual string: %q", s, stripansi.Strip(out))
}

// SendLine sends a string to the console and fails the test if something goes wrong
func (c *ConsoleWrapper) SendLine(s string) {
	_, err := c.console.SendLine(s)
	assert.NoError(c.tester, err, "Error sending line %s", s)
}

// ExpectEOF expects an EOF to be present on the console and reports an error if it is not
func (c *ConsoleWrapper) ExpectEOF() {
	out, err := c.console.ExpectEOF()
	assert.NoError(c.tester, err, "Expected EOF. Got %q", stripansi.Strip(out))
}

// Close closes the console input
func (c *ConsoleWrapper) Close() {
	err := c.console.Tty().Close()
	assert.NoError(c.tester, err)
}

// Cleanup closes all resources the console was using
func (c *ConsoleWrapper) Cleanup() {
	err := c.console.Close()
	assert.NoError(c.tester, err)
}

// CurrentState gets the last line of text currently on the console
func (c *ConsoleWrapper) CurrentState() string {
	return c.state.String()
}
