package app

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	optionLogLevel = "log-level"
)

var (
	longHelp = `Generates Go clientsets, OpenAPI spec and API docs for custom resources.

Custom resources are defined using Go structs.

Available generators include:

* openapi - generates OpenAPI specs, required to generate API docs and clients other than Go
* docs -  generates API docs from the OpenAPI specs
* clientset - generates a Go CRUD client directly from custom resources

`
)

// Run executes the Cobra root command.
func Run() error {
	rootCommand := &cobra.Command{
		Use:   "codegen",
		Short: "Uses Golang code-generators to generate various application resources and documentation.",
		Long:  longHelp,
		Run:   runHelp,
	}

	commonOpts := &CommonOptions{
		Cmd: rootCommand,
	}

	genOpts := GenerateOptions{
		CommonOptions: commonOpts,
	}

	rootCommand.PersistentFlags().StringVarP(&commonOpts.LogLevel, optionLogLevel, "", logrus.InfoLevel.String(), "Sets the logging level (panic, fatal, error, warning, info, debug)")
	rootCommand.PersistentFlags().StringVarP(&commonOpts.GeneratorVersion, "generator-version", "", "master",
		"Version (really a commit-ish) of the generator tool to use. Allows to pin version using Go modules. Default is master.")
	rootCommand.PersistentFlags().BoolVarP(&commonOpts.Verbose, optionVerbose, "", false, "Enable verbose logging (sets the logging level to debug)")

	rootCommand.AddCommand(NewGenerateClientSetCmd(genOpts))
	rootCommand.AddCommand(NewCmdCreateClientOpenAPI(genOpts))
	rootCommand.AddCommand(NewCreateDocsCmd(genOpts))

	return rootCommand.Execute()
}

func runHelp(cmd *cobra.Command, _args []string) {
	cmd.Help()
}
