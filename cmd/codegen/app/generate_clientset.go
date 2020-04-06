package app

import (
	"go/build"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/cmd/codegen/generator"
	"github.com/jenkins-x/jx/cmd/codegen/util"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

// ClientSetGenerationOptions contain the options for the clientset generation.
type ClientSetGenerationOptions struct {
	GenerateOptions
	Generators []string
}

var (
	createClientGoLong = `This command code generates clients for the specified custom resources.`

	createClientGoExample = `
# lets generate a client
codegen clientset
	--output-package=github.com/jenkins-x/jx/pkg/client \
	--input-package=github.com/jenkins-x/pkg-apis \
	--group-with-version=jenkins.io:v1

# You will normally want to add a target to your Makefile that looks like
generate-clients:
	codegen clientset
		--output-package=github.com/jenkins-x/jx/pkg/client \
		--input-package=github.com/jenkins-x/jx/pkg/apis \
		--group-with-version=jenkins.io:v1

# and then call
make generate-clien
`
)

// NewGenerateClientSetCmd creates the command
func NewGenerateClientSetCmd(genOpts GenerateOptions) *cobra.Command {
	o := &ClientSetGenerationOptions{
		GenerateOptions: genOpts,
	}

	cobraCmd := &cobra.Command{
		Use:     "clientset",
		Short:   "Creates Go client for Custom Resources",
		Long:    createClientGoLong,
		Example: createClientGoExample,

		Run: func(c *cobra.Command, args []string) {
			o.Cmd = c
			o.Args = args
			err := o.Run()
			util.CheckErr(err)
		},
	}

	availableGenerators := []string{
		"deepcopy",
		"clientset",
		"listers",
		"informers",
	}

	wd, err := os.Getwd()
	if err != nil {
		util.AppLogger().Warnf("Error getting working directory for %v\n", err)
	}

	cobraCmd.Flags().StringArrayVarP(&o.Generators, "generator", "", availableGenerators, "Enable a generator")
	cobraCmd.Flags().StringVarP(&o.OutputBase, "output-base", "", wd, "Output base directory, "+
		"by the current working directory")
	cobraCmd.Flags().StringVarP(&o.BoilerplateFile, optionBoilerplateFile, "", "custom-boilerplate.go.txt",
		"Custom boilerplate to add to all files if the file is missing it will be ignored")
	cobraCmd.Flags().StringArrayVarP(&o.GroupsWithVersions, optionGroupWithVersion, "g", make([]string, 0),
		"group name:version (e.g. jenkins.io:v1) to generate, must specify at least once")
	cobraCmd.Flags().StringVarP(&o.InputPackage, optionInputPackage, "i", "", "Input package, must specify")
	cobraCmd.Flags().StringVarP(&o.OutputPackage, optionOutputPackage, "o", "", "Output package, must specify")
	cobraCmd.Flags().StringVarP(&o.InputBase, optionInputBase, "", wd, "Input base, defaults working directory")
	cobraCmd.Flags().BoolVarP(&o.Global, global, "", false, "use the users GOPATH")
	return cobraCmd
}

// Run executes this command.
func (o *ClientSetGenerationOptions) Run() error {
	var err error
	o.BoilerplateFile, err = generator.GetBoilerplateFile(o.BoilerplateFile)
	if err != nil {
		return errors.Wrapf(err, "reading file %s specified by %s", o.BoilerplateFile, optionBoilerplateFile)
	}
	if len(o.GroupsWithVersions) < 1 {
		return util.InvalidOptionf(optionGroupWithVersion, o.GroupsWithVersions, "must specify at least once")
	}
	if o.InputPackage == "" {
		return util.MissingOption(optionInputPackage)
	}
	if o.OutputPackage == "" {
		return util.MissingOption(optionOutputPackage)
	}

	err = o.configure()
	if err != nil {
		return errors.Wrapf(err, "ensure GOPATH is set correctly")
	}

	gopath := util.GoPath()
	if !o.Global {
		gopath, err = util.IsolatedGoPath()
		if err != nil {
			return errors.Wrapf(err, "getting isolated gopath")
		}
	}
	err = generator.InstallCodeGenerators(o.GeneratorVersion, gopath)
	if err != nil {
		return errors.Wrapf(err, "installing kubernetes code generator tools")
	}
	util.AppLogger().Infof("generating Go code to %s in package %s from package %s\n", o.OutputBase, o.GoPathOutputPackage, o.GoPathInputPackage)
	return generator.GenerateClient(o.Generators, o.GroupsWithVersions, o.GoPathInputPackage, o.GoPathOutputPackage,
		filepath.Join(build.Default.GOPATH, "src"), o.BoilerplateFile, gopath)
}
