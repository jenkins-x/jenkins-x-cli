package step

import "github.com/jenkins-x/jx/pkg/cmd/opts"

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type StepOptions struct {
	*opts.CommonOptions

	DisableImport bool
	OutDir        string
}

// Run implements this command
func (o *StepOptions) Run() error {
	return o.Cmd.Help()
}

// StepGitMergeOptions contains the command line flags
type StepGitMergeOptions struct {
	StepOptions

	SHAs       []string
	Remote     string
	Dir        string
	BaseBranch string
	BaseSHA    string
}

// StepCreateOptions contains the command line flags
type StepCreateOptions struct {
	StepOptions
}

// StepUpdateOptions contains the command line flags
type StepUpdateOptions struct {
	StepOptions
}
