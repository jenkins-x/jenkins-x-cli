package app

import (
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/cmd/codegen/generator"
	"github.com/jenkins-x/jx/cmd/codegen/util"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

// GenerateDocsOptions contains the options for the create client docs command
type GenerateDocsOptions struct {
	GenerateOptions
	ReferenceDocsVersion string
}

var (
	createClientDocsLong = `This command code generates clients docs (Swagger,OpenAPI and HTML) for
	the specified custom resources.
 
`

	createClientDocsExample = `
# lets generate client docs
codegen docs

# You will normally want to add a target to your Makefile that looks like
generate-clients-docs:
	codegen docs

# and then call
make generate-clients-docs
`
)

// NewCreateDocsCmd creates apidocs for CRDs
func NewCreateDocsCmd(genOpts GenerateOptions) *cobra.Command {
	o := &GenerateDocsOptions{
		GenerateOptions: genOpts,
	}

	cobraCmd := &cobra.Command{
		Use:     "docs",
		Short:   "Creates client docs for Custom Resources",
		Long:    createClientDocsLong,
		Example: createClientDocsExample,

		Run: func(c *cobra.Command, args []string) {
			o.Cmd = c
			o.Args = args
			err := run(o)
			util.CheckErr(err)
		},
	}

	wd, err := os.Getwd()
	if err != nil {
		util.AppLogger().Warnf("error getting working directory for %v\n", err)
	}

	cobraCmd.Flags().StringVarP(&o.InputBase, optionInputBase, "", wd,
		"Input base (root of module), by default the current working directory")
	cobraCmd.Flags().StringVarP(&o.OutputBase, optionOutputBase, "o", filepath.Join(wd, "docs/apidocs"),
		"output base directory, by default the <current working directory>/docs/apidocs")
	cobraCmd.Flags().BoolVarP(&o.Global, global, "", false, "use the users GOPATH")
	return cobraCmd
}

func run(o *GenerateDocsOptions) error {
	var err error
	if o.OutputBase == "" {
		return util.MissingOption(optionOutputBase)
	}
	util.AppLogger().Infof("generating docs to %s\n", o.OutputBase)

	gopath := util.GoPath()
	if !o.Global {
		gopath, err = util.IsolatedGoPath()
		if err != nil {
			return errors.Wrapf(err, "getting isolated gopath")
		}
	}
	err = generator.InstallGenAPIDocs(o.GeneratorVersion, gopath)
	if err != nil {
		return err
	}

	referenceDocsRepo, err := generator.DetermineSourceLocation(o.InputBase, gopath)
	if err != nil {
		return err
	}

	err = generator.GenerateAPIDocs(o.OutputBase, gopath)
	if err != nil {
		return err
	}
	err = generator.AssembleAPIDocsStatic(referenceDocsRepo, o.OutputBase)
	if err != nil {
		return err
	}
	err = generator.AssembleAPIDocs(o.OutputBase, filepath.Join(o.OutputBase, "site"))
	if err != nil {
		return err
	}
	return nil
}
