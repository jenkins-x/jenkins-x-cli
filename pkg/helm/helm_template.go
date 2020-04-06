package helm

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// AnnotationChartName stores the chart name
	AnnotationChartName = "jenkins.io/chart"
	// AnnotationAppVersion stores the chart's app version
	AnnotationAppVersion = "jenkins.io/chart-app-version"
	// AnnotationAppDescription stores the chart's app version
	AnnotationAppDescription = "jenkins.io/chart-description"
	// AnnotationAppRepository stores the chart's app repository
	AnnotationAppRepository = "jenkins.io/chart-repository"

	// LabelReleaseName stores the chart release name
	LabelReleaseName = "jenkins.io/chart-release"

	// LabelNamespace stores the chart namespace for cluster wide resources
	LabelNamespace = "jenkins.io/namespace"

	// LabelReleaseChartVersion stores the version of a chart installation in a label
	LabelReleaseChartVersion = "jenkins.io/version"
	// LabelAppName stores the chart's app name
	LabelAppName = "jenkins.io/app-name"
	// LabelAppVersion stores the chart's app version
	LabelAppVersion = "jenkins.io/app-version"

	hookFailed    = "hook-failed"
	hookSucceeded = "hook-succeeded"

	// resourcesSeparator is used to separate multiple objects stored in the same YAML file
	resourcesSeparator = "---"
)

// HelmTemplate implements common helm actions but purely as client side operations
// delegating a separate Helmer such as HelmCLI for the client side operations
type HelmTemplate struct {
	Client          *HelmCLI
	WorkDir         string
	CWD             string
	Binary          string
	Runner          util.Commander
	KubectlValidate bool
	KubeClient      kubernetes.Interface
	Namespace       string
}

// NewHelmTemplate creates a new HelmTemplate instance configured to the given client side Helmer
func NewHelmTemplate(client *HelmCLI, workDir string, kubeClient kubernetes.Interface, ns string) *HelmTemplate {
	cli := &HelmTemplate{
		Client:          client,
		WorkDir:         workDir,
		Runner:          client.Runner,
		Binary:          "kubectl",
		CWD:             client.CWD,
		KubectlValidate: false,
		KubeClient:      kubeClient,
		Namespace:       ns,
	}
	return cli
}

type HelmHook struct {
	Kind               string
	Name               string
	File               string
	Hooks              []string
	HookDeletePolicies []string
}

// SetHost is used to point at a locally running tiller
func (h *HelmTemplate) SetHost(tillerAddress string) {
	// NOOP
}

// SetCWD configures the common working directory of helm CLI
func (h *HelmTemplate) SetCWD(dir string) {
	h.Client.SetCWD(dir)
	h.CWD = dir
}

// HelmBinary return the configured helm CLI
func (h *HelmTemplate) HelmBinary() string {
	return h.Client.HelmBinary()
}

// SetHelmBinary configure a new helm CLI
func (h *HelmTemplate) SetHelmBinary(binary string) {
	h.Client.SetHelmBinary(binary)
}

// Init executes the helm init command according with the given flags
func (h *HelmTemplate) Init(clientOnly bool, serviceAccount string, tillerNamespace string, upgrade bool) error {
	return h.Client.Init(true, serviceAccount, tillerNamespace, upgrade)
}

// AddRepo adds a new helm repo with the given name and URL
func (h *HelmTemplate) AddRepo(repo, URL, username, password string) error {
	return h.Client.AddRepo(repo, URL, username, password)
}

// RemoveRepo removes the given repo from helm
func (h *HelmTemplate) RemoveRepo(repo string) error {
	return h.Client.RemoveRepo(repo)
}

// ListRepos list the installed helm repos together with their URL
func (h *HelmTemplate) ListRepos() (map[string]string, error) {
	return h.Client.ListRepos()
}

// SearchCharts searches for all the charts matching the given filter
func (h *HelmTemplate) SearchCharts(filter string, allVersions bool) ([]ChartSummary, error) {
	return h.Client.SearchCharts(filter, false)
}

// IsRepoMissing checks if the repository with the given URL is missing from helm
func (h *HelmTemplate) IsRepoMissing(URL string) (bool, string, error) {
	return h.Client.IsRepoMissing(URL)
}

// UpdateRepo updates the helm repositories
func (h *HelmTemplate) UpdateRepo() error {
	return h.Client.UpdateRepo()
}

// RemoveRequirementsLock removes the requirements.lock file from the current working directory
func (h *HelmTemplate) RemoveRequirementsLock() error {
	return h.Client.RemoveRequirementsLock()
}

// BuildDependency builds the helm dependencies of the helm chart from the current working directory
func (h *HelmTemplate) BuildDependency() error {
	return h.Client.BuildDependency()
}

