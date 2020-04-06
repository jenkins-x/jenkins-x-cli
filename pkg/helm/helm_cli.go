package helm

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/kube"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/slice"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// HelmCLI implements common helm actions based on helm CLI
type HelmCLI struct {
	Binary     string
	BinVersion Version
	CWD        string
	Runner     util.Commander
	Debug      bool
	kuber      kube.Kuber
}

// NewHelmCLIWithRunner creates a new HelmCLI interface for the given runner
func NewHelmCLIWithRunner(runner util.Commander, binary string, version Version, cwd string, debug bool, kuber kube.Kuber) *HelmCLI {
	cli := &HelmCLI{
		Binary:     binary,
		BinVersion: version,
		CWD:        cwd,
		Runner:     runner,
		Debug:      debug,
		kuber:      kuber,
	}
	return cli
}

// NewHelmCLI creates a new HelmCLI instance configured to use the provided helm CLI in
// the given current working directory
func NewHelmCLI(binary string, version Version, cwd string, debug bool, args ...string) *HelmCLI {
	a := []string{}
	for _, x := range args {
		y := strings.Split(x, " ")
		a = append(a, y...)
	}
	runner := &util.Command{
		Args: a,
		Name: binary,
		Dir:  cwd,
	}
	cli := &HelmCLI{
		Binary:     binary,
		BinVersion: version,
		CWD:        cwd,
		Runner:     runner,
		Debug:      debug,
	}
	return cli
}

// NewHelmCLIWithCompatibilityCheck creates a new HelmCLI and checks the compatibility with
// the currently installed helm CLI. This will exit the program with a fatal log if the helm CLI
// is not compatible.
func NewHelmCLIWithCompatibilityCheck(binary string, version Version, cwd string, debug bool, args ...string) *HelmCLI {
	cli := NewHelmCLI(binary, version, cwd, debug, args...)
	cli.checkCompatibility()
	return cli
}

// checkCompatibility verifies whether the current helm CLI is compatible. The current
// implementation only supports helm CLI v2. This function will exit the program if the
// installed helm cli is not compatible.
func (h *HelmCLI) checkCompatibility() {
	version, err := h.VersionWithArgs(false, "--client")
	version = strings.TrimPrefix(version, "Client: ")
	if err != nil {
		log.Logger().Warnf("Unable to detect the current helm version due to: %s", err)
		return
	}
	v, err := semver.ParseTolerant(version)
	if err != nil {
		log.Logger().Warnf("Unable to parse the current helm version: %s", version)
		return
	}

	if h.BinVersion == V3 {
		if v.Major != 3 {
			log.Logger().Fatalf("Your current helm version v%d is not supported. Please upgrade to helm v3.", v.Major)
		}
	} else if os.Getenv("JX_HELM3") == "true" {
		if v.Major != 3 {
			log.Logger().Fatalf("You have set $JX_HELM3=true but your helm client version is %d", v.Major)
		}
	} else {
		if v.Major > 2 {
			log.Logger().Fatalf("Your current helm version v%d is not supported. Please downgrade to helm v2.", v.Major)
		}
	}

}

// SetHost is used to point at a locally running tiller
func (h *HelmCLI) SetHost(tillerAddress string) {
	if h.Debug {
		log.Logger().Infof("Setting tiller address to %s", util.ColorInfo(tillerAddress))
	}
	h.Runner.SetEnvVariable("HELM_HOST", tillerAddress)
}

// SetCWD configures the common working directory of helm CLI
func (h *HelmCLI) SetCWD(dir string) {
	h.CWD = dir
}

// HelmBinary return the configured helm CLI
func (h *HelmCLI) HelmBinary() string {
	return h.Binary
}

// SetHelmBinary configure a new helm CLI
func (h *HelmCLI) SetHelmBinary(binary string) {
	h.Binary = binary
}

func (h *HelmCLI) runHelm(args ...string) error {
	h.Runner.SetDir(h.CWD)
	h.Runner.SetName(h.Binary)
	h.Runner.SetArgs(args)
	_, err := h.Runner.RunWithoutRetry()
	return err
}

func (h *HelmCLI) runHelmWithOutput(args ...string) (string, error) {
	h.Runner.SetDir(h.CWD)
	h.Runner.SetName(h.Binary)
	h.Runner.SetArgs(args)
	return h.Runner.RunWithoutRetry()
}

