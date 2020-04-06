package prow

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/pkg/errors"

	"github.com/ghodss/yaml"
	prowconfig "github.com/jenkins-x/jx/pkg/prow/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	prowapi "k8s.io/test-infra/prow/apis/prowjobs/v1"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/plugins"
)

const (
	TektonAgent = "tekton"
)

const (
	ProwConfigMapName           = "config"
	ProwPluginsConfigMapName    = "plugins"
	ProwExternalPluginsFilename = "external-plugins.yaml"
	ProwConfigFilename          = "config.yaml"
	ProwPluginsFilename         = "plugins.yaml"
)

// Options for Prow
type Options struct {
	KubeClient           kubernetes.Interface
	Repos                []string
	NS                   string
	Kind                 prowconfig.Kind
	DraftPack            string
	EnvironmentNamespace string
	Context              string
	Agent                string
	IgnoreBranch         bool
	PluginsFileLocation  string
	ConfigFileLocation   string
}

type ExternalPlugins struct {
	Items []plugins.ExternalPlugin
}

func add(kubeClient kubernetes.Interface, repos []string, ns string, kind prowconfig.Kind, draftPack, environmentNamespace string, context string, teamSettings *v1.TeamSettings) error {
	if len(repos) == 0 {
		return fmt.Errorf("no repo defined")
	}
	agent := TektonAgent

	o := Options{
		KubeClient:           kubeClient,
		Repos:                repos,
		NS:                   ns,
		Kind:                 kind,
		DraftPack:            draftPack,
		EnvironmentNamespace: environmentNamespace,
		Context:              context,
		Agent:                agent,
	}
	if err := o.AddProwConfig(); err != nil {
		return errors.Wrap(err, "adding prow config")
	}
	if err := o.AddProwPlugins(); err != nil {
		return errors.Wrap(err, "adding prow plugins")
	}
	return nil
}

func remove(kubeClient kubernetes.Interface, repos []string, ns string, kind prowconfig.Kind) error {
	if len(repos) == 0 {
		return fmt.Errorf("no repo defined")
	}
	o := Options{
		KubeClient: kubeClient,
		Repos:      repos,
		NS:         ns,
		Kind:       kind,
	}
	return o.RemoveProwConfig()
}

// AddEnvironment adds an environment git repo config
func AddEnvironment(kubeClient kubernetes.Interface, repos []string, ns, environmentNamespace string, teamSettings *v1.TeamSettings, remoteEnvironment bool) error {
	kind := prowconfig.Environment
	if remoteEnvironment {
		kind = prowconfig.RemoteEnvironment
	}
	return add(kubeClient, repos, ns, kind, "", environmentNamespace, "", teamSettings)
}

// AddApplication adds an app git repo config
func AddApplication(kubeClient kubernetes.Interface, repos []string, ns, draftPack string, teamSettings *v1.TeamSettings) error {
	return add(kubeClient, repos, ns, prowconfig.Application, draftPack, "", "", teamSettings)
}

// DeleteApplication will delete the Prow configuration for a given set of repositories
func DeleteApplication(kubeClient kubernetes.Interface, repos []string, ns string) error {
	return remove(kubeClient, repos, ns, prowconfig.Application)
}

// AddProtection adds a protection entry in the prow config
func AddProtection(kubeClient kubernetes.Interface, repos []string, context string, ns string, teamSettings *v1.TeamSettings) error {
	return add(kubeClient, repos, ns, prowconfig.Protection, "", "", context, teamSettings)
}

// AddExternalPlugins adds one or more external plugins to the specified repos. If repos is nil,
// then the external plugins will be added to all repos that have plugins
func AddExternalPlugins(kubeClient kubernetes.Interface, repos []string, ns string,
	add ...plugins.ExternalPlugin) error {
	o := Options{
		KubeClient: kubeClient,
		NS:         ns,
	}
	if err := o.AddExternalProwPlugins(add); err != nil {
		return errors.Wrap(err, "add external prow plugins")
	}
	if err := o.AddProwPlugins(); err != nil {
		return errors.Wrap(err, "add prow plugins")
	}
	return nil
}

// create Git repo?
// get config and update / overwrite repos?
// should we get the existing CM and do a diff?
// should we just be using git for config and use Prow to auto update via gitops?

func (o *Options) createPreSubmitEnvironment() config.Presubmit {
	ps := config.Presubmit{}

	ps.Name = prowconfig.PromotionBuild
	ps.AlwaysRun = true
	ps.SkipReport = false
	ps.Context = prowconfig.PromotionBuild
	ps.Agent = o.Agent
	ps.RerunCommand = "/test this"
	ps.Trigger = "(?m)^/test( all| this),?(\\s+|$)"

	return ps
}

