package app

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/cmd/codegen/generator"
	"github.com/jenkins-x/jx/cmd/codegen/util"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"
)

// CreateClientOpenAPIOptions the options for the create client openapi command
type CreateClientOpenAPIOptions struct {
	GenerateOptions
	Title                string
	Version              string
	ReferenceDocsVersion string
	OpenAPIDependencies  []string
	OpenAPIOutputDir     string
	ModuleName           string
}

var (
	createClientOpenAPILong = `This command code generates OpenAPI specs for
the specified custom resources.
`

	createClientOpenAPIExample = `
# lets generate client docs
codegen openapi
	--output-package=github.com/jenkins-x/jx/pkg/client \
	--input-package=github.com/jenkins-x/pkg-apis \
	--group-with-version=jenkins.io:v1
	--version=1.2.3
	--title=Jenkins X

# You will normally want to add a target to your Makefile that looks like
generate-openapi:
	codegen openapi
		--output-package=github.com/jenkins-x/jx/pkg/client \
		--input-package=github.com/jenkins-x/jx/pkg/apis \
		--group-with-version=jenkins.io:v1
		--version=${VERSION}
		--title=${TITLE}

# and then call
make generate-openapi
`
)

// NewCmdCreateClientOpenAPI creates the command
func NewCmdCreateClientOpenAPI(genOpts GenerateOptions) *cobra.Command {
	o := &CreateClientOpenAPIOptions{
		GenerateOptions: genOpts,
	}

	cobraCmd := &cobra.Command{
		Use:     "openapi",
		Short:   "Creates OpenAPI specs for Custom Resources",
		Long:    createClientOpenAPILong,
		Example: createClientOpenAPIExample,

		Run: func(c *cobra.Command, args []string) {
			o.Cmd = c
			o.Args = args
			err := o.Run()
			util.CheckErr(err)
		},
	}

	wd, err := os.Getwd()
	if err != nil {
		util.AppLogger().Warnf("Error getting working directory for %v\n", err)
	}

	openAPIDependencies := []string{
		"k8s.io/apimachinery?modules:pkg/apis:meta:v1",
		"k8s.io/apimachinery?modules:pkg/api:resource:",
		"k8s.io/apimachinery?modules:pkg/util:intstr:",
		"k8s.io/api?modules::batch:v1",
		"k8s.io/api?modules::core:v1",
		"k8s.io/api?modules::rbac:v1",
	}

	moduleName := strings.TrimPrefix(strings.TrimPrefix(wd, filepath.Join(build.Default.GOPATH, "src")), "/")

	cobraCmd.Flags().StringVarP(&o.OutputBase, "output-base", "", wd,
		"Output base directory, by default the current working directory")
	cobraCmd.Flags().StringVarP(&o.BoilerplateFile, optionBoilerplateFile, "", "custom-boilerplate.go.txt",
		"Custom boilerplate to add to all files if the file is missing it will be ignored")
	cobraCmd.Flags().StringVarP(&o.InputBase, optionInputBase, "", wd,
		"Input base (the root of module the OpenAPI is being generated for), by default the current working directory")
	cobraCmd.Flags().StringVarP(&o.InputPackage, optionInputPackage, "i", "", "Input package (relative to input base), "+
		"must specify")
	cobraCmd.Flags().StringVarP(&o.OutputPackage, optionOutputPackage, "o", "", "Output package, must specify")
	cobraCmd.Flags().StringVarP(&o.Title, "title", "", "Jenkins X", "Title for OpenAPI, JSON Schema and HTML docs")
	cobraCmd.Flags().StringVarP(&o.Version, "version", "", "", "Version for OpenAPI, JSON Schema and HTML docs")
	cobraCmd.Flags().StringArrayVarP(&o.OpenAPIDependencies, "open-api-dependency", "", openAPIDependencies,
		"Add <path?modules:package:group:apiVersion> dependencies for OpenAPI generation")
	cobraCmd.Flags().StringVarP(&o.OpenAPIOutputDir, "openapi-output-directory", "",
		"docs/apidocs", "Output directory for the OpenAPI specs, "+
			"relative to the output-base unless absolute. "+
			"OpenAPI spec JSON and YAML files are placed in openapi-spec sub directory.")
	cobraCmd.Flags().StringArrayVarP(&o.GroupsWithVersions, optionGroupWithVersion, "g", make([]string, 0),
		"group name:version (e.g. jenkins.io:v1) to generate, must specify at least once")
	cobraCmd.Flags().StringVarP(&o.ModuleName, optionModuleName, "", moduleName,
		"module name (e.g. github.com/jenkins-x/jx)")
	cobraCmd.Flags().BoolVarP(&o.Global, global, "", false, "use the users GOPATH")
	return cobraCmd
}

// Run implements this command
func (o *CreateClientOpenAPIOptions) Run() error {
	var err error
	o.BoilerplateFile, err = generator.GetBoilerplateFile(o.BoilerplateFile)
	if err != nil {
		return errors.Wrapf(err, "reading file %s specified by %s", o.BoilerplateFile, optionBoilerplateFile)
	}
	if o.InputPackage == "" {
		return util.MissingOption(optionInputPackage)
	}
	if o.OutputPackage == "" {
		return util.MissingOption(optionOutputPackage)
	}

	err = o.configure()
	if err != nil {
		return errors.Wrapf(err, "ensuring GOPATH is set correctly")
	}

	if len(o.GroupsWithVersions) < 1 {
		return util.InvalidOptionf(optionGroupWithVersion, o.GroupsWithVersions, "must specify at least once")
	}

	gopath := util.GoPath()
	if !o.Global {
		gopath, err = util.IsolatedGoPath()
		if err != nil {
			return errors.Wrapf(err, "getting isolated gopath")
		}
	}
	err = generator.InstallOpenApiGen(o.GeneratorVersion, gopath)
	if err != nil {
		return errors.Wrapf(err, "error installing kubernetes openapi tools")
	}

	if !filepath.IsAbs(o.OpenAPIOutputDir) {
		o.OpenAPIOutputDir = filepath.Join(o.OutputBase, o.OpenAPIOutputDir)
	}

	util.AppLogger().Infof("generating Go code to %s in package %s from package %s\n", o.OutputBase, o.GoPathOutputPackage, o.InputPackage)
	err = generator.GenerateOpenApi(o.GroupsWithVersions, o.InputPackage, o.GoPathOutputPackage, o.OutputPackage,
		filepath.Join(build.Default.GOPATH, "src"), o.OpenAPIDependencies, o.InputBase, o.ModuleName, o.BoilerplateFile, gopath)
	if err != nil {
		return errors.Wrapf(err, "generating openapi structs to %s", o.GoPathOutputPackage)
	}

	util.AppLogger().Infof("generating OpenAPI spec files to %s from package %s\n", o.OpenAPIOutputDir, filepath.Join(o.InputBase,
		o.InputPackage))
	err = generator.GenerateSchema(o.OpenAPIOutputDir, o.OutputPackage, o.InputBase, o.Title, o.Version, gopath)
	if err != nil {
		return errors.Wrapf(err, "generating schema to %s", o.OpenAPIOutputDir)
	}
	return nil
}