// ListReleases lists the releases in ns
func (h *HelmTemplate) ListReleases(ns string) (map[string]ReleaseSummary, []string, error) {
	list, err := h.KubeClient.AppsV1().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	charts := make(map[string]ReleaseSummary)
	keys := make([]string, 0)
	if list != nil {
		for _, deploy := range list.Items {
			labels := deploy.Labels
			ann := deploy.Annotations
			if labels != nil && ann != nil {
				status := "ERROR"
				if deploy.Status.Replicas > 0 {
					if deploy.Status.UnavailableReplicas > 0 {
						status = "PENDING"
					} else {
						status = "DEPLOYED"
					}
				}
				updated := deploy.CreationTimestamp.Format("Mon Jan 2 15:04:05 2006")
				chartName := ann[AnnotationChartName]
				chartVersion := labels[LabelReleaseChartVersion]
				releaseName := labels[LabelReleaseName]
				keys = append(keys, releaseName)
				charts[releaseName] = ReleaseSummary{
					Chart:         chartName,
					ChartFullName: chartName + "-" + chartVersion,
					Revision:      strconv.FormatInt(deploy.Generation, 10),
					Updated:       updated,
					Status:        status,
					ChartVersion:  chartVersion,
					ReleaseName:   releaseName,
					AppVersion:    ann[AnnotationAppVersion],
					Namespace:     ns,
				}
			}
		}
	}
	return charts, keys, nil
}

// FindChart find a chart in the current working directory, if no chart file is found an error is returned
func (h *HelmTemplate) FindChart() (string, error) {
	return h.Client.FindChart()
}

// Lint lints the helm chart from the current working directory and returns the warnings in the output
func (h *HelmTemplate) Lint(valuesFiles []string) (string, error) {
	return h.Client.Lint(valuesFiles)
}

// Env returns the environment variables for the helmer
func (h *HelmTemplate) Env() map[string]string {
	return h.Client.Env()
}

// PackageChart packages the chart from the current working directory
func (h *HelmTemplate) PackageChart() error {
	return h.Client.PackageChart()
}

// Version executes the helm version command and returns its output
func (h *HelmTemplate) Version(tls bool) (string, error) {
	return h.Client.Version(tls)
}

// Template generates the YAML from the chart template to the given directory
func (h *HelmTemplate) Template(chart string, releaseName string, ns string, outDir string, upgrade bool, values []string, valueStrings []string,
	valueFiles []string) error {

	return h.Client.Template(chart, releaseName, ns, outDir, upgrade, values, valueStrings, valueFiles)
}

// Mutation API

// InstallChart installs a helm chart according with the given flags
func (h *HelmTemplate) InstallChart(chart string, releaseName string, ns string, version string, timeout int,
	values []string, valueStrings []string, valueFiles []string, repo string, username string, password string) error {

	err := h.clearOutputDir(releaseName)
	if err != nil {
		return err
	}
	outputDir, _, chartsDir, err := h.getDirectories(releaseName)
	if err != nil {
		return err
	}

	chartDir, err := h.fetchChart(chart, version, chartsDir, repo, username, password)
	if err != nil {
		return err
	}
	err = h.Client.Template(chartDir, releaseName, ns, outputDir, false, values, valueStrings, valueFiles)
	if err != nil {
		return err
	}

	// Skip the chart when no resources are generated by the template
	if empty, err := util.IsEmpty(outputDir); empty || err != nil {
		return nil
	}

	metadata, versionText, err := h.getChart(chartDir, version)
	if err != nil {
		return err
	}

	helmHooks, err := h.addLabelsToFiles(chart, releaseName, versionText, metadata, ns)
	if err != nil {
		return err
	}
	helmCrdPhase := "crd-install"
	helmPrePhase := "pre-install"
	helmPostPhase := "post-install"
	wait := true
	create := true
	force := true

	err = h.runHooks(helmHooks, helmCrdPhase, ns, chart, releaseName, wait, create, force)
	if err != nil {
		return err
	}

	err = h.runHooks(helmHooks, helmPrePhase, ns, chart, releaseName, wait, create, force)
	if err != nil {
		return err
	}

	err = h.kubectlApply(ns, releaseName, wait, create, force, outputDir)
	if err != nil {
		err2 := h.deleteHooks(helmHooks, helmPrePhase, hookFailed, ns)
		return util.CombineErrors(err, err2)
	}
	err = h.deleteHooks(helmHooks, helmPrePhase, hookSucceeded, ns)
	if err != nil {
		log.Logger().Warnf("Failed to delete the %s hook, due to: %s", helmPrePhase, err)
	}

	err = h.runHooks(helmHooks, helmPostPhase, ns, chart, releaseName, wait, create, force)
	if err != nil {
		err2 := h.deleteHooks(helmHooks, helmPostPhase, hookFailed, ns)
		return util.CombineErrors(err, err2)
	}

	err = h.deleteHooks(helmHooks, helmPostPhase, hookSucceeded, ns)
	err2 := h.deleteOldResources(ns, releaseName, versionText, wait)

	return util.CombineErrors(err, err2)
}

