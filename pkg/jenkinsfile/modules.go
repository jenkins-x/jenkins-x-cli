package jenkinsfile

import "github.com/jenkins-x/jx/pkg/util"

const (
	// ModuleFileName the name of the module imports file name
	ModuleFileName = "imports.yaml"
)

// ImportFile represents an import of a file from a module (usually a version of a git repo)
type ImportFile struct {
	Import string
	File   string
}

// ImportFileResolver resolves a build pack file resolver strategy
type ImportFileResolver func(importFile *ImportFile) (string, error)

// Modules defines the dependent modules for a build pack
type Modules struct {
	Modules []*Module `json:"modules,omitempty"`
}

// Module defines a dependent module for a build pack
type Module struct {
	Name   string `json:"name,omitempty"`
	GitURL string `json:"gitUrl,omitempty"`
	GitRef string `json:"gitRef,omitempty"`
}

// Validate returns an error if any data is missing
func (m *Module) Validate() error {
	if m.GitURL == "" {
		return util.MissingOption("GitURL")
	}
	return nil
}