// Init executes the helm init command according with the given flags
func (h *HelmCLI) Init(clientOnly bool, serviceAccount string, tillerNamespace string, upgrade bool) error {
	args := []string{}
	args = append(args, "init")
	if clientOnly {
		args = append(args, "--client-only")
	}
	if serviceAccount != "" {
		args = append(args, "--service-account", serviceAccount)
	}
	if tillerNamespace != "" {
		args = append(args, "--tiller-namespace", tillerNamespace)
	}
	if upgrade {
		args = append(args, "--upgrade", "--wait", "--force-upgrade")
	}

	if h.Debug {
		log.Logger().Debugf("Initialising Helm '%s'", util.ColorInfo(strings.Join(args, " ")))
	}

	return h.runHelm(args...)
}

// AddRepo adds a new helm repo with the given name and URL
func (h *HelmCLI) AddRepo(repo, URL, username, password string) error {
	args := []string{"repo", "add", repo, URL}
	if username != "" {
		args = append(args, "--username", username)
	}
	if password != "" {
		args = append(args, "--password", password)
	}
	return h.runHelm(args...)
}

// RemoveRepo removes the given repo from helm
func (h *HelmCLI) RemoveRepo(repo string) error {
	return h.runHelm("repo", "remove", repo)
}

// ListRepos list the installed helm repos together with their URL
func (h *HelmCLI) ListRepos() (map[string]string, error) {
	output, err := h.runHelmWithOutput("repo", "list")
	repos := map[string]string{}
	if err != nil {
		// helm3 now returns an error if there are no repos
		return repos, nil
		//return nil, shouldError.Wrap(err, "failed to list repositories")
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) > 1 {
			repos[strings.TrimSpace(fields[0])] = fields[1]
		} else if len(fields) > 0 {
			repos[fields[0]] = ""
		}
	}
	return repos, nil
}

// SearchCharts searches for all the charts matching the given filter
func (h *HelmCLI) SearchCharts(filter string, allVersions bool) ([]ChartSummary, error) {
	answer := []ChartSummary{}
	args := []string{"search", filter}
	if allVersions {
		args = append(args, "--versions")
	}
	output, err := h.runHelmWithOutput(args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to search charts")
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "NAME") || line == "" {
			continue
		}
		line = strings.TrimSpace(line)
		fields := strings.Split(line, "\t")
		chart := ChartSummary{}
		l := len(fields)
		if l == 0 {
			continue
		}
		chart.Name = strings.TrimSpace(fields[0])
		if l > 1 {
			chart.ChartVersion = strings.TrimSpace(fields[1])
		}
		if l > 2 {
			chart.AppVersion = strings.TrimSpace(fields[2])
		}
		if l > 3 {
			chart.Description = strings.TrimSpace(fields[3])
		}
		answer = append(answer, chart)
	}
	return answer, nil
}

// IsRepoMissing checks if the repository with the given URL is missing from helm.
// If the repo is found, the name of the repo will be returned
func (h *HelmCLI) IsRepoMissing(URL string) (bool, string, error) {
	repos, err := h.ListRepos()
	if err != nil {
		return true, "", errors.Wrap(err, "failed to list the repositories")
	}
	searchedURL, err := url.Parse(URL)
	if err != nil {
		return true, "", errors.Wrap(err, "provided repo URL is invalid")
	}
	for name, repoURL := range repos {
		if len(repoURL) > 0 {
			url, err := url.Parse(repoURL)
			if err != nil {
				return true, "", errors.Wrap(err, "failed to parse the repo URL")
			}
			// match on the whole URL as helm dep build requires on username + passowrd in the URL
			if url.Host == searchedURL.Host && url.Path == searchedURL.Path {
				return false, name, nil
			}
		}
	}
	return true, "", nil
}

// UpdateRepo updates the helm repositories
func (h *HelmCLI) UpdateRepo() error {
	return h.runHelm("repo", "update")
}

// RemoveRequirementsLock removes the requirements.lock file from the current working directory
func (h *HelmCLI) RemoveRequirementsLock() error {
	dir := h.CWD
	path := filepath.Join(dir, "requirements.lock")
	exists, err := util.FileExists(path)
	if err != nil {
		return errors.Wrapf(err, "no requirements.lock file found in directory '%s'", dir)
	}
	if exists {
		err = os.Remove(path)
		if err != nil {
			return errors.Wrap(err, "failed to remove the requirements.lock file")
		}
	}
	return nil
}