// FetchChart fetches a Helm Chart
func (h *HelmTemplate) FetchChart(chart string, version string, untar bool, untardir string, repo string,
	username string, password string) error {
	_, err := h.fetchChart(chart, version, untardir, repo, username, password)
	return err
}

// UpgradeChart upgrades a helm chart according with given helm flags
func (h *HelmTemplate) UpgradeChart(chart string, releaseName string, ns string, version string, install bool, timeout int, force bool, wait bool, values []string, valueStrings []string, valueFiles []string, repo string, username string, password string) error {
	err := h.clearOutputDir(releaseName)
	if err != nil {
		return err
	}
	outputDir, _, chartsDir, err := h.getDirectories(releaseName)
	if err != nil {
		return err
	}

	// check if we are installing a chart from the filesystem
	chartDir := filepath.Join(h.CWD, chart)
	exists, err := util.DirExists(chartDir)
	if err != nil {
		return err
	}
	if !exists {
		log.Logger().Debugf("Fetching chart: %s", chart)
		chartDir, err = h.fetchChart(chart, version, chartsDir, repo, username, password)
		if err != nil {
			return err
		}
	}
	err = h.Client.Template(chartDir, releaseName, ns, outputDir, false, values, valueStrings, valueFiles)
	if err != nil {
		return err
	}

	// Skip the chart when no resources are generated by the template
	if empty, err := util.IsEmpty(outputDir); empty || err != nil {
		return nil
	}

	metadata, versionText, err := h.getChart(chartDir, version)
	if err != nil {
		return err
	}

	helmHooks, err := h.addLabelsToFiles(chart, releaseName, versionText, metadata, ns)
	if err != nil {
		return err
	}

	helmCrdPhase := "crd-install"
	helmPrePhase := "pre-upgrade"
	helmPostPhase := "post-upgrade"
	create := false

	err = h.runHooks(helmHooks, helmCrdPhase, ns, chart, releaseName, wait, create, force)
	if err != nil {
		return err
	}

	err = h.runHooks(helmHooks, helmPrePhase, ns, chart, releaseName, wait, create, force)
	if err != nil {
		return err
	}

	err = h.kubectlApply(ns, releaseName, wait, create, force, outputDir)
	if err != nil {
		err2 := h.deleteHooks(helmHooks, helmPrePhase, hookFailed, ns)
		return util.CombineErrors(err, err2)
	}
	err = h.deleteHooks(helmHooks, helmPrePhase, hookSucceeded, ns)
	if err != nil {
		log.Logger().Warnf("Failed to delete the %s hook, due to: %s", helmPrePhase, err)
	}

	err = h.runHooks(helmHooks, helmPostPhase, ns, chart, releaseName, wait, create, force)
	if err != nil {
		err2 := h.deleteHooks(helmHooks, helmPostPhase, hookFailed, ns)
		return util.CombineErrors(err, err2)
	}

	err = h.deleteHooks(helmHooks, helmPostPhase, hookSucceeded, ns)
	err2 := h.deleteOldResources(ns, releaseName, versionText, wait)

	return util.CombineErrors(err, err2)
}

func (h *HelmTemplate) DecryptSecrets(location string) error {
	return h.Client.DecryptSecrets(location)
}

func (h *HelmTemplate) kubectlApply(ns string, releaseName string, wait bool, create bool, force bool, dir string) error {
	namespacesDir := filepath.Join(dir, "namespaces")
	if _, err := os.Stat(namespacesDir); !os.IsNotExist(err) {

		fileInfo, err := ioutil.ReadDir(namespacesDir)
		if err != nil {
			return errors.Wrapf(err, "unable to locate subdirs in %s", namespacesDir)
		}

		for _, path := range fileInfo {
			namespace := filepath.Base(path.Name())
			fullPath := filepath.Join(namespacesDir, path.Name())

			log.Logger().Debugf("Applying generated chart %q YAML via kubectl in dir: %s to namespace %s", releaseName, fullPath, namespace)

			command := "apply"
			if create {
				command = "create"
			}
			args := []string{command, "--recursive", "-f", fullPath, "-l", LabelReleaseName + "=" + releaseName}
			applyNs := namespace
			if applyNs == "" {
				applyNs = ns
			}
			if applyNs != "" {
				args = append(args, "--namespace", applyNs)
			}
			if wait && !create {
				args = append(args, "--wait")
			}
			if !h.KubectlValidate {
				args = append(args, "--validate=false")
			}
			err = h.runKubectl(args...)
			if err != nil {
				return err
			}
			log.Logger().Info("")
		}
		return err
	}

	log.Logger().Debugf("Applying generated chart %q YAML via kubectl in dir: %s to namespace %s", releaseName, dir, ns)
	command := "apply"
	if create {
		command = "create"
	}
	args := []string{command, "--recursive", "-f", dir, "-l", LabelReleaseName + "=" + releaseName}
	if ns != "" {
		args = append(args, "--namespace", ns)
	}
	if wait && !create {
		args = append(args, "--wait")
	}
	if force {
		args = append(args, "--force")
	}
	if !h.KubectlValidate {
		args = append(args, "--validate=false")
	}
	err := h.runKubectl(args...)
	if err != nil {
		return err
	}

	return nil

}

