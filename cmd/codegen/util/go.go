package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	uuid "github.com/satori/go.uuid"

	"github.com/pkg/errors"
)

const (
	gopath                  = "GOPATH"
	defaultWritePermissions = 0760
)

// GoPath returns the first element of the GOPATH.
// The empty string is returned if GOPATH is not set.
func GoPath() string {
	goPath := os.Getenv(gopath)

	// GOPATH can have multiple elements, we take the first which is consistent with what 'go get' does
	pathElements := strings.Split(goPath, string(os.PathListSeparator))
	path := pathElements[0]
	return path
}

// GoPathSrc returns the src directory of the first GOPATH element.
func GoPathSrc(gopath string) string {
	return filepath.Join(gopath, "src")
}

// GoPathBin returns the bin directory of the first GOPATH element.
func GoPathBin(gopath string) string {
	return filepath.Join(gopath, "bin")
}

// GoPathMod returns the modules directory of the first GOPATH element.
func GoPathMod(gopath string) string {
	return filepath.Join(gopath, "pkg", "mod")
}

// EnsureGoPath ensures the GOPATH environment variable is set and points to a valid directory.
func EnsureGoPath() error {
	goPath := os.Getenv(gopath)
	if goPath == "" {
		return errors.New("GOPATH needs to be set")
	}

	// GOPATH can have multiple elements, if so take the first
	pathElements := strings.Split(goPath, string(os.PathListSeparator))
	path := pathElements[0]
	if len(pathElements) > 1 {
		AppLogger().Debugf("GOPATH contains more than one element using %s", path)
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	} else if os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("the GOPATH directory %s does not exist", path))
	} else {
		return err
	}
}

// GoGet runs go get to install the specified binary.
func GoGet(path string, version string, gopath string, goModules bool, sourceOnly bool, update bool) error {
	modulesMode := "off"
	if goModules {
		modulesMode = "on"
	}

	fullPath := path
	if version != "" {
		if goModules {
			fullPath = fmt.Sprintf("%s@%s", path, version)
		} else {
			fullPath = fmt.Sprintf("%s/...", path)
		}

	}
	args := []string{
		"get",
	}
	if update {
		args = append(args, "-u")
	}
	if sourceOnly || !goModules {
		args = append(args, "-d")
	}
	args = append(args, fullPath)
	goGetCmd := Command{
		Name: "go",
		Args: args,
		Env: map[string]string{
			"GO111MODULE": modulesMode,
			"GOPATH":      gopath,
		},
	}
	out, err := goGetCmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "error running %s, output %s", goGetCmd.String(), out)
	}
	parts := []string{
		GoPathSrc(gopath),
	}
	parts = append(parts, strings.Split(path, "/")...)
	dir := filepath.Join(parts...)
	if !goModules && version != "" {

		branchNameUUID, err := uuid.NewV4()
		if err != nil {
			return errors.WithStack(err)
		}
		branchName := branchNameUUID.String()
		oldBranchName, err := branch(dir)
		if err != nil {
			return errors.Wrapf(err, "getting current branch name")
		}
		err = createBranchFrom(dir, branchName, version)
		if err != nil {
			return errors.Wrapf(err, "creating branch from %s", version)
		}
		err = checkout(dir, branchName)
		defer func() {
			err := checkout(dir, oldBranchName)
			if err != nil {
				AppLogger().Errorf("Error checking out original branch %s: %v", oldBranchName, err)
			}
		}()
		if err != nil {
			return errors.Wrapf(err, "checking out branch from %s", branchName)
		}

	}
	if !sourceOnly && !goModules {
		cmd := Command{
			Dir:  dir,
			Name: "go",
			Args: []string{
				"install",
			},
			Env: map[string]string{
				"GO111MODULE": modulesMode,
				"GOPATH":      gopath,
			},
		}
		out, err = cmd.RunWithoutRetry()
		if err != nil {
			return errors.Wrapf(err, "error running %s, output %s", goGetCmd.String(), out)
		}
	}
	return nil
}

