package options

import "github.com/jenkins-x/jx/pkg/cmd/opts"

// CreateOptions contains the command line options
type CreateOptions struct {
	*opts.CommonOptions

	DisableImport bool
	OutDir        string
}

// Run implements this command
func (o *CreateOptions) Run() error {
	return o.Cmd.Help()
}