func (h *HelmTemplate) kubectlApplyFile(ns string, helmHook string, wait bool, create bool, force bool, file string) error {
	log.Logger().Debugf("Applying Helm hook %s YAML via kubectl in file: %s", helmHook, file)

	command := "apply"
	if create {
		command = "create"
	}
	args := []string{command, "-f", file}
	if ns != "" {
		args = append(args, "--namespace", ns)
	}
	if wait && !create {
		args = append(args, "--wait")
	}
	if force {
		args = append(args, "--force")
	}
	if !h.KubectlValidate {
		args = append(args, "--validate=false")
	}
	err := h.runKubectl(args...)
	return err
}

func (h *HelmTemplate) kubectlDeleteFile(ns string, file string) error {
	log.Logger().Debugf("Deleting helm hook sources from file: %s", file)
	return h.runKubectl("delete", "-f", file, "--namespace", ns, "--wait")
}

func (h *HelmTemplate) deleteOldResources(ns string, releaseName string, versionText string, wait bool) error {
	selector := LabelReleaseName + "=" + releaseName + "," + LabelReleaseChartVersion + "!=" + versionText
	return h.deleteResourcesAndClusterResourcesBySelector(ns, selector, wait, "older releases")
}

func (h *HelmTemplate) deleteResourcesAndClusterResourcesBySelector(ns string, selector string, wait bool, message string) error {
	kinds := []string{"all", "pvc", "configmap", "release", "sa", "role", "rolebinding", "secret"}
	clusterKinds := []string{"clusterrole", "clusterrolebinding"}

	errList := []error{}

	log.Logger().Debugf("Removing Kubernetes resources from %s using selector: %s from %s", message, util.ColorInfo(selector), strings.Join(kinds, " "))
	errs := h.deleteResourcesBySelector(ns, kinds, selector, wait)
	errList = append(errList, errs...)

	selector += "," + LabelNamespace + "=" + ns
	log.Logger().Debugf("Removing Kubernetes resources from %s using selector: %s from %s", message, util.ColorInfo(selector), strings.Join(clusterKinds, " "))
	errs = h.deleteResourcesBySelector("", clusterKinds, selector, wait)
	errList = append(errList, errs...)
	return util.CombineErrors(errList...)
}

func (h *HelmTemplate) deleteResourcesBySelector(ns string, kinds []string, selector string, wait bool) []error {
	errList := []error{}
	for _, kind := range kinds {
		args := []string{"delete", kind, "--ignore-not-found", "-l", selector}
		if ns != "" {
			args = append(args, "--namespace", ns)
		}
		if wait {
			args = append(args, "--wait")
		}
		output, err := h.runKubectlWithOutput(args...)
		if err != nil {
			errList = append(errList, err)
		} else {
			output = strings.TrimSpace(output)
			if output != "No resources found" {
				log.Logger().Info(output)
			}
		}
	}
	return errList
}

// isClusterKind returns true if the kind or resource name is a cluster wide resource
func isClusterKind(kind string) bool {
	lower := strings.ToLower(kind)
	return strings.HasPrefix(lower, "cluster") || strings.HasPrefix(lower, "namespace")
}

// DeleteRelease removes the given release
func (h *HelmTemplate) DeleteRelease(ns string, releaseName string, purge bool) error {
	if ns == "" {
		ns = h.Namespace
	}
	selector := LabelReleaseName + "=" + releaseName
	return h.deleteResourcesAndClusterResourcesBySelector(ns, selector, true, fmt.Sprintf("release %s", releaseName))
}

// StatusRelease returns the output of the helm status command for a given release
func (h *HelmTemplate) StatusRelease(ns string, releaseName string) error {
	releases, _, err := h.ListReleases(ns)
	if err != nil {
		return errors.Wrap(err, "listing current chart releases")
	}
	if _, ok := releases[releaseName]; ok {
		return nil
	}
	return fmt.Errorf("chart release %q not found", releaseName)
}

