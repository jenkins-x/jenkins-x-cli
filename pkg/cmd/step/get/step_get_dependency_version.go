package get

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/dependencymatrix"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	getDependencyVersionLong = templates.LongDesc(`
		Outputs the version of a specific dependency from the dependency matrix in the version stream or a local directory
`)

	getDependencyVersionExample = templates.Examples(`
		# display the version of jx in the version stream
		jx step get dependency-version --host=github.com --owner=jenkins-x --repo=jx

		# display the version of jx in a local directory containing a "dependency-matrix" subdirectory, only logging the version
		jx step get dependency-version --host=github.com --owner=jenkins-x --repo=jx --dir=/some/directory --short
			`)
)

// StepGetDependencyVersionOptions contains the command line flags
type StepGetDependencyVersionOptions struct {
	step.StepOptions

	Host        string
	Owner       string
	Repo        string
	Dir         string
	ShortOutput bool
}

// NewCmdStepGetDependencyVersion Creates a new Command object
func NewCmdStepGetDependencyVersion(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepGetDependencyVersionOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "dependency-version",
		Short:   "Outputs the version of a dependency from the Jenkins X dependency matrix",
		Long:    getDependencyVersionLong,
		Example: getDependencyVersionExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Host, "host", "", "", "Host for dependency repo in the matrix")
	cmd.Flags().StringVarP(&options.Owner, "owner", "", "", "Owner for dependency repo in the matrix")
	cmd.Flags().StringVarP(&options.Repo, "repo", "", "", "Repo name for dependency repo in the matrix")
	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "Directory to read dependency matrix from instead of using the version stream")
	cmd.Flags().BoolVarP(&options.ShortOutput, "short", "", false, "Display the dependency version only")
	return cmd
}

// Run implements this command
func (o *StepGetDependencyVersionOptions) Run() error {
	var missingArgs []string

	if o.Host == "" {
		missingArgs = append(missingArgs, "host")
	}
	if o.Owner == "" {
		missingArgs = append(missingArgs, "owner")
	}
	if o.Repo == "" {
		missingArgs = append(missingArgs, "repo")
	}
	if len(missingArgs) > 0 {
		return fmt.Errorf("one or more required arguments are missing: %s", strings.Join(missingArgs, ", "))
	}

	if o.Dir == "" {
		resolver, err := o.GetVersionResolver()
		if err != nil {
			return err
		}
		o.Dir = resolver.VersionsDir
	}

	matrix, err := dependencymatrix.LoadDependencyMatrix(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load dependency matrix at %s", o.Dir)
	}

	version, err := matrix.FindVersionForDependency(o.Host, o.Owner, o.Repo)
	if err != nil {
		return err
	}
	if o.ShortOutput {
		fmt.Fprintf(o.Out, "%s\n", version)
		return nil
	}
	fmt.Fprintf(o.Out, "Version for host %s, owner %s, repo %s in matrix at %s is: %s\n", o.Host, o.Owner, o.Repo, o.Dir, version)
	return nil
}