func (o *Options) createPostSubmitEnvironment() config.Postsubmit {
	ps := config.Postsubmit{}
	ps.Name = "promotion"
	ps.Agent = o.Agent
	ps.Branches = []string{"^master$"}

	return ps
}

func (o *Options) createPostSubmitApplication() config.Postsubmit {
	ps := config.Postsubmit{}
	ps.Branches = []string{"^master$"}
	ps.Name = "release"
	ps.Agent = o.Agent

	return ps
}

func (o *Options) createPreSubmitApplication() config.Presubmit {
	ps := config.Presubmit{}

	ps.Context = prowconfig.ServerlessJenkins
	ps.Name = prowconfig.ServerlessJenkins
	ps.RerunCommand = "/test this"
	ps.Trigger = "(?m)^/test( all| this),?(\\s+|$)"
	ps.AlwaysRun = true
	ps.SkipReport = false
	ps.Agent = o.Agent

	ps.RerunCommand = "/test this"
	ps.Trigger = "(?m)^/test( all| this),?(\\s+|$)"

	return ps
}

// AddProwConfig adds config to Prow
func (o *Options) AddProwConfig() error {
	var preSubmit config.Presubmit
	var postSubmit config.Postsubmit

	switch o.Kind {
	case prowconfig.Application:
		preSubmit = o.createPreSubmitApplication()
		postSubmit = o.createPostSubmitApplication()
	case prowconfig.Environment:
		preSubmit = o.createPreSubmitEnvironment()
		postSubmit = o.createPostSubmitEnvironment()
	case prowconfig.RemoteEnvironment:
		preSubmit = o.createPreSubmitEnvironment()
	case prowconfig.Protection:
		// Nothing needed
	default:
		return fmt.Errorf("unknown Prow config kind %s", o.Kind)
	}

	prowConfig, create, err := o.GetProwConfig()
	if err != nil {
		return errors.Wrap(err, "getting prow config")
	}

	prowConfig.PodNamespace = o.NS
	prowConfig.ProwJobNamespace = o.NS

	for _, r := range o.Repos {
		err = prowconfig.AddRepoToTideConfig(&prowConfig.Tide, r, o.Kind)
		if err != nil {
			return errors.Wrapf(err, "adding repo %q to tide config", r)
		}
		err = prowconfig.AddRepoToBranchProtection(&prowConfig.BranchProtection, r, o.Context, o.Kind)
		if err != nil {
			return errors.Wrapf(err, "adding repo %q to branch protection", r)
		}
	}

	for _, r := range o.Repos {
		if prowConfig.Presubmits[r] == nil {
			prowConfig.Presubmits[r] = make([]config.Presubmit, 0)
		}
		if prowConfig.Postsubmits[r] == nil {
			prowConfig.Postsubmits[r] = make([]config.Postsubmit, 0)
		}
		if preSubmit.Name != "" {
			found := false
			for i, j := range prowConfig.Presubmits[r] {
				if j.Name == preSubmit.Name {
					found = true
					prowConfig.Presubmits[r][i] = preSubmit
					break
				}
			}
			if !found {
				prowConfig.Presubmits[r] = append(prowConfig.Presubmits[r], preSubmit)
			}
		}
		if o.Kind != prowconfig.RemoteEnvironment && postSubmit.Name != "" {
			found := false
			for i, j := range prowConfig.Postsubmits[r] {
				if j.Name == postSubmit.Name {
					found = true
					prowConfig.Postsubmits[r][i] = postSubmit
					break
				}
			}
			if !found {
				prowConfig.Postsubmits[r] = append(prowConfig.Postsubmits[r], postSubmit)
			}
		}
	}

	if err := o.saveProwConfig(prowConfig, create); err != nil {
		return errors.Wrap(err, "saving prow config")
	}
	return nil
}

// RemoveProwConfig deletes a config (normally a repository integration) from Prow
func (o *Options) RemoveProwConfig() error {
	prowConfig, created, err := o.GetProwConfig()
	if created {
		return errors.New("no existing prow config. Nothing to remove")
	}
	if err != nil {
		return errors.Wrap(err, "getting existing prow config")
	}

	for _, repo := range o.Repos {
		err = prowconfig.RemoveRepoFromTideConfig(&prowConfig.Tide, repo, o.Kind)
		if err != nil {
			return errors.Wrapf(err, "removing repo %s from tide config", repo)
		}
		err = prowconfig.RemoveRepoFromBranchProtection(&prowConfig.BranchProtection, repo)
		if err != nil {
			return errors.Wrapf(err, "removing repo %s from branch protection", repo)
		}

		delete(prowConfig.Presubmits, repo)
		delete(prowConfig.Postsubmits, repo)
	}

	if err := o.saveProwConfig(prowConfig, created); err != nil {
		return errors.Wrap(err, "saving prow config")
	}
	return nil
}