// StatusReleaseWithOutput returns the output of the helm status command for a given release
func (h *HelmTemplate) StatusReleaseWithOutput(ns string, releaseName string, outputFormat string) (string, error) {
	return h.Client.StatusReleaseWithOutput(ns, releaseName, outputFormat)
}

func (h *HelmTemplate) getDirectories(releaseName string) (string, string, string, error) {
	if releaseName == "" {
		return "", "", "", fmt.Errorf("No release name specified!")
	}
	if h.WorkDir == "" {
		var err error
		h.WorkDir, err = ioutil.TempDir("", "helm-template-workdir-")
		if err != nil {
			return "", "", "", errors.Wrap(err, "Failed to create temporary directory for helm template workdir")
		}
	}
	workDir := h.WorkDir
	outDir := filepath.Join(workDir, releaseName, "output")
	helmHookDir := filepath.Join(workDir, releaseName, "helmHooks")
	chartsDir := filepath.Join(workDir, releaseName, "chartFiles")

	dirs := []string{outDir, helmHookDir, chartsDir}
	for _, d := range dirs {
		err := os.MkdirAll(d, util.DefaultWritePermissions)
		if err != nil {
			return "", "", "", err
		}
	}
	return outDir, helmHookDir, chartsDir, nil
}

// clearOutputDir removes all files in the helm output dir
func (h *HelmTemplate) clearOutputDir(releaseName string) error {
	dir, helmDir, chartsDir, err := h.getDirectories(releaseName)
	if err != nil {
		return err
	}
	return util.RecreateDirs(dir, helmDir, chartsDir)
}

func (h *HelmTemplate) fetchChart(chart string, version string, dir string, repo string, username string,
	password string) (string, error) {
	exists, err := util.FileExists(chart)
	if err != nil {
		return "", err
	}
	if exists {
		log.Logger().Infof("Chart dir already exists: %s", dir)
		return chart, nil
	}
	if dir == "" {
		return "", fmt.Errorf("must specify dir for chart %s", chart)
	}
	args := []string{
		"fetch", "-d", dir, "--untar", chart,
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	if version != "" {
		args = append(args, "--version", version)
	}
	if username != "" {
		args = append(args, "--username", username)
	}
	if password != "" {
		args = append(args, "--password", password)
	}
	err = h.Client.runHelm(args...)
	if err != nil {
		return "", err
	}
	answer := dir
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, f := range files {
		if f.IsDir() {
			answer = filepath.Join(dir, f.Name())
			break
		}
	}
	log.Logger().Debugf("Fetched chart %s to dir %s", chart, answer)
	return answer, nil
}

func (h *HelmTemplate) addLabelsToFiles(chart string, releaseName string, version string, metadata *chart.Metadata, ns string) ([]*HelmHook, error) {
	dir, helmHookDir, _, err := h.getDirectories(releaseName)
	if err != nil {
		return nil, err
	}
	return addLabelsToChartYaml(dir, helmHookDir, chart, releaseName, version, metadata, ns)
}

func splitObjectsInFiles(inputFile string, baseDir string, relativePath, defaultNamespace string) ([]string, error) {
	result := make([]string, 0)
	f, err := os.Open(inputFile)
	if err != nil {
		return result, errors.Wrapf(err, "opening inputFile %q", inputFile)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var buf bytes.Buffer
	fileName := filepath.Base(inputFile)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == resourcesSeparator {
			// ensure that we actually have YAML in the buffer
			data := buf.Bytes()
			if isWhitespaceOrComments(data) {
				buf.Reset()
				continue
			}

			m := yaml.MapSlice{}
			err = yaml.Unmarshal(data, &m)

			namespace := getYamlValueString(&m, "metadata", "namespace")
			if namespace == "" {
				namespace = defaultNamespace
			}

			if err != nil {
				return make([]string, 0), errors.Wrapf(err, "Failed to parse the following YAML from inputFile '%s':\n%s", inputFile, buf.String())
			}
			if len(m) == 0 {
				buf.Reset()
				continue
			}

			partFile, err := writeObjectInFile(&buf, baseDir, relativePath, namespace, fileName, count)
			if err != nil {
				return result, errors.Wrapf(err, "saving object")
			}
			result = append(result, partFile)
			buf.Reset()
			count += count + 1
		} else {
			_, err := buf.WriteString(line)
			if err != nil {
				return result, errors.Wrapf(err, "writing line from inputFile %q into a buffer", inputFile)
			}
			_, err = buf.WriteString("\n")
			if err != nil {
				return result, errors.Wrapf(err, "writing a new line in the buffer")
			}
		}
	}
	if buf.Len() > 0 && !isWhitespaceOrComments(buf.Bytes()) {
		data := buf.Bytes()

		m := yaml.MapSlice{}
		err = yaml.Unmarshal(data, &m)

		namespace := getYamlValueString(&m, "metadata", "namespace")
		if namespace == "" {
			namespace = defaultNamespace
		}

		partFile, err := writeObjectInFile(&buf, baseDir, relativePath, namespace, fileName, count)
		if err != nil {
			return result, errors.Wrapf(err, "saving object")
		}
		result = append(result, partFile)
	}

	return result, nil
}

// isWhitespaceOrComments returns true if the data is empty, whitespace or comments only
func isWhitespaceOrComments(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "#") {
			return false
		}
	}
	return true
}