// BuildDependency builds the helm dependencies of the helm chart from the current working directory
func (h *HelmCLI) BuildDependency() error {
	if h.Debug {
		log.Logger().Infof("Running %s dependency build in %s\n", h.Binary, util.ColorInfo(h.CWD))
		out, err := h.runHelmWithOutput("dependency", "build")
		log.Logger().Infof(out)
		return err
	}
	return h.runHelm("dependency", "build")
}

// InstallChart installs a helm chart according with the given flags
func (h *HelmCLI) InstallChart(chart string, releaseName string, ns string, version string, timeout int,
	values []string, valueStrings []string, valueFiles []string, repo string, username string, password string) error {
	var err error

	args := []string{}
	args = append(args, "install", "--wait", "--name", releaseName, "--namespace", ns, chart)
	repo, err = addUsernamePasswordToURL(repo, username, password)
	if err != nil {
		return err
	}

	if timeout != -1 {
		if h.Binary == "helm3" {
			args = append(args, "--timeout", fmt.Sprintf("%ss", strconv.Itoa(timeout)))
		} else {
			args = append(args, "--timeout", strconv.Itoa(timeout))
		}
	}
	if version != "" {
		args = append(args, "--version", version)
	}
	for _, value := range values {
		args = append(args, "--set", value)
	}
	for _, value := range valueStrings {
		args = append(args, "--set-string", value)
	}
	for _, valueFile := range valueFiles {
		args = append(args, "--values", valueFile)
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	if username != "" {
		args = append(args, "--username", username)
	}
	if password != "" {
		args = append(args, "--password", password)
	}
	logLevel := os.Getenv("JX_HELM_VERBOSE")
	if logLevel != "" {
		args = append(args, "-v", logLevel)
	}
	if h.Debug {
		log.Logger().Infof("Installing Chart '%s'", util.ColorInfo(strings.Join(args, " ")))
	}

	err = h.runHelm(args...)
	if err != nil {
		return err
	}

	return nil
}

// FetchChart fetches a Helm Chart
func (h *HelmCLI) FetchChart(chart string, version string, untar bool, untardir string, repo string,
	username string, password string) error {
	args := []string{}
	args = append(args, "fetch", chart)
	repo, err := addUsernamePasswordToURL(repo, username, password)
	if err != nil {
		return err
	}

	if untardir != "" {
		args = append(args, "--untardir", untardir)
	}
	if untar {
		args = append(args, "--untar")
	}

	if username != "" {
		args = append(args, "--username", username)
	}
	if password != "" {
		args = append(args, "--password", password)
	}

	if version != "" {
		args = append(args, "--version", version)
	}

	if repo != "" {
		args = append(args, "--repo", repo)
	}

	if h.Debug {
		log.Logger().Infof("Fetching Chart '%s'", util.ColorInfo(strings.Join(args, " ")))
	}

	return h.runHelm(args...)
}

// Template generates the YAML from the chart template to the given directory
func (h *HelmCLI) Template(chart string, releaseName string, ns string, outDir string, upgrade bool,
	values []string, valueStrings []string, valueFiles []string) error {
	args := []string{"template", "--name", releaseName, "--namespace", ns, chart, "--output-dir", outDir, "--debug"}
	if upgrade {
		args = append(args, "--is-upgrade")
	}
	for _, value := range values {
		args = append(args, "--set", value)
	}
	for _, value := range valueStrings {
		args = append(args, "--set-string", value)
	}
	for _, valueFile := range valueFiles {
		args = append(args, "--values", valueFile)
	}

	if h.Debug {
		log.Logger().Debugf("Generating Chart Template '%s'", util.ColorInfo(strings.Join(args, " ")))
	}
	err := h.runHelm(args...)
	if err != nil {
		return errors.Wrapf(err, "Failed to run helm %s", strings.Join(args, " "))
	}
	return err
}

// UpgradeChart upgrades a helm chart according with given helm flags
func (h *HelmCLI) UpgradeChart(chart string, releaseName string, ns string, version string, install bool, timeout int, force bool, wait bool, values []string, valueStrings []string, valueFiles []string, repo string, username string, password string) error {
	var err error
	args := []string{}
	args = append(args, "upgrade")
	args = append(args, "--namespace", ns)
	repo, err = addUsernamePasswordToURL(repo, username, password)
	if err != nil {
		return err
	}

	if install {
		args = append(args, "--install")
	}
	if wait {
		args = append(args, "--wait")
	}
	if force {
		args = append(args, "--force")
	}
	if timeout != -1 {
		if h.BinVersion == V3 {
			args = append(args, "--timeout", fmt.Sprintf("%ss", strconv.Itoa(timeout)))
		} else {
			args = append(args, "--timeout", strconv.Itoa(timeout))
		}
	}
	if version != "" {
		args = append(args, "--version", version)
	}
	for _, value := range values {
		args = append(args, "--set", value)
	}
	for _, value := range valueStrings {
		args = append(args, "--set-string", value)
	}
	for _, valueFile := range valueFiles {
		args = append(args, "--values", valueFile)
	}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	if username != "" {
		args = append(args, "--username", username)
	}
	if password != "" {
		args = append(args, "--password", password)
	}
	logLevel := os.Getenv("JX_HELM_VERBOSE")
	if logLevel != "" {
		args = append(args, "-v", logLevel)
	}
	args = append(args, releaseName, chart)

	if h.Debug {
		log.Logger().Infof("Upgrading Chart '%s'", util.ColorInfo(strings.Join(args, " ")))
	}

	err = h.runHelm(args...)
	if err != nil {
		return err
	}

	return nil
}

// DeleteRelease removes the given release
func (h *HelmCLI) DeleteRelease(ns string, releaseName string, purge bool) error {
	args := []string{}
	args = append(args, "delete")
	if purge {
		args = append(args, "--purge")
	}
	args = append(args, releaseName)
	return h.runHelm(args...)
}

//ListReleases lists the releases in ns
func (h *HelmCLI) ListReleases(ns string) (map[string]ReleaseSummary, []string, error) {
	output, err := h.runHelmWithOutput("list", "--all", "--namespace", ns)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "running helm list --all --namespace %s", ns)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := make(map[string]ReleaseSummary, 0)
	keys := make([]string, 0)
	if len(lines) > 1 {
		if h.Binary == "helm" {
			for _, line := range lines[1:] {
				fields := strings.Fields(line)
				if len(fields) == 10 || len(fields) == 11 {
					chartFullName := fields[8]
					lastDash := strings.LastIndex(chartFullName, "-")
					releaseName := fields[0]
					keys = append(keys, releaseName)
					result[releaseName] = ReleaseSummary{
						ReleaseName: fields[0],
						Revision:    fields[1],
						Updated: fmt.Sprintf("%s %s %s %s %s", fields[2], fields[3], fields[4], fields[5],
							fields[6]),
						Status:        fields[7],
						ChartFullName: chartFullName,
						Namespace:     ns,
						Chart:         chartFullName[:lastDash],
						ChartVersion:  chartFullName[lastDash+1:],
					}
				} else {
					return nil, nil, errors.Errorf("Cannot parse %s as helm list output", line)
				}
			}
		} else {
			for _, line := range lines[1:] {
				fields := strings.Fields(line)
				if len(fields) == 9 {
					chartFullName := fields[8]
					lastDash := strings.LastIndex(chartFullName, "-")
					releaseName := fields[0]
					keys = append(keys, releaseName)
					result[releaseName] = ReleaseSummary{
						ReleaseName:   fields[0],
						Revision:      fields[2],
						Updated:       fmt.Sprintf("%s %s %s %s", fields[3], fields[4], fields[5], fields[6]),
						Status:        strings.ToUpper(fields[7]),
						ChartFullName: chartFullName,
						Namespace:     ns,
						Chart:         chartFullName[:lastDash],
						ChartVersion:  chartFullName[lastDash+1:],
					}
				} else {
					return nil, nil, errors.Errorf("Cannot parse %s as helm3 list output", line)
				}
			}
		}
	}
	slice.SortStrings(keys)
	return result, keys, nil
}