func (o *Options) saveProwConfig(prowConfig *config.Config, create bool) error {
	configYAML, err := yaml.Marshal(prowConfig)
	if err != nil {
		return errors.Wrap(err, "marshaling the prow config")
	}

	data := make(map[string]string)
	data[ProwConfigFilename] = string(configYAML)
	cm := &corev1.ConfigMap{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: ProwConfigMapName,
		},
	}

	if create {
		// replace with git repository version of a configmap
		_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Create(cm)
		err = errors.Wrapf(err, "creating the config map %q", ProwConfigMapName)
	} else {
		_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Update(cm)
		err = errors.Wrapf(err, "updating the config map %q", ProwConfigMapName)
	}
	return err
}

func (o *Options) GetProwConfig() (*config.Config, bool, error) {
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(ProwConfigMapName, metav1.GetOptions{})
	create := true
	prowConfig := &config.Config{}
	// config doesn't exist, creating
	if err != nil {
		prowConfig.Presubmits = make(map[string][]config.Presubmit)
		prowConfig.Postsubmits = make(map[string][]config.Postsubmit)
		prowConfig.BranchProtection = config.BranchProtection{}

		// calculate the tide url from the ingress config
		ingressConfigMap, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(kube.IngressConfigConfigmap, metav1.GetOptions{})
		if err != nil {
			return prowConfig, create, errors.Wrapf(err, "get %q config map from namespace %q", kube.IngressConfigConfigmap, o.NS)
		}
		domain := ingressConfigMap.Data["domain"]
		tls := ingressConfigMap.Data["tls"]
		scheme := "http"
		if tls == "true" {
			scheme = "https"
		}

		tideURL := fmt.Sprintf("%s://deck.%s.%s", scheme, o.NS, domain)
		prowConfig.Tide = prowconfig.CreateTide(tideURL)
	} else {
		// config exists, updating
		create = false
		err = yaml.Unmarshal([]byte(cm.Data[ProwConfigFilename]), &prowConfig)
		if err != nil {
			return prowConfig, create, errors.Wrap(err, "unmarshaling prow config")
		}
		if len(prowConfig.Presubmits) == 0 {
			prowConfig.Presubmits = make(map[string][]config.Presubmit)
		}
		if len(prowConfig.Postsubmits) == 0 {
			prowConfig.Postsubmits = make(map[string][]config.Postsubmit)
		}
		if prowConfig.BranchProtection.Orgs == nil {
			prowConfig.BranchProtection.Orgs = make(map[string]config.Org, 0)
		}
	}
	return prowConfig, create, nil
}

// LoadProwConfigFromFile loads prow config from a file
func (o *Options) LoadProwConfigFromFile() (*config.Config, error) {
	exists, err := util.FileExists(o.ConfigFileLocation)
	if err != nil {
		return nil, errors.Wrap(err, "loading prow config from "+o.ConfigFileLocation)
	}
	if exists {
		data, err := ioutil.ReadFile(o.ConfigFileLocation)
		if err != nil {
			return nil, errors.New("loading prow config from " + o.ConfigFileLocation)
		}
		prowConfig := &config.Config{}
		if err == nil {
			err = yaml.Unmarshal(data, &prowConfig)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshaling prow config")
			}
			return prowConfig, nil
		}

	}
	return nil, errors.New("loading prow config from " + o.ConfigFileLocation)
}

// LoadProwPluginsFromFile loads prow plugins from a file
func (o *Options) LoadProwPluginsFromFile() (*plugins.Configuration, error) {
	exists, err := util.FileExists(o.PluginsFileLocation)
	if err != nil {
		return nil, errors.Wrap(err, "loading prow plugins from "+o.PluginsFileLocation)
	}
	if exists {
		data, err := ioutil.ReadFile(o.PluginsFileLocation)
		if err != nil {
			return nil, errors.New("loading prow plugins from " + o.PluginsFileLocation)
		}
		if err == nil {
			pluginConfig := &plugins.Configuration{}
			err = yaml.Unmarshal(data, &pluginConfig)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshaling plugin config")
			}
			return pluginConfig, nil
		}

	}
	return nil, errors.New("loading prow plugins from " + o.ConfigFileLocation)
}