func writeObjectInFile(buf io.WriterTo, baseDir string, relativePath, namespace string, fileName string, count int) (string, error) {
	relativeDir := filepath.Dir(relativePath)

	const filePrefix = "part"
	partFile := fmt.Sprintf("%s%d-%s", filePrefix, count, fileName)
	absFile := filepath.Join(baseDir, "namespaces", namespace, relativeDir, partFile)

	absFileDir := filepath.Dir(absFile)

	log.Logger().Debugf("creating file: %s", absFile)

	err := os.MkdirAll(absFileDir, os.ModePerm)
	if err != nil {
		return "", errors.Wrapf(err, "creating directory %q", absFileDir)
	}
	file, err := os.Create(absFile)
	if err != nil {
		return "", errors.Wrapf(err, "creating file %q", absFile)
	}

	log.Logger().Debugf("writing data to %s", absFile)

	defer file.Close()
	_, err = buf.WriteTo(file)
	if err != nil {
		return "", errors.Wrapf(err, "writing object to file %q", absFile)
	}
	return absFile, nil
}

func addLabelsToChartYaml(basedir string, hooksDir string, chart string, releaseName string, version string, metadata *chart.Metadata, ns string) ([]*HelmHook, error) {
	helmHooks := []*HelmHook{}

	log.Logger().Debugf("Searching for yaml files from basedir %s", basedir)

	err := filepath.Walk(basedir, func(path string, f os.FileInfo, err error) error {
		ext := filepath.Ext(path)
		if ext == ".yaml" {
			file := path

			relativePath, err := filepath.Rel(basedir, file)
			if err != nil {
				return errors.Wrapf(err, "unable to determine relative path %q", file)
			}

			partFiles, err := splitObjectsInFiles(file, basedir, relativePath, ns)
			if err != nil {
				return errors.Wrapf(err, "splitting objects from file %q", file)
			}

			log.Logger().Debugf("part files list: %v", partFiles)

			for _, partFile := range partFiles {
				log.Logger().Debugf("processing part file: %s", partFile)
				data, err := ioutil.ReadFile(partFile)
				if err != nil {
					return errors.Wrapf(err, "Failed to load partFile %s", partFile)
				}
				m := yaml.MapSlice{}
				err = yaml.Unmarshal(data, &m)
				if err != nil {
					return errors.Wrapf(err, "Failed to parse YAML of partFile %s", partFile)
				}
				kind := getYamlValueString(&m, "kind")
				helmHookType := getYamlValueString(&m, "metadata", "annotations", "helm.sh/hook")
				if helmHookType != "" {
					helmHook, err := getHelmHookFromFile(basedir, path, hooksDir, helmHookType, kind, &m, partFile)
					if err != nil {
						return errors.Wrap(err, fmt.Sprintf("when getting helm hook from part file '%s'", partFile))
					}
					helmHooks = append(helmHooks, helmHook)
				} else {
					err := processChartResource(partFile, data, kind, ns, releaseName, &m, metadata, version, chart)
					if err != nil {
						return errors.Wrap(err, fmt.Sprintf("when processing chart resource '%s'", partFile))
					}

				}
			}
		}
		return nil
	})

	return helmHooks, err
}

func getHelmHookFromFile(basedir string, path string, hooksDir string, helmHook string, kind string, m *yaml.MapSlice, partFile string) (*HelmHook, error) {
	// lets move any helm hooks to the new partFile
	relPath, err := filepath.Rel(basedir, path)
	if err != nil {
		return &HelmHook{}, err
	}
	if relPath == "" {
		return &HelmHook{}, fmt.Errorf("Failed to find relative path of basedir %s and path %s", basedir, partFile)
	}

	// add the hook type into the directory structure
	newPath := filepath.Join(hooksDir, relPath)
	newDir, _ := filepath.Split(newPath)
	err = os.MkdirAll(newDir, util.DefaultWritePermissions)
	if err != nil {
		return &HelmHook{}, errors.Wrap(err, fmt.Sprintf("when creating '%s'", newDir))
	}

	// copy the hook part file to the hooks path
	_, hookFileName := filepath.Split(partFile)
	hookFile := filepath.Join(newDir, hookFileName)
	err = os.Rename(partFile, hookFile)
	if err != nil {
		return &HelmHook{}, errors.Wrap(err, fmt.Sprintf("when copying from '%s' to '%s'", partFile, hookFile))
	}

	name := getYamlValueString(m, "metadata", "name")
	helmDeletePolicy := getYamlValueString(m, "metadata", "annotations", "helm.sh/hook-delete-policy")
	return NewHelmHook(kind, name, hookFile, helmHook, helmDeletePolicy), nil
}

