package step

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/chartutil"
)

const (
	VERSION = "version"

	defaultVersionFile = "VERSION"

	ValuesYamlRepositoryPrefix = "  repository:"
	ValuesYamlTagPrefix        = "  tag:"
)

// CreateClusterOptions the flags for running create cluster
type StepTagOptions struct {
	step.StepOptions

	Flags StepTagFlags
}

type StepTagFlags struct {
	Version              string
	VersionFile          string
	Dir                  string
	ChartsDir            string
	ChartValueRepository string
	NoApply              bool
}

var (
	stepTagLong = templates.LongDesc(`
		This pipeline step command creates a git tag using a version number prefixed with 'v' and pushes it to a
		remote origin repo.

		This commands effectively runs:

		    $ git commit -a -m "release $(VERSION)" --allow-empty
		    $ git tag -fa v$(VERSION) -m "Release version $(VERSION)"
		    $ git push origin v$(VERSION)

`)

	stepTagExample = templates.Examples(`

		jx step tag --version 1.0.0

`)
)

func NewCmdStepTag(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepTagOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "tag",
		Short:   "Creates a git tag and pushes to remote repo",
		Long:    stepTagLong,
		Example: stepTagExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Flags.Version, VERSION, "v", "", "version number for the tag [required]")
	cmd.Flags().StringVarP(&options.Flags.VersionFile, "version-file", "", defaultVersionFile, "The file name used to load the version number from if no '--version' option is specified")

	cmd.Flags().StringVarP(&options.Flags.ChartsDir, "charts-dir", "d", "", "the directory of the chart to update the version")
	cmd.Flags().StringVarP(&options.Flags.Dir, "dir", "", "", "the directory which may contain a 'jenkins-x.yml'")
	cmd.Flags().StringVarP(&options.Flags.ChartValueRepository, "charts-value-repository", "r", "", "the fully qualified image name without the version tag. e.g. 'dockerregistry/myorg/myapp'")

	cmd.Flags().BoolVarP(&options.Flags.NoApply, "no-apply", "", false, "Do not push the tag to the server, this is used for example in dry runs")

	return cmd
}

func (o *StepTagOptions) Run() error {
	if o.Flags.Version == "" {
		// lets see if its defined in the VERSION file
		path := o.Flags.VersionFile
		if path == "" {
			path = "VERSION"
		}
		exists, err := util.FileExists(path)
		if exists && err == nil {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			o.Flags.Version = strings.TrimSpace(string(data))
		}
	}
	if o.Flags.Version == "" {
		return errors.New("No version flag")
	}
	log.Logger().Debug("looking for charts folder...")
	chartsDir := o.Flags.ChartsDir
	if chartsDir == "" {
		exists, err := util.FileExists("Chart.yaml")
		if !exists && err == nil {
			// lets try find the charts/foo dir ignoring the charts/preview dir
			chartsDir, err = o.findChartsDir()
			if err != nil {
				return err
			}
		}
	}
	log.Logger().Debugf("updating chart if it exists")
	err := o.updateChart(o.Flags.Version, chartsDir)
	if err != nil {
		return err
	}
	err = o.updateChartValues(o.Flags.Version, chartsDir)
	if err != nil {
		return err
	}

	tag := "v" + o.Flags.Version
	log.Logger().Debugf("performing git commit")
	err = o.Git().AddCommit("", fmt.Sprintf("release %s", o.Flags.Version))
	if err != nil {
		return err
	}

	err = o.Git().CreateTag("", tag, fmt.Sprintf("release %s", o.Flags.Version))
	if err != nil {
		return err
	}

	if o.Flags.NoApply {
		log.Logger().Infof("NoApply: no push tag to git server")
	} else {

		log.Logger().Debugf("pushing git tag %s", tag)
		err = o.Git().PushTag("", tag)
		if err != nil {
			return err
		}

		log.Logger().Infof("Tag %s created and pushed to remote origin", tag)
	}
	return nil
}