// FindChart find a chart in the current working directory, if no chart file is found an error is returned
func (h *HelmCLI) FindChart() (string, error) {
	dir := h.CWD
	chartFile := filepath.Join(dir, ChartFileName)
	exists, err := util.FileExists(chartFile)
	if err != nil {
		return "", errors.Wrapf(err, "no Chart.yaml file found in directory '%s'", dir)
	}
	if !exists {
		files, err := filepath.Glob(filepath.Join(dir, "*", "Chart.yaml"))
		if err != nil {
			return "", errors.Wrap(err, "no Chart.yaml file found")
		}
		if len(files) > 0 {
			chartFile = files[0]
		} else {
			files, err = filepath.Glob(filepath.Join(dir, "*", "*", "Chart.yaml"))
			if err != nil {
				return "", errors.Wrap(err, "no Chart.yaml file found")
			}
			if len(files) > 0 {
				for _, file := range files {
					if !strings.HasSuffix(file, "/preview/Chart.yaml") {
						return file, nil
					}
				}
			}
		}
	}
	return chartFile, nil
}

// StatusRelease returns the output of the helm status command for a given release
func (h *HelmCLI) StatusRelease(ns string, releaseName string) error {
	return h.runHelm("status", releaseName)
}

// StatusReleaseWithOutput returns the output of the helm status command for a given release
func (h *HelmCLI) StatusReleaseWithOutput(ns string, releaseName string, outputFormat string) (string, error) {
	if outputFormat == "" {
		return h.runHelmWithOutput("status", releaseName)
	}
	return h.runHelmWithOutput("status", releaseName, "--output", outputFormat)
}