func processChartResource(partFile string, data []byte, kind string, ns string, releaseName string, m *yaml.MapSlice, metadata *chart.Metadata, version string, chart string) error {
	err := setYamlValue(m, releaseName, "metadata", "labels", LabelReleaseName)
	if err != nil {
		return errors.Wrapf(err, "Failed to modify YAML of partFile %s", partFile)
	}
	if !isClusterKind(kind) {
		err = setYamlValue(m, ns, "metadata", "labels", LabelNamespace)
		if err != nil {
			return errors.Wrapf(err, "Failed to modify YAML of partFile %s", partFile)
		}
	}
	err = setYamlValue(m, version, "metadata", "labels", LabelReleaseChartVersion)
	if err != nil {
		return errors.Wrapf(err, "Failed to modify YAML of partFile %s", partFile)
	}
	chartName := ""

	if metadata != nil {
		chartName = metadata.GetName()
		appVersion := metadata.GetAppVersion()
		if appVersion != "" {
			err = setYamlValue(m, appVersion, "metadata", "annotations", AnnotationAppVersion)
			if err != nil {
				return errors.Wrapf(err, "Failed to modify YAML of partFile %s", partFile)
			}
		}
	}
	if chartName == "" {
		chartName = chart
	}
	err = setYamlValue(m, chartName, "metadata", "annotations", AnnotationChartName)
	if err != nil {
		return errors.Wrapf(err, "Failed to modify YAML of partFile %s", partFile)
	}

	data, err = yaml.Marshal(m)
	if err != nil {
		return errors.Wrapf(err, "Failed to marshal YAML of partFile %s", partFile)
	}
	err = ioutil.WriteFile(partFile, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "Failed to write YAML partFile %s", partFile)
	}
	return nil
}

func getYamlValueString(mapSlice *yaml.MapSlice, keys ...string) string {
	value := getYamlValue(mapSlice, keys...)
	answer, ok := value.(string)
	if ok {

		return answer
	}
	return ""
}

func getYamlValue(mapSlice *yaml.MapSlice, keys ...string) interface{} {
	if mapSlice == nil {
		return nil
	}
	if mapSlice == nil {
		return fmt.Errorf("No map input!")
	}
	m := mapSlice
	lastIdx := len(keys) - 1
	for idx, k := range keys {
		last := idx >= lastIdx
		found := false
		for _, mi := range *m {
			if mi.Key == k {
				found = true
				if last {
					return mi.Value
				} else {
					value := mi.Value
					if value == nil {
						return nil
					} else {
						v, ok := value.(yaml.MapSlice)
						if ok {
							m = &v
						} else {
							v2, ok := value.(*yaml.MapSlice)
							if ok {
								m = v2
							} else {
								return nil
							}
						}
					}
				}
			}
		}
		if !found {
			return nil
		}
	}
	return nil

}

// setYamlValue navigates through the YAML object structure lazily creating or inserting new values
func setYamlValue(mapSlice *yaml.MapSlice, value string, keys ...string) error {
	if mapSlice == nil {
		return fmt.Errorf("No map input!")
	}
	m := mapSlice
	lastIdx := len(keys) - 1
	for idx, k := range keys {
		last := idx >= lastIdx
		found := false
		for i, mi := range *m {
			if mi.Key == k {
				found = true
				if last {
					(*m)[i].Value = value
				} else if i < len(*m) {
					value := (*m)[i].Value
					if value == nil {
						v := &yaml.MapSlice{}
						(*m)[i].Value = v
						m = v
					} else {
						v, ok := value.(yaml.MapSlice)
						if ok {
							m2 := &yaml.MapSlice{}
							*m2 = append(*m2, v...)
							(*m)[i].Value = m2
							m = m2
						} else {
							v2, ok := value.(*yaml.MapSlice)
							if ok {
								m2 := &yaml.MapSlice{}
								*m2 = append(*m2, *v2...)
								(*m)[i].Value = m2
								m = m2
							} else {
								return fmt.Errorf("Could not convert key %s value %#v to a yaml.MapSlice", k, value)
							}
						}
					}
				}
			}
		}
		if !found {
			if last {
				*m = append(*m, yaml.MapItem{
					Key:   k,
					Value: value,
				})
			} else {
				m2 := &yaml.MapSlice{}
				*m = append(*m, yaml.MapItem{
					Key:   k,
					Value: m2,
				})
				m = m2
			}
		}
	}
	return nil
}

