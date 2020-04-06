package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/cmd/codegen/util"
	"github.com/pkg/errors"
)

const (
	basePath       = "k8s.io/code-generator/cmd"
	defaultVersion = "kubernetes-1.11.3"
)

var (
	// AllGenerators is a list of all the generators provide by kubernetes code-generator
	allGenerators = map[string]string{
		"clientset": "client-gen",
		"defaulter": "defaulter-gen",
		"listers":   "lister-gen",
		"informers": "informer-gen",
		"deepcopy":  "deepcopy-gen",
	}
)

// InstallCodeGenerators installs client-gen from the kubernetes-incubator/reference-docs repository.
func InstallCodeGenerators(version string, gopath string) error {
	if version == "" {
		version = defaultVersion
	}
	for _, generator := range allGenerators {
		path := fmt.Sprintf("%s/%s", basePath, generator)
		util.AppLogger().Infof("installing %s version %s into %s", path, version, gopath)
		err := util.GoGet(path, version, gopath, false, false, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// GenerateClient runs the generators specified. It looks at the specified groupsWithVersions in inputPackage and generates to outputPackage (
//// relative to the module outputBase). A boilerplateFile is written to the top of any generated files.
func GenerateClient(generators []string, groupsWithVersions []string, inputPackage string, outputPackage string,
	outputBase string, boilerplateFile string, gopath string) error {
	for _, generatorName := range generators {
		if generator, ok := allGenerators[generatorName]; ok {
			switch generatorName {
			case "clientset":
				err := generateWithOutputPackage(generator,
					generatorName,
					groupsWithVersions,
					inputPackage,
					outputPackage,
					outputBase,
					boilerplateFile,
					gopath,
					"--clientset-name",
					"versioned")
				if err != nil {
					return err
				}
			case "deepcopy":
				err := defaultGenerate(generator,
					generatorName,
					groupsWithVersions,
					inputPackage,
					outputPackage,
					outputBase,
					boilerplateFile,
					gopath,
					"-O",
					"zz_generated.deepcopy",
					"--bounding-dirs",
					inputPackage)
				if err != nil {
					return err
				}
			case "informers":
				err := generateWithOutputPackage(generator,
					generatorName,
					groupsWithVersions,
					inputPackage,
					outputPackage,
					outputBase,
					boilerplateFile,
					gopath,
					"--versioned-clientset-package",
					fmt.Sprintf("%s/clientset/versioned", outputPackage),
					"--listers-package",
					fmt.Sprintf("%s/listers", outputPackage))
				if err != nil {
					return err
				}
			default:
				err := generateWithOutputPackage(generator, generatorName, groupsWithVersions, inputPackage,
					outputPackage, outputBase, boilerplateFile, gopath)
				if err != nil {
					return err
				}
			}
		} else {
			return errors.Errorf("no generator %s available", generatorName)
		}
	}
	return nil
}

// GetBoilerplateFile is responsible for canonicalizing the name of the boilerplate file
func GetBoilerplateFile(fileName string) (string, error) {
	if fileName != "" {
		if _, err := os.Stat(fileName); err != nil && !os.IsNotExist(err) {
			return "", errors.Wrapf(err, "error reading boilerplate file %s", fileName)
		} else if err == nil {
			abs, err := filepath.Abs(fileName)
			if err == nil {
				fileName = abs
			} else {
				util.AppLogger().Errorf("error converting %s to absolute path so leaving as is %v", fileName, err)
			}
		}
	}
	return fileName, nil
}

func generateWithOutputPackage(generator string, name string, groupsWithVersions []string,
	inputPackage string, outputPackage string, outputBase string, boilerplateFile string, gopath string, args ...string) error {
	args = append(args, "--output-package", fmt.Sprintf("%s/%s", outputPackage, name))
	return defaultGenerate(generator, name, groupsWithVersions, inputPackage, outputPackage, outputBase,
		boilerplateFile, gopath, args...)
}
