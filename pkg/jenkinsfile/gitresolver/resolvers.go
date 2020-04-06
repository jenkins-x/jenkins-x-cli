package gitresolver

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/util"
	"sigs.k8s.io/yaml"

	"io/ioutil"
	"path/filepath"
)

// CreateResolver creates a new module resolver
func CreateResolver(packsDir string, gitter gits.Gitter) (jenkinsfile.ImportFileResolver, error) {
	modules, err := LoadModules(packsDir)
	if err != nil {
		return nil, err
	}
	moduleResolver, err := ResolveModules(modules, gitter)
	if err != nil {
		return nil, err
	}
	return moduleResolver.AsImportResolver(), nil
}

// Resolve resolves this module to a directory
func Resolve(m *jenkinsfile.Module, gitter gits.Gitter) (*ModuleResolver, error) {
	err := m.Validate()
	if err != nil {
		return nil, err
	}

	dir, err := InitBuildPack(gitter, m.GitURL, m.GitRef)
	if err != nil {
		return nil, err
	}

	answer := &ModuleResolver{
		Module:   m,
		PacksDir: dir,
	}
	return answer, nil
}

// ModulesResolver resolves a number of modules into a structure we can use to resolve imports
type ModulesResolver struct {
	Modules map[string]*ModuleResolver
}

// ModuleResolver a resolver for a single module
type ModuleResolver struct {
	Module   *jenkinsfile.Module
	PacksDir string
}

// LoadModules loads the modules in the given build pack directory if there are any
func LoadModules(dir string) (*jenkinsfile.Modules, error) {
	fileName := filepath.Join(dir, jenkinsfile.ModuleFileName)
	config := jenkinsfile.Modules{}
	exists, err := util.FileExists(fileName)
	if err != nil || !exists {
		return &config, err
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return &config, fmt.Errorf("Failed to load file %s due to %s", fileName, err)
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return &config, fmt.Errorf("Failed to unmarshal YAML file %s due to %s", fileName, err)
	}
	return &config, err
}

// ResolveModules Resolve the modules into a module resolver
func ResolveModules(m *jenkinsfile.Modules, gitter gits.Gitter) (*ModulesResolver, error) {
	answer := &ModulesResolver{
		Modules: map[string]*ModuleResolver{},
	}
	for _, mod := range m.Modules {
		resolver, err := Resolve(mod, gitter)
		if err != nil {
			return answer, err
		}
		answer.Modules[mod.Name] = resolver
	}
	return answer, nil
}

// AsImportResolver returns an ImportFileResolver for these modules
func (m *ModulesResolver) AsImportResolver() jenkinsfile.ImportFileResolver {
	return m.ResolveImport
}

// ResolveImport resolves an import relative to the local git clone of the import
func (m *ModulesResolver) ResolveImport(importFile *jenkinsfile.ImportFile) (string, error) {
	resolver := m.Modules[importFile.Import]
	if resolver == nil {
		return "", nil
	}
	return filepath.Join(resolver.PacksDir, importFile.File), nil
}
