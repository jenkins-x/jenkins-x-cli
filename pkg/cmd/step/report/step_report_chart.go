package report

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/api/apps/v1beta1"
)

var (
	stepReportChartLong    = templates.LongDesc(`Creates a report of all the images used in the charts in a version stream".`)
	stepReportChartExample = templates.Examples(`
`)
)

var (
	defaultChartReportName = "chart-images"
)

// ChartReport the report
type ChartReport struct {
	Charts          []*ChartData             `json:"charts,omitempty"`
	DuplicateImages []*DuplicateImageVersion `json:"duplicateImages,omitempty"`
}

// ChartData the image information
type ChartData struct {
	Prefix  string   `json:"prefix,omitempty"`
	Name    string   `json:"name,omitempty"`
	RepoURL string   `json:"url,omitempty"`
	Version string   `json:"version,omitempty"`
	Images  []string `json:"images,omitempty"`
}

// DuplicateImageVersion the duplicate images
type DuplicateImageVersion struct {
	Image    string
	Versions map[string]string
}

// StepReportChartOptions contains the command line flags and other helper objects
type StepReportChartOptions struct {
	StepReportOptions
	VersionsDir     string
	ReportName      string
	FailOnDuplicate bool

	Report ChartReport
}

// NewCmdStepReportChart Creates a new Command object
func NewCmdStepReportChart(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepReportChartOptions{
		StepReportOptions: StepReportOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "chart",
		Short:   "Creates a report of all the images used in the charts in a version stream",
		Long:    stepReportChartLong,
		Example: stepReportChartExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.StepReportOptions.AddReportFlags(cmd)

	cmd.Flags().StringVarP(&options.VersionsDir, "dir", "d", "", "The dir of the version stream. If not specified it the version stream is cloned")
	cmd.Flags().StringVarP(&options.ReportName, "name", "n", defaultChartReportName, "The name of the file to generate")
	cmd.Flags().BoolVarP(&options.FailOnDuplicate, "fail-on-duplicate", "f", false, "If true lets fail the step if we have any duplicate")
	return cmd
}

// Run generates the report
func (o *StepReportChartOptions) Run() error {
	if o.VersionsDir == "" {
		resolver, err := o.GetVersionResolver()
		if err != nil {
			return err
		}
		o.VersionsDir = resolver.VersionsDir
	}
	dir := filepath.Join(o.VersionsDir, "charts")
	exists, err := util.DirExists(dir)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("directory does not exist %s", dir)
	}

	repoPrefixes, err := versionstream.GetRepositoryPrefixes(filepath.Join(dir, ".."))
	if err != nil {
		return err
	}

	report := &o.Report

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		name := info.Name()
		chartsDir := filepath.Dir(path)
		chartName := strings.TrimSuffix(name, filepath.Ext(name))
		_, chartPrefix := filepath.Split(chartsDir)

		if info.IsDir() || name == "repositories.yml" || !strings.HasSuffix(name, ".yml") {
			return nil
		}

		v, err := versionstream.LoadStableVersionFile(path)
		if err != nil {
			return err
		}
		prefixInfos := repoPrefixes.URLsForPrefix(chartPrefix)
		version := v.Version
		if len(prefixInfos) == 0 {
			log.Logger().Warnf("chart %s/%s does not have a repository URL for prefix in registries.yml", chartPrefix, chartName)
			return nil
		}
		if version == "" {
			log.Logger().Warnf("chart %s/%s does not have a version", chartPrefix, chartName)
			return nil
		}
		chartRepo := prefixInfos[0]
		log.Logger().Infof("found chart %s/%s in repo %s with version %s", chartPrefix, chartName, chartRepo, version)

		chartInfo := &ChartData{
			RepoURL: chartRepo,
			Prefix:  chartPrefix,
			Name:    chartName,
			Version: version,
		}
		report.Charts = append(report.Charts, chartInfo)
		err = o.processChart(chartInfo)
		if err != nil {
			return err
		}
		sort.Strings(chartInfo.Images)

		cl := util.ColorInfo
		log.Logger().Infof("chart %s/%s has images: %s", cl(chartInfo.Prefix), cl(chartInfo.Name), cl(strings.Join(chartInfo.Images, ", ")))
		return nil
	})

	imagesDir := filepath.Join(o.VersionsDir, "docker")

	SortChartData(report.Charts)
	err = report.CalculateDuplicates(imagesDir)
	if err != nil {
		return errors.Wrapf(err, "failed to calculate duplicate images")
	}

	if o.OutputDir == "" {
		o.OutputDir = "."
	}
	fileName := o.ReportName
	if fileName != "" {
		fileName += ".yml"
	}
	return o.OutputReport(report, fileName, o.OutputDir)
}

