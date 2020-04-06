package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/cmd/codegen/util"
	"github.com/pkg/errors"
)

func defaultGenerate(generator string, name string, groupsWithVersions []string, inputPackage string,
	outputPackage string, outputBase string, boilerplateFile string, gopath string, args ...string) error {
	util.AppLogger().Infof("generating %s structs for %s at %s\n", name, groupsWithVersions, outputPackage)

	generateCommand := util.Command{
		Name: filepath.Join(util.GoPathBin(gopath), generator),
		Args: []string{
			"--output-base",
			outputBase,
			"--go-header-file",
			boilerplateFile,
		},
		Env: map[string]string{
			"GO111MODULE": "on",
		},
	}
	if name == "clientset" {
		inputDirs := make([]string, 0)
		for _, gv := range groupsWithVersions {
			groupVersion := strings.Split(gv, ":")
			if len(groupVersion) != 2 {
				return errors.Errorf("argument %s must be like cheese:v1", gv)
			}
			inputDirs = append(inputDirs, fmt.Sprintf("%s/%s", groupVersion[0], groupVersion[1]))
		}
		inputDirsStr := strings.Join(inputDirs, ",")
		generateCommand.Args = append(generateCommand.Args, "--input", inputDirsStr, "--input-base", inputPackage)
	} else {
		inputDirs := make([]string, 0)
		for _, gv := range groupsWithVersions {
			groupVersion := strings.Split(gv, ":")
			if len(groupVersion) != 2 {
				return errors.Errorf("argument %s must be like cheese:v1", gv)
			}
			inputDirs = append(inputDirs, fmt.Sprintf("%s/%s/%s", inputPackage, groupVersion[0], groupVersion[1]))
		}
		inputDirsStr := strings.Join(inputDirs, ",")
		generateCommand.Args = append(generateCommand.Args, "--input-dirs", inputDirsStr)
	}
	for _, arg := range args {
		generateCommand.Args = append(generateCommand.Args, arg)
	}

	out, err := generateCommand.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "running %s, output %s", generateCommand.String(), out)
	}
	return nil
}
