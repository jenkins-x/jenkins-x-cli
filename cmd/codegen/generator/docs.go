package generator

import (
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/cmd/codegen/util"
	"github.com/pkg/errors"
)

const (
	genAPIDocsRepo = "github.com/kubernetes-incubator/reference-docs"
	genAPIDocsBin  = genAPIDocsRepo + "/gen-apidocs"
)

// InstallGenAPIDocs installs the gen-apidocs tool from the kubernetes-incubator/reference-docs repository.
func InstallGenAPIDocs(version string, gopath string) error {
	util.AppLogger().Infof("installing %s in version %s via 'go get'", genAPIDocsBin, version)
	err := util.GoGet(genAPIDocsBin, version, gopath, true, false, true)
	if err != nil {
		return err
	}

	return nil
}

// DetermineSourceLocation determines the source location for the installed kubernetes-incubator/reference-docs/
// The location is based on GOPATH/pkd/mod and the current version.
func DetermineSourceLocation(moduleDir string, gopath string) (string, error) {
	moduleDir, err := util.GetModuleDir(moduleDir, genAPIDocsRepo, gopath)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to determine source directory for %s", genAPIDocsRepo)
	}
	return moduleDir, nil
}

// GenerateAPIDocs runs the apidocs-gen tool against configDirectory which includes the openapi-spec dir,
// the config.yaml file, static content and the static_includes
func GenerateAPIDocs(configDir string, gopath string) error {
	includesDir := filepath.Join(configDir, "includes")
	err := util.DeleteDirContents(includesDir)
	if err != nil {
		return errors.Wrapf(err, "deleting contents of %s", includesDir)
	}
	buildDir := filepath.Join(configDir, "build")
	err = util.DeleteDirContents(buildDir)
	if err != nil {
		return errors.Wrapf(err, "deleting contents of %s", buildDir)
	}
	cmd := util.Command{
		Dir:  configDir,
		Name: filepath.Join(util.GoPathBin(gopath), "gen-apidocs"),
		Args: []string{
			"--config-dir",
			configDir,
			"--munge-groups",
			"false",
		},
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "running %s, output %s", cmd.String(), out)
	}
	util.AppLogger().Debugf("running %s\n", cmd.String())
	util.AppLogger().Debug(out)
	return nil
}

// AssembleAPIDocsStatic copies the static files from the referenceDocsRepo to the outputDir.
// It also downloads from CDN jquery and bootstrap js
func AssembleAPIDocsStatic(referenceDocsRepo string, outputDir string) error {
	srcDir := filepath.Join(referenceDocsRepo, "gen-apidocs", "generators", "static")
	outDir := filepath.Join(outputDir, "static")
	util.AppLogger().Infof("copying static files from %s to %s\n", srcDir, outDir)
	err := util.CopyDirPreserve(srcDir, outDir)
	if err != nil {
		return errors.Wrapf(err, "copying %s to %s", srcDir, outDir)
	}
	err = util.DownloadFile(filepath.Join(outDir, bootstrapJsFileName), bootstrapJsUrl)
	if err != nil {
		return err
	}
	err = util.DownloadFile(filepath.Join(outDir, jqueryFileName), jqueryUrl)
	if err != nil {
		return err
	}
	return nil
}

// AssembleAPIDocs copies the generated html files and the static files from srcDir into outputDir
func AssembleAPIDocs(srcDir string, outputDir string) error {
	// Clean the dir
	err := util.DeleteDirContents(outputDir)
	if err != nil {
		return errors.Wrapf(err, "deleting contents of %s", outputDir)
	}
	// Copy the fonts over
	err = copyStaticFiles(filepath.Join(srcDir, "static"), filepath.Join(outputDir, "fonts"), fonts)
	if err != nil {
		return err
	}
	// Copy the css over
	err = copyStaticFiles(filepath.Join(srcDir, "static"), filepath.Join(outputDir, "css"), css)
	if err != nil {
		return err
	}
	// Copy the static jsroot over
	err = copyStaticFiles(filepath.Join(srcDir, "static"), filepath.Join(outputDir, ""), jsroot)
	if err != nil {
		return err
	}

	// Copy the static js over
	err = copyStaticFiles(filepath.Join(srcDir, "static"), filepath.Join(outputDir, "js"), js)
	if err != nil {
		return err
	}
	// Copy the generated files over
	err = copyStaticFiles(filepath.Join(srcDir, "build"), filepath.Join(outputDir, ""), build)
	if err != nil {
		return err
	}
	return nil
}

func copyStaticFiles(srcDir string, outputDir string, resources []string) error {
	err := os.MkdirAll(outputDir, 0700)
	if err != nil {
		return errors.Wrapf(err, "making %s", outputDir)
	}
	for _, resource := range resources {
		srcPath := filepath.Join(srcDir, resource)
		dstPath := filepath.Join(outputDir, resource)
		err := util.CopyFile(srcPath, dstPath)
		if err != nil {
			return errors.Wrapf(err, "copying %s to %s", srcPath, dstPath)
		}
	}
	return nil
}