func (h *HelmTemplate) runKubectl(args ...string) error {
	h.Runner.SetDir(h.CWD)
	h.Runner.SetName(h.Binary)
	h.Runner.SetArgs(args)
	output, err := h.Runner.RunWithoutRetry()
	log.Logger().Debugf(output)
	return err
}

func (h *HelmTemplate) runKubectlWithOutput(args ...string) (string, error) {
	h.Runner.SetDir(h.CWD)
	h.Runner.SetName(h.Binary)
	h.Runner.SetArgs(args)
	return h.Runner.RunWithoutRetry()
}

// getChartNameAndVersion returns the chart name and version for the current chart folder
func (h *HelmTemplate) getChartNameAndVersion(chartDir string, version *string) (string, string, error) {
	versionText := ""
	if version != nil && *version != "" {
		versionText = *version
	}
	file := filepath.Join(chartDir, ChartFileName)
	if !filepath.IsAbs(chartDir) {
		file = filepath.Join(h.Runner.CurrentDir(), file)
	}
	exists, err := util.FileExists(file)
	if err != nil {
		return "", versionText, err
	}
	if !exists {
		return "", versionText, fmt.Errorf("No file %s found!", file)
	}
	chartName, versionText, err := LoadChartNameAndVersion(file)
	return chartName, versionText, err
}

// getChart returns the chart metadata for the given dir
func (h *HelmTemplate) getChart(chartDir string, version string) (*chart.Metadata, string, error) {
	file := filepath.Join(chartDir, ChartFileName)
	if !filepath.IsAbs(chartDir) {
		file = filepath.Join(h.Runner.CurrentDir(), file)
	}
	exists, err := util.FileExists(file)
	if err != nil {
		return nil, version, err
	}
	if !exists {
		return nil, version, fmt.Errorf("no file %s found!", file)
	}
	metadata, err := chartutil.LoadChartfile(file)
	if version == "" && metadata != nil {
		version = metadata.GetVersion()
	}
	return metadata, version, err
}

func (h *HelmTemplate) runHooks(hooks []*HelmHook, hookPhase string, ns string, chart string, releaseName string, wait bool, create bool, force bool) error {
	matchingHooks := MatchingHooks(hooks, hookPhase, "")
	for _, hook := range matchingHooks {
		err := h.kubectlApplyFile(ns, hookPhase, wait, create, force, hook.File)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *HelmTemplate) deleteHooks(hooks []*HelmHook, hookPhase string, hookDeletePolicy string, ns string) error {
	flag := os.Getenv("JX_DISABLE_DELETE_HELM_HOOKS")
	matchingHooks := MatchingHooks(hooks, hookPhase, hookDeletePolicy)
	for _, hook := range matchingHooks {
		kind := hook.Kind
		name := hook.Name
		if kind == "Job" && name != "" {
			log.Logger().Debugf("Waiting for helm %s hook Job %s to complete before removing it", hookPhase, name)
			err := kube.WaitForJobToComplete(h.KubeClient, ns, name, time.Minute*30, false)
			if err != nil {
				log.Logger().Warnf("Job %s has not yet terminated for helm hook phase %s due to: %s so removing it anyway", name, hookPhase, err)
			}
		} else {
			log.Logger().Warnf("Could not wait for hook resource to complete as it is kind %s and name %s for phase %s", kind, name, hookPhase)
		}
		if flag == "true" {
			log.Logger().Infof("Not deleting the Job %s as we have the $JX_DISABLE_DELETE_HELM_HOOKS enabled", name)
			continue
		}
		err := h.kubectlDeleteFile(ns, hook.File)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewHelmHook returns a newly created HelmHook
func NewHelmHook(kind string, name string, file string, hook string, hookDeletePolicy string) *HelmHook {
	return &HelmHook{
		Kind:               kind,
		Name:               name,
		File:               file,
		Hooks:              strings.Split(hook, ","),
		HookDeletePolicies: strings.Split(hookDeletePolicy, ","),
	}
}

// MatchingHooks returns the matching files which have the given hook name and if hookPolicy is not blank the hook policy too
func MatchingHooks(hooks []*HelmHook, hook string, hookDeletePolicy string) []*HelmHook {
	answer := []*HelmHook{}
	for _, h := range hooks {
		if util.StringArrayIndex(h.Hooks, hook) >= 0 &&
			(hookDeletePolicy == "" || util.StringArrayIndex(h.HookDeletePolicies, hookDeletePolicy) >= 0) {
			answer = append(answer, h)
		}
	}
	return answer
}