// LoadProwConfig loads prow config from configmap
func (o *Options) LoadProwConfig() (*config.Config, error) {
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(ProwConfigMapName, metav1.GetOptions{})
	prowConfig := &config.Config{}
	if err == nil {
		err = yaml.Unmarshal([]byte(cm.Data[ProwConfigFilename]), &prowConfig)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshaling prow config")
		}
	} else {
		return nil, errors.Wrap(err, "loading prow config configmap")
	}
	return prowConfig, nil
}

// LoadPluginConfig loads prow plugins from a configmap
func (o *Options) LoadPluginConfig() (*plugins.Configuration, error) {
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(ProwPluginsConfigMapName, metav1.GetOptions{})
	pluginConfig := &plugins.Configuration{}
	if err == nil {
		err = yaml.Unmarshal([]byte(cm.Data[ProwPluginsFilename]), pluginConfig)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshaling plugins")
		}
	} else {
		return nil, errors.Wrap(err, "loading prow plugins configmap")
	}
	return pluginConfig, nil
}

func (o *Options) upsertPluginConfig(closure func(pluginConfig *plugins.Configuration,
	externalPlugins *ExternalPlugins) error) error {
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(ProwPluginsConfigMapName, metav1.GetOptions{})
	create := true
	pluginConfig := &plugins.Configuration{}
	externalPlugins := &ExternalPlugins{}
	if err == nil {
		create = false
		err = yaml.Unmarshal([]byte(cm.Data[ProwPluginsFilename]), pluginConfig)
		if err != nil {
			return errors.Wrap(err, "unmarshaling plugins")
		}
		err = yaml.Unmarshal([]byte(cm.Data[ProwExternalPluginsFilename]), externalPlugins)
		if err != nil {
			return errors.Wrap(err, "unmarshaling external plugins")
		}
	}

	if pluginConfig == nil {
		pluginConfig = &plugins.Configuration{}
		pluginConfig.ConfigUpdater.Maps = make(map[string]plugins.ConfigMapSpec)
		pluginConfig.ConfigUpdater.Maps["prow/config.yaml"] = plugins.ConfigMapSpec{Name: ProwConfigMapName}
		pluginConfig.ConfigUpdater.Maps["prow/plugins.yaml"] = plugins.ConfigMapSpec{Name: ProwPluginsConfigMapName}

	}
	if len(pluginConfig.Plugins) == 0 {
		pluginConfig.Plugins = make(map[string][]string)
	}
	if len(pluginConfig.ExternalPlugins) == 0 {
		pluginConfig.ExternalPlugins = make(map[string][]plugins.ExternalPlugin)
	}
	if len(pluginConfig.Approve) == 0 {
		pluginConfig.Approve = []plugins.Approve{}
	}
	if len(pluginConfig.Welcome) == 0 {
		pluginConfig.Welcome = []plugins.Welcome{
			{
				MessageTemplate: "Welcome",
			},
		}
	}

	err = closure(pluginConfig, externalPlugins)
	if err != nil {
		return errors.Wrap(err, "transforming the plugins and external plugins config")
	}

	pluginYAML, err := yaml.Marshal(pluginConfig)
	if err != nil {
		return errors.Wrap(err, "marshaling plugins config")
	}

	externalPluginsYAML, err := yaml.Marshal(externalPlugins)
	if err != nil {
		return errors.Wrap(err, "marshaling external plguins config")
	}

	data := make(map[string]string)
	data[ProwPluginsFilename] = string(pluginYAML)
	data[ProwExternalPluginsFilename] = string(externalPluginsYAML)
	cm = &corev1.ConfigMap{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: ProwPluginsConfigMapName,
		},
	}
	if create {
		_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Create(cm)
		err = errors.Wrapf(err, "createing the %q config map in namespace %q", ProwPluginsConfigMapName, o.NS)
	} else {
		_, err = o.KubeClient.CoreV1().ConfigMaps(o.NS).Update(cm)
		err = errors.Wrapf(err, "updating the %q config map in namespace %q", ProwPluginsConfigMapName, o.NS)
	}
	return err
}

