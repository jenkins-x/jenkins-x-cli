package main

import (
	"flag"
	"os"
	"strings"

	openapi "github.com/jenkins-x/jx/pkg/client/openapi/all"

	"github.com/go-openapi/spec"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/cmd/codegen/generator"
)

func main() {
	var outputDir, namesStr, title, version string
	flag.StringVar(&outputDir, "output-directory", "", "directory to write generated files to")
	flag.StringVar(&namesStr, "names", "", "comma separated list of resources to generate schema for, "+
		"if empty all resources will be generated")
	flag.StringVar(&title, "title", "", "title for OpenAPI and HTML generated docs")
	flag.StringVar(&version, "version", "", "version for OpenAPI and HTML generated docs")
	flag.Parse()
	if outputDir == "" {
		panic(errors.New("--output-directory cannot be empty"))
	}
	var names []string
	if namesStr != "" {
		names = strings.Split(namesStr, ",")
	} else {
		refCallback := func(path string) spec.Ref {
			return spec.Ref{}
		}
		names = openapi.GetNames(refCallback)
	}
	err := generator.WriteSchemaToDisk(outputDir, title, version, openapi.GetOpenAPIDefinitions, names)
	if err != nil {
		panic(errors.Wrapf(err, "writing schema to %s", outputDir))
	}
	os.Exit(0)
}