func (o *StepTagOptions) updateChart(version string, chartsDir string) error {
	chartFile := filepath.Join(chartsDir, "Chart.yaml")

	exists, err := util.FileExists(chartFile)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	chart, err := chartutil.LoadChartfile(chartFile)
	if err != nil {
		return err
	}
	if chart.Version == version {
		return nil
	}
	chart.Version = version
	chart.AppVersion = version
	log.Logger().Infof("Updating chart version in %s to %s", chartFile, version)
	err = chartutil.SaveChartfile(chartFile, chart)
	if err != nil {
		return fmt.Errorf("Failed to save chart %s: %s", chartFile, err)
	}
	return nil
}

func (o *StepTagOptions) updateChartValues(version string, chartsDir string) error {
	valuesFile := filepath.Join(chartsDir, "values.yaml")

	exists, err := util.FileExists(valuesFile)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	data, err := ioutil.ReadFile(valuesFile)
	lines := strings.Split(string(data), "\n")
	chartValueRepository := o.Flags.ChartValueRepository
	if chartValueRepository == "" {
		chartValueRepository = o.defaultChartValueRepository()
	}
	updated := false
	changedRepository := false
	changedTag := false
	for idx, line := range lines {
		if chartValueRepository != "" && strings.HasPrefix(line, ValuesYamlRepositoryPrefix) && !changedRepository {
			// lets ensure we use a valid docker image name
			chartValueRepository = naming.ToValidImageName(chartValueRepository)
			updated = true
			changedRepository = true
			log.Logger().Infof("Updating repository in %s to %s", valuesFile, chartValueRepository)
			lines[idx] = ValuesYamlRepositoryPrefix + " " + chartValueRepository
		} else if strings.HasPrefix(line, ValuesYamlTagPrefix) && !changedTag {
			version = naming.ToValidImageVersion(version)
			updated = true
			changedTag = true
			log.Logger().Infof("Updating tag in %s to %s", valuesFile, version)
			lines[idx] = ValuesYamlTagPrefix + " " + version
		}
	}
	if updated {
		err = ioutil.WriteFile(valuesFile, []byte(strings.Join(lines, "\n")), util.DefaultWritePermissions)
		if err != nil {
			return fmt.Errorf("Failed to save chart file %s: %s", valuesFile, err)
		}
	}
	return nil
}

func (o *StepTagOptions) defaultChartValueRepository() string {
	gitInfo, err := o.FindGitInfo(o.Flags.ChartsDir)
	if err != nil {
		log.Logger().Warnf("failed to find git repository: %s", err.Error())
	}

	projectConfig, _, _ := config.LoadProjectConfig(o.Flags.Dir)
	dockerRegistry := o.GetDockerRegistry(projectConfig)
	dockerRegistryOrg := o.GetDockerRegistryOrg(projectConfig, gitInfo)
	if dockerRegistryOrg == "" {
		dockerRegistryOrg = os.Getenv("ORG")
	}
	if dockerRegistryOrg == "" {
		dockerRegistryOrg = os.Getenv("REPO_OWNER")
	}
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = os.Getenv("REPO_NAME")
	}
	if dockerRegistryOrg == "" && gitInfo != nil {
		dockerRegistryOrg = gitInfo.Organisation
	}
	if appName == "" && gitInfo != nil {
		appName = gitInfo.Name
	}
	if dockerRegistry != "" && dockerRegistryOrg != "" && appName != "" {
		return dockerRegistry + "/" + dockerRegistryOrg + "/" + naming.ToValidName(appName)
	}
	log.Logger().Warnf("could not generate chart repository name for GetDockerRegistry %s, GetDockerRegistryOrg %s, appName %s", dockerRegistry, dockerRegistryOrg, appName)
	return ""
}

// lets try find the charts dir
func (o *StepTagOptions) findChartsDir() (string, error) {
	files, err := filepath.Glob("*/*/Chart.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to find Chart.yaml file: %s", err)
	}
	if len(files) > 0 {
		for _, file := range files {
			paths := strings.Split(file, string(os.PathSeparator))
			if len(paths) > 2 && paths[len(paths)-2] != "preview" {
				dir, _ := filepath.Split(file)
				return dir, nil
			}
		}
	}
	return "", nil
}
