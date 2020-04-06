// +build unit

package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestLoggingSetup(t *testing.T) {
	origLogLevel, exists := os.LookupEnv("JX_LOG_LEVEL")
	if exists {
		defer func() {
			_ = os.Setenv("JX_LOG_LEVEL", origLogLevel)
		}()
	}

	var logTests = []struct {
		envLogLevel    string
		verbose        bool
		expectedOutput string
	}{
		{"trace", false, "TRACE: trace\nDEBUG: debug\ninfo\nWARNING: warn\nERROR: error\n"},
		{"trace", true, "TRACE: trace\nDEBUG: debug\ninfo\nWARNING: warn\nERROR: error\n"},
		{"debug", false, "DEBUG: debug\ninfo\nWARNING: warn\nERROR: error\n"},
		{"debug", true, "DEBUG: debug\ninfo\nWARNING: warn\nERROR: error\n"},
		{"info", false, "info\nWARNING: warn\nERROR: error\n"},
		{"info", true, "info\nWARNING: warn\nERROR: error\n"},
		{"warn", false, "WARNING: warn\nERROR: error\n"},
		{"warn", true, "WARNING: warn\nERROR: error\n"},
		{"error", false, "ERROR: error\n"},
		{"error", true, "ERROR: error\n"},
		{"", true, "DEBUG: debug\ninfo\nWARNING: warn\nERROR: error\n"},
		{"", false, "info\nWARNING: warn\nERROR: error\n"},
		{"foo", false, "info\nWARNING: warn\nERROR: error\n"},
		{"foo", true, "info\nWARNING: warn\nERROR: error\n"},
	}

	testCommandName := "logtest"
	for _, logTest := range logTests {
		t.Run(fmt.Sprintf("JX_LOG_LEVEL=%s verbose=%t", logTest.envLogLevel, logTest.verbose), func(t *testing.T) {
			if logTest.envLogLevel == "" {
				err := os.Unsetenv("JX_LOG_LEVEL")
				assert.NoError(t, err)
			} else {
				err := os.Setenv("JX_LOG_LEVEL", logTest.envLogLevel)
				assert.NoError(t, err)
			}

			logCommand := &cobra.Command{
				Use:   testCommandName,
				Short: "dummy test command",
				Run: func(cmd *cobra.Command, args []string) {
					out := log.CaptureOutput(func() {
						log.Logger().Trace("trace")
						log.Logger().Debug("debug")
						log.Logger().Info("info")
						log.Logger().Warn("warn")
						log.Logger().Error("error")
					})

					assert.Equal(t, logTest.expectedOutput, out)
				},
			}

			rootCmd := NewJXCommand(fake.NewFakeFactory(), os.Stdin, os.Stdout, os.Stderr, nil)
			rootCmd.AddCommand(logCommand)

			args := []string{testCommandName}
			if logTest.verbose {
				args = append(args, "--verbose")
			}
			rootCmd.SetArgs(args)
			_ = log.CaptureOutput(func() {
				err := rootCmd.Execute()
				assert.NoError(t, err)
			})
		})
	}
}