func checkout(dir string, branch string) error {
	return gitCmd(dir, "checkout", branch)
}

// branch returns the current branch of the repository located at the given directory
func branch(dir string) (string, error) {
	return gitCmdWithOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// createBranchFrom creates a new branch called branchName from startPoint
func createBranchFrom(dir string, branchName string, startPoint string) error {
	return gitCmd(dir, "branch", branchName, startPoint)
}

func gitCmd(dir string, args ...string) error {
	cmd := Command{
		Dir:  dir,
		Name: "git",
		Args: args,
	}

	output, err := cmd.RunWithoutRetry()
	return errors.Wrapf(err, "git output: %s", output)
}

func gitCmdWithOutput(dir string, args ...string) (string, error) {
	cmd := Command{
		Dir:  dir,
		Name: "git",
		Args: args,
	}
	return cmd.RunWithoutRetry()
}

// GetModuleDir determines the directory on disk of the specified module dependency.
// Returns the empty string if the target requirement is not part of the module graph.
func GetModuleDir(moduleDir string, targetRequirement string, gopath string) (string, error) {
	out, err := getModGraph(moduleDir, gopath)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return "", errors.Errorf("line of go mod graph should be like '<module> <requirement>' but was %s",
				line)
		}
		requirement := parts[1]
		if strings.HasPrefix(requirement, targetRequirement) {
			return filepath.Join(GoPathMod(gopath), requirement), nil
		}
	}
	return "", nil
}

// GetModuleRequirements returns the requirements for the GO module rooted in dir
// It returns a map[<module name>]map[<requirement name>]<requirement version>
func GetModuleRequirements(dir string, gopath string) (map[string]map[string]string, error) {
	out, err := getModGraph(dir, gopath)
	if err != nil {
		return nil, err
	}

	answer := make(map[string]map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "go: ") {
			// lines that start with go: are things like module download messages
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return nil, errors.Errorf("line of go mod graph should be like '<module> <requirement>' but was %s",
				line)
		}
		moduleName := parts[0]
		requirement := parts[1]
		parts1 := strings.Split(requirement, "@")
		if len(parts1) != 2 {
			return nil, errors.Errorf("go mod graph line should be like '<module> <requirementName>@<requirementVersion>' but was %s", line)
		}
		requirementName := parts1[0]
		requirementVersion := parts1[1]
		if _, ok := answer[moduleName]; !ok {
			answer[moduleName] = make(map[string]string)
		}
		answer[moduleName][requirementName] = requirementVersion
	}
	return answer, nil
}

func getModGraph(dir string, gopath string) (string, error) {
	cmd := Command{
		Dir:  dir,
		Name: "go",
		Args: []string{
			"mod",
			"graph",
		},
		Env: map[string]string{
			"GO111MODULE": "on",
			"GOPATH":      gopath,
		},
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve module graph: %s", out)
	}

	// deal with windows
	out = strings.Replace(out, "\r\n", "\n", -1)

	return out, nil
}

// IsolatedGoPath returns the isolated go path for codegen
func IsolatedGoPath() (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", errors.Wrapf(err, "getting JX_HOME")
	}
	path := filepath.Join(configDir, "codegen", "go")
	err = os.MkdirAll(path, defaultWritePermissions)
	if err != nil {
		return "", errors.Wrapf(err, "making %s", path)
	}
	return path, nil
}

// HomeDir returns the users home directory
func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h := os.Getenv("USERPROFILE") // windows
	if h == "" {
		h = "."
	}
	return h
}

// ConfigDir returns the JX_HOME directory, creating it if missing
func ConfigDir() (string, error) {
	path := os.Getenv("JX_HOME")
	if path != "" {
		return path, nil
	}
	h := HomeDir()
	path = filepath.Join(h, ".jx")
	err := os.MkdirAll(path, defaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}