// Lint lints the helm chart from the current working directory and returns the warnings in the output
func (h *HelmCLI) Lint(valuesFiles []string) (string, error) {
	args := []string{"lint",
		"--set", "tags.jx-lint=true",
		"--set", "global.jxLint=true",
		"--set-string", "global.jxTypeEnv=lint",
	}
	for _, valueFile := range valuesFiles {
		if valueFile != "" {
			args = append(args, "--values", valueFile)
		}
	}
	return h.runHelmWithOutput(args...)
}

// Env returns the environment variables for the helmer
func (h *HelmCLI) Env() map[string]string {
	return h.Runner.CurrentEnv()
}

// Version executes the helm version command and returns its output
func (h *HelmCLI) Version(tls bool) (string, error) {
	versionString, err := h.VersionWithArgs(tls)
	if err != nil {
		return "", errors.Wrapf(err, "unable to query helm version")
	}

	return h.extractSemanticVersion(versionString)
}

// VersionWithArgs executes the helm version command and returns its output
func (h *HelmCLI) VersionWithArgs(tls bool, extraArgs ...string) (string, error) {
	args := []string{"version", "--short", "--client"}
	if tls {
		args = append(args, "--tls")
	}
	args = append(args, extraArgs...)
	return h.runHelmWithOutput(args...)
}

// PackageChart packages the chart from the current working directory
func (h *HelmCLI) PackageChart() error {
	return h.runHelm("package", h.CWD)
}

func (h *HelmCLI) DecryptSecrets(location string) error {
	return h.runHelm("secrets", "dec", location)
}

// Kube returns the k8s config client
func (h *HelmCLI) Kube() kube.Kuber {
	if h.kuber == nil {
		h.kuber = kube.NewKubeConfig()
	}
	return h.kuber
}

// SetKube  sets the kube config client
func (h *HelmCLI) SetKube(kuber kube.Kuber) {
	h.kuber = kuber
}

// Helm really prefers to have the username and password embedded in the URL (ugh) so this
// function makes that happen
func addUsernamePasswordToURL(urlStr string, username string, password string) (string, error) {
	if urlStr != "" && username != "" && password != "" {
		u, err := url.Parse(urlStr)
		if err != nil {
			return "", err
		}
		u.User = url.UserPassword(username, password)
		return u.String(), nil
	}
	return urlStr, nil
}

func (h *HelmCLI) getCurrentNamespace() (string, error) {
	config, _, err := h.Kube().LoadConfig()
	if err != nil {
		return "", errors.Wrap(err, "loading Kubernetes configuration")
	}
	currentNS := kube.CurrentNamespace(config)

	return currentNS, nil
}

func (h *HelmCLI) setNamespace(namespace string) error {
	config, pathOptions, err := h.Kube().LoadConfig()
	if err != nil {
		return errors.Wrap(err, "loading Kubernetes configuration")
	}

	newConfig := *config
	ctx := kube.CurrentContext(config)
	if ctx == nil {
		return fmt.Errorf("unable to get context")
	}
	if ctx.Namespace == namespace {
		return nil
	}
	ctx.Namespace = namespace
	err = clientcmd.ModifyConfig(pathOptions, newConfig, false)
	if err != nil {
		return fmt.Errorf("failed to update the kube config %s", err)
	}
	return nil
}

// extractSemanticVersion tries to extract a semantic version string substring from the specified string
func (h *HelmCLI) extractSemanticVersion(versionString string) (string, error) {
	r := regexp.MustCompile(`.*v?(?P<SemVer>\d+\.\d+\.\d+).*`)
	match := r.FindStringSubmatch(versionString)
	if match == nil {
		return "", errors.Errorf("unable to extract a semantic version from %s", versionString)
	}

	for i, name := range r.SubexpNames() {
		if name == "SemVer" {
			return match[i], nil
		}
	}

	return "", errors.Errorf("unable to extract a semantic version from %s", versionString)
}