// AddProwPlugins adds plugins and external plugins to prow for any repos defined in o.Repos,
// or for all repos which have plugins if o.Repos is nil
func (o *Options) AddProwPlugins() error {
	pluginsList := []string{"config-updater", "approve", "assign", "blunderbuss", "help", "hold", "lgtm", "lifecycle", "size", "trigger", "wip", "heart", "cat", "override"}
	closure := func(pluginConfig *plugins.Configuration, externalPlugins *ExternalPlugins) error {
		if o.Repos == nil {
			// Then we need react for all repos defined in the plugins list
			o.Repos = make([]string, 0)
			for r := range pluginConfig.Plugins {
				o.Repos = append(o.Repos, r)
			}
		}
		for _, r := range o.Repos {
			pluginConfig.Plugins[r] = pluginsList
			pTrue := true
			a := plugins.Approve{
				Repos:               []string{r},
				RequireSelfApproval: &pTrue,
				LgtmActsAsApprove:   true,
			}
			pluginConfig.Approve = append(pluginConfig.Approve, a)

			parts := strings.Split(r, "/")
			t := plugins.Trigger{
				Repos:      []string{r},
				TrustedOrg: parts[0],
			}
			pluginConfig.Triggers = append(pluginConfig.Triggers, t)

			// External Plugins
			pluginConfig.ExternalPlugins[r] = externalPlugins.Items
		}
		return nil
	}
	if err := o.upsertPluginConfig(closure); err != nil {
		return errors.Wrap(err, "upserting the plugins config")
	}
	return nil
}

func (o *Options) AddExternalProwPlugins(adds []plugins.ExternalPlugin) error {
	closure := func(pluginConfig *plugins.Configuration, externalPlugins *ExternalPlugins) error {
		for _, add := range adds {
			foundIndex := -1
			for i, p := range externalPlugins.Items {
				if p.Name == add.Name {
					foundIndex = i
					break
				}
			}
			if foundIndex < 0 {
				externalPlugins.Items = append(externalPlugins.Items, add)
			} else {
				externalPlugins.Items[foundIndex] = add
			}
		}
		return nil
	}
	if err := o.upsertPluginConfig(closure); err != nil {
		return errors.Wrap(err, "upserting the external plugins config")
	}
	return nil
}

func (o *Options) GetReleaseJobs() ([]string, error) {
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(ProwConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "getting the release jobs from %q config map in namespace %q", ProwConfigMapName, o.NS)
	}

	prowConfig := &config.Config{}
	err = yaml.Unmarshal([]byte(cm.Data[ProwConfigFilename]), &prowConfig)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshaling prow config")
	}
	var jobs []string

	for repo, p := range prowConfig.Postsubmits {
		for _, q := range p {
			for _, b := range q.Branches {
				repo = strings.Replace(repo, ":", "", -1)
				branch := prowBranchConfigToGitBranchName(b)
				jobName := fmt.Sprintf("%s/%s", repo, branch)
				if o.IgnoreBranch {
					jobName = repo
				}
				jobs = append(jobs, jobName)
			}
		}
	}
	return jobs, nil
}

func (o *Options) GetPostSubmitJob(org, repo, branch string) (config.Postsubmit, error) {
	p := config.Postsubmit{}
	cm, err := o.KubeClient.CoreV1().ConfigMaps(o.NS).Get(ProwConfigMapName, metav1.GetOptions{})
	if err != nil {
		return p, errors.Wrapf(err, "getting the presubmit jobs from %q config map in namespace %q", ProwConfigMapName, o.NS)
	}

	prowConfig := &config.Config{}
	err = yaml.Unmarshal([]byte(cm.Data[ProwConfigFilename]), &prowConfig)
	if err != nil {
		return p, errors.Wrap(err, "unmarshaling prow config")
	}

	key := fmt.Sprintf("%s/%s", org, repo)
	postsubmits := prowConfig.Postsubmits[key]

	err = config.SetPostsubmitRegexes(postsubmits)
	if err != nil {
		return p, errors.Wrap(err, "compiling prow postsubmit regexes")
	}

	for _, p := range postsubmits {
		if p.Brancher.ShouldRun(branch) {
			return p, nil
		}
	}
	return p, fmt.Errorf("no prow config build spec found for %s/%s/%s", org, repo, branch)
}

// CreateProwJob creates a new ProbJob resource for the Prow build controller to run
func CreateProwJob(client kubernetes.Interface, ns string, j prowapi.ProwJob) (prowapi.ProwJob, error) {
	retJob := prowapi.ProwJob{}
	body, err := json.Marshal(j)
	if err != nil {
		return retJob, errors.Wrap(err, "marshalling the prow job")
	}
	resp, err := client.CoreV1().RESTClient().Post().RequestURI(fmt.Sprintf("/apis/prow.k8s.io/v1/namespaces/%s/prowjobs", ns)).Body(body).DoRaw()
	if err != nil {
		return retJob, fmt.Errorf("creating prowjob %v: %s", err, string(resp))
	}
	return retJob, err
}

// prowBranchConfigToGitBranchName converts a (prow) branch (regex)
// into a git branch name
func prowBranchConfigToGitBranchName(b string) string {
	branch := strings.TrimLeft(b, "^")
	branch = strings.TrimRight(branch, "$")
	return branch
}