func (o *StepReportChartOptions) processChart(info *ChartData) error {
	tmpDir, err := ioutil.TempDir("", "helm-"+info.Name+"-")
	if err != nil {
		return errors.Wrapf(err, "failed to create tempo dir")
	}

	chartDir := filepath.Join(tmpDir, "chart")
	fullChart := fmt.Sprintf("%s/%s", info.Prefix, info.Name)
	version := info.Version

	// lets fetch the chart
	err = o.RunCommandVerbose("helm", "fetch", "-d", chartDir, "--version", version, "--untar", "--repo", info.RepoURL, info.Name)
	if err != nil {
		return err
	}

	exists, err := util.DirExists(chartDir)
	if err != nil {
		return errors.Wrapf(err, "failed to check chart dir exists %s", chartDir)
	}
	if !exists {
		return fmt.Errorf("should have fetched the chart %s version %s", fullChart, version)
	}

	output, err := o.GetCommandOutput(chartDir, "helm", "template", info.Name)
	if err != nil {
		return err
	}

	files := strings.Split(output, "---\n")
	for _, text := range files {
		if isEmptyTemplate(text) {
			continue
		}
		err = o.processTemplate(info, text)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *StepReportChartOptions) processTemplate(info *ChartData, text string) error {
	data := map[string]interface{}{}

	err := yaml.Unmarshal([]byte(text), &data)
	if err != nil {
		return errors.Wrapf(err, "failed to parse template for chart %s/%s for template: %s", info.Prefix, info.Name, text)
	}

	if util.GetMapValueAsStringViaPath(data, "kind") == "Deployment" {
		d := v1beta1.Deployment{}
		err := yaml.Unmarshal([]byte(text), &d)
		if err != nil {
			return errors.Wrapf(err, "failed to parse Deployment template for chart %s/%s for template: %s", info.Prefix, info.Name, text)
		}

		containers := d.Spec.Template.Spec.Containers
		if len(containers) == 0 {
			log.Logger().Warnf("Deployment template %s for chart %s/%s has no containers", d.Name, info.Prefix, info.Name)
			return nil
		}

		for _, c := range containers {
			if c.Image != "" {
				info.addImage(c.Image)
			}
		}
	}
	return nil
}

func isEmptyTemplate(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		return false
	}
	return true
}

// ImageMap used to map images to their versions
type ImageMap struct {
	Images map[string]map[string]string
}

// AddImage adds a new image to the map
func (m *ImageMap) AddImage(name string, version string, from string) {
	if m.Images == nil {
		m.Images = map[string]map[string]string{}
	}

	versions := m.Images[name]
	if versions == nil {
		versions = map[string]string{}
		m.Images[name] = versions
	}
	versions[version] = from
}

// DuplicateImages returns the duplicate images
func (m *ImageMap) DuplicateImages() []*DuplicateImageVersion {
	answer := []*DuplicateImageVersion{}
	for name, versions := range m.Images {
		if len(versions) > 1 {
			answer = append(answer, &DuplicateImageVersion{
				Image:    name,
				Versions: versions,
			})
		}
	}
	return answer
}

// LoadImageMap loads the images from the given image stream dir
func LoadImageMap(imagesDir string) (ImageMap, error) {
	m := ImageMap{}
	err := filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		name := info.Name()
		if info.IsDir() || !strings.HasSuffix(name, ".yml") {
			return nil
		}

		r, err := filepath.Rel(imagesDir, path)
		if err != nil {
			return errors.Wrapf(err, "failed to calculate relative path")
		}
		r = strings.TrimSuffix(r, ".yml")
		v, err := versionstream.LoadStableVersionFile(path)
		if err != nil {
			log.Logger().Warnf("failed to parse image version file %s due to %s", path, err.Error())
			return nil
		}
		m.AddImage(r, v.Version, "stream")
		return nil
	})
	if err != nil {
		return m, errors.Wrapf(err, "failed to walk images dir %s", imagesDir)
	}
	return m, nil
}

// CalculateDuplicates figures out if there are any duplicate image versions
func (r *ChartReport) CalculateDuplicates(imagesDir string) error {
	m, err := LoadImageMap(imagesDir)
	if err != nil {
		return err
	}

	for _, chart := range r.Charts {
		for _, image := range chart.Images {
			image = strings.Trim(image, "docker.io/")
			idx := strings.Index(image, ":")
			if idx < 0 {
				log.Logger().Warnf("image %s does not have a version", image)
				continue
			}
			r := image[0:idx]
			v := image[idx+1:]
			m.AddImage(r, v, chart.Prefix+"/"+chart.Name+":"+chart.Version)
		}
	}

	// now lets find the duplicates
	r.DuplicateImages = m.DuplicateImages()
	return nil
}

func (i *ChartData) addImage(image string) {
	if util.StringArrayIndex(i.Images, image) < 0 {
		i.Images = append(i.Images, image)
		sort.Strings(i.Images)
	}
}

type chartDataOrder []*ChartData

func (a chartDataOrder) Len() int      { return len(a) }
func (a chartDataOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a chartDataOrder) Less(i, j int) bool {
	r1 := a[i]
	r2 := a[j]

	n1 := r1.Prefix
	n2 := r2.Prefix
	if n1 != n2 {
		return n1 < n2
	}
	v1 := r1.Name
	v2 := r2.Name
	return v1 < v2
}

// SortChartData sorts the charts in order
func SortChartData(items []*ChartData) {
	sort.Sort(chartDataOrder(items))
}
