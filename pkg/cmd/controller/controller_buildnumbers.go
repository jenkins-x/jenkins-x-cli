package controller

import (
	"github.com/jenkins-x/jx/pkg/buildnum"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

const (
	command    = "buildnumbers"
	optionPort = "port"
	optionBind = "bind"
)

// ControllerBuildNumbersOptions holds the options for the build number service.
type ControllerBuildNumbersOptions struct {
	*opts.CommonOptions
	BindAddress string
	Port        int
}

var (
	serveBuildNumbersLong = templates.LongDesc(`Runs the build number controller that serves sequential build 
		numbers over an HTTP interface.`)

	serveBuildNumbersExample = templates.Examples("jx " + command)
)

// NewCmdControllerBuildNumbers builds a new command to serving build numbers over an HTTP interface.
func NewCmdControllerBuildNumbers(commonOpts *opts.CommonOptions) *cobra.Command {
	options := ControllerBuildNumbersOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     command,
		Short:   "Runs the service to generate build numbers.",
		Long:    serveBuildNumbersLong,
		Example: serveBuildNumbersExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().IntVarP(&options.Port, optionPort, "", 8080, "The TCP port to listen on.")
	cmd.Flags().StringVarP(&options.BindAddress, optionBind, "", "",
		"The interface address to bind to (by default, will listen on all interfaces/addresses).")
	return cmd
}

// Run will execute this command, starting the HTTP build number generation service with the specified options.
func (o *ControllerBuildNumbersOptions) Run() error {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	buildNumGen := buildnum.NewCRDBuildNumGen(jxClient, ns)

	httpBuildNumServer := buildnum.NewHTTPBuildNumberServer(o.BindAddress, o.Port, buildNumGen)
	return httpBuildNumServer.Start()
}
