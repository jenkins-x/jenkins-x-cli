package pipelinescheduler

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/prow"

	"github.com/jenkins-x/jx/pkg/environments"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	uuid "github.com/satori/go.uuid"
	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/helm/pkg/proto/hapi/chart"

	"k8s.io/client-go/kubernetes"

	"github.com/ghodss/yaml"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/plugins"
)

// GenerateProw will generate the prow config for the namespace
func GenerateProw(gitOps bool, autoApplyConfigUpdater bool, jxClient versioned.Interface, namespace string, teamSchedulerName string, devEnv *jenkinsv1.Environment, loadSchedulerResourcesFunc func(versioned.Interface, string) (map[string]*jenkinsv1.Scheduler, *jenkinsv1.SourceRepositoryGroupList, *jenkinsv1.SourceRepositoryList, error)) (*config.Config,
	*plugins.Configuration, error) {
	if loadSchedulerResourcesFunc == nil {
		loadSchedulerResourcesFunc = loadSchedulerResources
	}
	schedulers, sourceRepoGroups, sourceRepos, err := loadSchedulerResourcesFunc(jxClient, namespace)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "loading scheduler resources")
	}
	if sourceRepos == nil || len(sourceRepos.Items) < 1 {
		return nil, nil, errors.New("No source repository resources were found")
	}
	defaultScheduler := schedulers[teamSchedulerName]
	leaves := make([]*SchedulerLeaf, 0)
	for _, sourceRepo := range sourceRepos.Items {
		applicableSchedulers := []*jenkinsv1.SchedulerSpec{}
		// Apply config-updater to devEnv
		applicableSchedulers = addConfigUpdaterToDevEnv(gitOps, autoApplyConfigUpdater, applicableSchedulers, devEnv, &sourceRepo.Spec)
		// Apply repo scheduler
		applicableSchedulers = addRepositoryScheduler(sourceRepo, schedulers, applicableSchedulers)
		// Apply project schedulers
		applicableSchedulers = addProjectSchedulers(sourceRepoGroups, sourceRepo, schedulers, applicableSchedulers)
		// Apply team scheduler
		applicableSchedulers = addTeamScheduler(teamSchedulerName, defaultScheduler, applicableSchedulers)
		if len(applicableSchedulers) < 1 {
			continue
		}
		merged, err := Build(applicableSchedulers)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "building scheduler")
		}
		leaves = append(leaves, &SchedulerLeaf{
			Repo:          sourceRepo.Spec.Repo,
			Org:           sourceRepo.Spec.Org,
			SchedulerSpec: merged,
		})
		if err != nil {
			return nil, nil, errors.Wrapf(err, "building prow config")
		}
	}
	cfg, plugs, err := BuildProwConfig(leaves)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "building prow config")
	}
	if cfg != nil {
		cfg.PodNamespace = namespace
		cfg.ProwJobNamespace = namespace
	}
	return cfg, plugs, nil
}

func loadSchedulerResources(jxClient versioned.Interface, namespace string) (map[string]*jenkinsv1.Scheduler, *jenkinsv1.SourceRepositoryGroupList, *jenkinsv1.SourceRepositoryList, error) {
	schedulers, err := jxClient.JenkinsV1().Schedulers(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}
	if len(schedulers.Items) == 0 {
		return nil, nil, nil, errors.New("No pipeline schedulers are configured")
	}
	lookup := make(map[string]*jenkinsv1.Scheduler)
	for _, item := range schedulers.Items {
		lookup[item.Name] = item.DeepCopy()
	}
	// Process Schedulers linked to SourceRepositoryGroups
	sourceRepoGroups, err := jxClient.JenkinsV1().SourceRepositoryGroups(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "Error finding source repository groups")
	}
	// Process Schedulers linked to SourceRepositoryGroups
	sourceRepos, err := jxClient.JenkinsV1().SourceRepositories(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "Error finding source repositories")
	}
	return lookup, sourceRepoGroups, sourceRepos, nil
}

// CreateSchedulersFromProwConfig will generate Pipeline Schedulers from the prow configmaps in the specified namespace or the config and plugins files specified as an option
func CreateSchedulersFromProwConfig(configFileLocation string, pluginsFileLocation string, skipVerification bool, dryRun bool, gitOps bool, jxClient versioned.Interface, kubeClient kubernetes.Interface, namespace string, teamSchedulerName string, devEnv *jenkinsv1.Environment) ([]*jenkinsv1.SourceRepositoryGroup, []*jenkinsv1.SourceRepository, map[string]*jenkinsv1.Scheduler, error) {
	prowConfig, pluginConfig, err := loadExistingProwConfig(configFileLocation, pluginsFileLocation, kubeClient, namespace)
	if err != nil {
		return nil, nil, nil, err
	}

	sourceRepoGroups, sourceRepositories, sourceRepoMap, schedulers, err := BuildSchedulers(prowConfig, pluginConfig)
	teamSchedulerName = "default-scheduler"
	cleanupExistingProwConfig(prowConfig, pluginConfig, sourceRepoMap)

	migratedConfigFunc := func(jxClient versioned.Interface, namespace string) (map[string]*jenkinsv1.Scheduler, *jenkinsv1.SourceRepositoryGroupList, *jenkinsv1.SourceRepositoryList, error) {
		values := []jenkinsv1.SourceRepository{}
		for _, value := range sourceRepositories {
			values = append(values, *value)
		}
		return schedulers, nil, &jenkinsv1.SourceRepositoryList{Items: values}, nil
	}
	if !skipVerification {
		log.Logger().Info("Verifying generated config")
		migratedConfig, migratedPlugins, err := GenerateProw(gitOps, false, jxClient, namespace, teamSchedulerName, devEnv, migratedConfigFunc)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "Error generating prow config")
		}
		if dryRun {
			dumpProwConfigToFiles("migrated", migratedConfig, migratedPlugins)
			dumpProwConfigToFiles("existing", prowConfig, pluginConfig)
		}
		sortSliceStringOpt := cmpopts.SortSlices(func(i string, j string) bool {
			return i < j
		})
		pluginCmpOptions := cmp.Options{sortSliceStringOpt,
			cmpopts.SortSlices(func(i plugins.Trigger, j plugins.Trigger) bool {
				iString := strings.Join(i.Repos, ",")
				jString := strings.Join(j.Repos, ",")
				return iString < jString
			}), cmpopts.SortSlices(func(i plugins.Approve, j plugins.Approve) bool {
				iString := strings.Join(i.Repos, ",")
				jString := strings.Join(j.Repos, ",")
				return iString < jString
			}), cmpopts.SortSlices(func(i plugins.Lgtm, j plugins.Lgtm) bool {
				iString := strings.Join(i.Repos, ",")
				jString := strings.Join(j.Repos, ",")
				return iString < jString
			}),
		}

		if !cmp.Equal(migratedPlugins, pluginConfig, pluginCmpOptions) {
			return nil, nil, nil, errors.Errorf("Migrated Prow plugins do not match, not applying! \nDiff: \n%s", cmp.Diff(migratedPlugins, pluginConfig, pluginCmpOptions))
		}
		cnfgCmpOptions := cmp.Options{
			cmpopts.IgnoreUnexported(config.Brancher{}),
			cmpopts.IgnoreUnexported(config.RegexpChangeMatcher{}),
			cmpopts.IgnoreUnexported(config.Presubmit{}),
			cmpopts.IgnoreUnexported(config.Periodic{}),
			sortSliceStringOpt,
			cmpopts.SortSlices(func(i config.TideQuery, j config.TideQuery) bool {
				sort.Strings(i.Repos)
				sort.Strings(j.Repos)
				iLabels := append(i.Labels, i.MissingLabels...)
				iString := strings.Join(append(i.Repos, iLabels...), ",")
				jLabels := append(j.Labels, j.MissingLabels...)
				jString := strings.Join(append(j.Repos, jLabels...), ",")
				return iString < jString
			})}
		if !cmp.Equal(migratedConfig, prowConfig, cnfgCmpOptions) {
			return nil, nil, nil, errors.Errorf("Migrated Prow config do not match, not applying! \nDiff: \n%s", cmp.Diff(migratedConfig, prowConfig, cnfgCmpOptions))
		}
		log.Logger().Info("Generated config passed validation")
	}
	return sourceRepoGroups, sourceRepositories, schedulers, nil
}

//cleanupExistingProwConfig Removes config that we do not currently support
func cleanupExistingProwConfig(prowConfig *config.Config, pluginConfig *plugins.Configuration, sourceRepoMap map[string]*jenkinsv1.SourceRepository) {
	// Deck is not supported
	prowConfig.Deck = config.Deck{}
	// heart plugin is not supported
	pluginConfig.Heart = plugins.Heart{}
	queries := prowConfig.Tide.Queries[:0]
	for _, query := range prowConfig.Tide.Queries {
		repos := query.Repos[:0]
		for _, repo := range query.Repos {
			// We do not support tide queries for repos with not presubmits or postsubmits, ignore the dummy environment for now
			if sourceRepoMap[repo] != nil {
				repos = append(repos, repo)
			}
		}
		if len(repos) > 0 {
			query.Repos = repos
			queries = append(queries, query)
		}
	}
	prowConfig.Tide.Queries = queries

	pluginConfig.ConfigUpdater = plugins.ConfigUpdater{}
	triggers := make([]plugins.Trigger, 0)
	for _, trigger := range pluginConfig.Triggers {
		for _, repo := range trigger.Repos {
			// We do not support tide queries for repos with not presubmits or postsubmits, ignore the dummy environment for now
			if sourceRepoMap[repo] != nil {
				triggers = append(triggers, trigger)
			}
		}
	}
	pluginConfig.Triggers = triggers

	lgtms := make([]plugins.Lgtm, 0)
	for _, lgtm := range pluginConfig.Lgtm {
		for _, repo := range lgtm.Repos {
			// We do not support tide queries for repos with not presubmits or postsubmits, ignore the dummy environment for now
			if sourceRepoMap[repo] != nil {
				newLgtm := lgtm
				newLgtm.Repos = []string{repo}
				lgtms = append(lgtms, newLgtm)
			}
		}
	}
	pluginConfig.Lgtm = lgtms

	approves := make([]plugins.Approve, 0)
	for _, approve := range pluginConfig.Approve {
		for _, repo := range approve.Repos {
			if !strings.Contains(repo, "/") {
				// Expand the org in to repos for now
				for existingRepo := range sourceRepoMap {
					if strings.HasPrefix(existingRepo, repo+"/") {
						newApprove := approve
						newApprove.Repos = []string{existingRepo}
						approves = append(approves, newApprove)
					}
				}
			} else {
				// We do not support tide queries for repos with not presubmits or postsubmits, ignore the dummy environment for now
				if sourceRepoMap[repo] != nil {
					approves = append(approves, approve)
				}
			}
		}
	}
	pluginConfig.Approve = approves

	// Branch org protection policy out in to repos
	for org := range prowConfig.BranchProtection.Orgs {
		protectionOrg := prowConfig.BranchProtection.Orgs[org]
		if protectionOrg.Repos == nil {
			protectionOrg.Repos = make(map[string]config.Repo)
			replacedOrgPolicy := false
			for existingRepo := range sourceRepoMap {
				if strings.HasPrefix(existingRepo, org+"/") {
					orgRepo := strings.Split(existingRepo, "/")
					repoPolicy := config.Repo{
						Policy: protectionOrg.Policy,
					}
					protectionOrg.Repos[orgRepo[1]] = repoPolicy
					prowConfig.BranchProtection.Orgs[org] = protectionOrg
					replacedOrgPolicy = true
				}
			}
			if replacedOrgPolicy {
				protectionOrg := prowConfig.BranchProtection.Orgs[org]
				protectionOrg.Policy = config.Policy{}
				prowConfig.BranchProtection.Orgs[org] = protectionOrg
			}
		}
	}

	for repo, plugins := range pluginConfig.ExternalPlugins {
		if plugins == nil {
			delete(pluginConfig.ExternalPlugins, repo)
			continue
		}
		if sourceRepoMap[repo] == nil {
			delete(pluginConfig.ExternalPlugins, repo)
		}
	}
	for repo := range pluginConfig.Plugins {
		if sourceRepoMap[repo] == nil {
			delete(pluginConfig.Plugins, repo)
		}
	}
}

func dumpProwConfigToFiles(prefix string, prowConfig *config.Config, pluginConfig *plugins.Configuration) {
	migratedConfigFile := prefix + "Config.yaml"
	cnfBytes, err := yaml.Marshal(prowConfig)
	if err != nil {
		log.Logger().Error("marshaling prow plugins config to yaml")
	}
	err = ioutil.WriteFile(migratedConfigFile, cnfBytes, 0600)
	if err != nil {
		log.Logger().Errorf("writing %s", migratedConfigFile)
	}
	log.Logger().Infof("Writing migrated config to %s", migratedConfigFile)
	migratedPluginsFile := prefix + "Plugins.yaml"
	plugsBytes, err := yaml.Marshal(pluginConfig)
	if err != nil {
		log.Logger().Error("marshaling prow plugins config to yaml")
	}
	err = ioutil.WriteFile(migratedPluginsFile, plugsBytes, 0600)
	if err != nil {
		log.Logger().Errorf("writing %s", migratedPluginsFile)
	}
	log.Logger().Infof("Writing migrated plugins to %s", migratedPluginsFile)
}

// GenerateSchedulers will generate Pipeline Schedulers from the prow configmaps in the specified namespace
func loadExistingProwConfig(configFileLocation string, pluginsFileLocation string, kubeClient kubernetes.Interface, namespace string) (prowConfig *config.Config, pluginConfig *plugins.Configuration, err error) {
	prowOptions := prow.Options{
		KubeClient:          kubeClient,
		NS:                  namespace,
		ConfigFileLocation:  configFileLocation,
		PluginsFileLocation: pluginsFileLocation,
	}
	if configFileLocation != "" {
		prowConfig, err = prowOptions.LoadProwConfigFromFile()
		log.Logger().Infof("Loading prow config from file %s", configFileLocation)
	} else {
		prowConfig, err = prowOptions.LoadProwConfig()
		log.Logger().Info("Loading prow config from configmap")
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting prow config")
	}
	if pluginsFileLocation != "" {
		pluginConfig, err = prowOptions.LoadProwPluginsFromFile()
		log.Logger().Infof("Loading prow plugins from file %s", pluginsFileLocation)
	} else {
		pluginConfig, err = prowOptions.LoadPluginConfig()
		log.Logger().Info("Loading prow plugins from configmap")
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "getting prow plugins config")
	}
	return prowConfig, pluginConfig, nil
}

func addTeamScheduler(defaultSchedulerName string, defaultScheduler *jenkinsv1.Scheduler, applicableSchedulers []*jenkinsv1.SchedulerSpec) []*jenkinsv1.SchedulerSpec {
	if defaultScheduler != nil {
		applicableSchedulers = append([]*jenkinsv1.SchedulerSpec{&defaultScheduler.Spec}, applicableSchedulers...)
	} else {
		if defaultSchedulerName != "" {
			log.Logger().Warnf("A team pipeline scheduler named %s was configured but could not be found", defaultSchedulerName)
		}
	}
	return applicableSchedulers
}

func addRepositoryScheduler(sourceRepo jenkinsv1.SourceRepository, lookup map[string]*jenkinsv1.Scheduler, applicableSchedulers []*jenkinsv1.SchedulerSpec) []*jenkinsv1.SchedulerSpec {
	if sourceRepo.Spec.Scheduler.Name != "" {
		scheduler := lookup[sourceRepo.Spec.Scheduler.Name]
		if scheduler != nil {
			applicableSchedulers = append([]*jenkinsv1.SchedulerSpec{&scheduler.Spec}, applicableSchedulers...)
		} else {
			log.Logger().Warnf("A scheduler named %s is referenced by repository(%s) but could not be found", sourceRepo.Spec.Scheduler.Name, sourceRepo.Name)
		}
	}
	return applicableSchedulers
}

func addProjectSchedulers(sourceRepoGroups *jenkinsv1.SourceRepositoryGroupList, sourceRepo jenkinsv1.SourceRepository, lookup map[string]*jenkinsv1.Scheduler, applicableSchedulers []*jenkinsv1.SchedulerSpec) []*jenkinsv1.SchedulerSpec {
	if sourceRepoGroups != nil {
		for _, sourceGroup := range sourceRepoGroups.Items {
			for _, groupRepo := range sourceGroup.Spec.SourceRepositorySpec {
				if groupRepo.Name == sourceRepo.Name {
					if sourceGroup.Spec.Scheduler.Name != "" {
						scheduler := lookup[sourceGroup.Spec.Scheduler.Name]
						if scheduler != nil {
							applicableSchedulers = append([]*jenkinsv1.SchedulerSpec{&scheduler.Spec}, applicableSchedulers...)
						} else {
							log.Logger().Warnf("A scheduler named %s is referenced by repository group(%s) but could not be found", sourceGroup.Spec.Scheduler.Name, sourceGroup.Name)
						}
					}
				}
			}
		}
	}
	return applicableSchedulers
}

func addConfigUpdaterToDevEnv(gitOps bool, autoApplyConfigUpdater bool, applicableSchedulers []*jenkinsv1.SchedulerSpec, devEnv *jenkinsv1.Environment, sourceRepo *jenkinsv1.SourceRepositorySpec) []*jenkinsv1.SchedulerSpec {
	if gitOps && autoApplyConfigUpdater && strings.Contains(devEnv.Spec.Source.URL, sourceRepo.Org+"/"+sourceRepo.Repo) {
		maps := make(map[string]jenkinsv1.ConfigMapSpec)
		maps["env/prow/config.yaml"] = jenkinsv1.ConfigMapSpec{
			Name: "config",
		}
		maps["env/prow/plugins.yaml"] = jenkinsv1.ConfigMapSpec{
			Name: "plugins",
		}
		environmentUpdaterSpec := &jenkinsv1.SchedulerSpec{
			ConfigUpdater: &jenkinsv1.ConfigUpdater{
				Map: maps,
			},
			Plugins: &jenkinsv1.ReplaceableSliceOfStrings{
				Items: []string{"config-updater"},
			},
		}
		applicableSchedulers = append([]*jenkinsv1.SchedulerSpec{environmentUpdaterSpec}, applicableSchedulers...)
	}
	return applicableSchedulers
}

//ApplyDirectly directly applies the prow config to the cluster
func ApplyDirectly(kubeClient kubernetes.Interface, namespace string, cfg *config.Config,
	plugs *plugins.Configuration) error {
	cfgYaml, err := yaml.Marshal(cfg)
	if err != nil {
		return errors.Wrapf(err, "marshalling config to yaml")
	}
	cfgConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"config.yaml": string(cfgYaml),
		},
	}
	plugsYaml, err := yaml.Marshal(plugs)
	if err != nil {
		return errors.Wrapf(err, "marshalling plugins to yaml")
	}
	plugsConfigMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plugins",
			Namespace: namespace,
		},
		Data: map[string]string{
			"plugins.yaml": string(plugsYaml),
		},
	}
	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Update(cfgConfigMap)
	if kubeerrors.IsNotFound(err) {
		_, err := kubeClient.CoreV1().ConfigMaps(namespace).Create(cfgConfigMap)
		if err != nil {
			return errors.Wrapf(err, "creating ConfigMap config")
		}
	} else if err != nil {
		return errors.Wrapf(err, "updating ConfigMap config")
	}
	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Update(plugsConfigMap)
	if kubeerrors.IsNotFound(err) {
		_, err := kubeClient.CoreV1().ConfigMaps(namespace).Create(plugsConfigMap)
		if err != nil {
			return errors.Wrapf(err, "creating ConfigMap plugins")
		}
	} else if err != nil {
		return errors.Wrapf(err, "updating ConfigMap plugins")
	}
	return nil
}

//ApplySchedulersDirectly directly applies pipeline schedulers to the cluster
func ApplySchedulersDirectly(jxClient versioned.Interface, namespace string, sourceRepositoryGroups []*jenkinsv1.SourceRepositoryGroup, sourceRepositories []*jenkinsv1.SourceRepository, schedulers map[string]*jenkinsv1.Scheduler, devEnv *jenkinsv1.Environment) error {
	log.Logger().Infof("Applying scheduler configuration to namespace %s", namespace)
	err := jxClient.JenkinsV1().Schedulers(namespace).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "Error removing existing schedulers")
	}
	for _, scheduler := range schedulers {
		_, err := jxClient.JenkinsV1().Schedulers(namespace).Update(scheduler)
		if kubeerrors.IsNotFound(err) {
			_, err := jxClient.JenkinsV1().Schedulers(namespace).Create(scheduler)
			if err != nil {
				return errors.Wrapf(err, "creating scheduler")
			}
		} else if err != nil {
			return errors.Wrapf(err, "updating scheduler")
		}
		if scheduler.Name == "default-scheduler" {
			devEnv.Spec.TeamSettings.DefaultScheduler.Name = scheduler.Name
			devEnv.Spec.TeamSettings.DefaultScheduler.Kind = "Scheduler"
			jxClient.JenkinsV1().Environments(namespace).PatchUpdate(devEnv)
		}
	}
	for _, repo := range sourceRepositories {
		sourceRepo, err := kube.GetOrCreateSourceRepository(jxClient, namespace, repo.Spec.Repo, repo.Spec.Org, repo.Spec.Provider)
		if err != nil || sourceRepo == nil {
			return errors.New("Getting / creating source repo")
		}
		sourceRepo.Spec.Scheduler.Name = repo.Spec.Scheduler.Name
		sourceRepo.Spec.Scheduler.Kind = repo.Spec.Scheduler.Kind
		_, err = jxClient.JenkinsV1().SourceRepositories(namespace).Update(sourceRepo)
		if err != nil {
			return errors.Wrapf(err, "updating source repo")
		}
	}
	for _, repoGroup := range sourceRepositoryGroups {
		_, err := jxClient.JenkinsV1().SourceRepositoryGroups(namespace).Update(repoGroup)
		if kubeerrors.IsNotFound(err) {
			_, err := jxClient.JenkinsV1().SourceRepositoryGroups(namespace).Create(repoGroup)
			if err != nil {
				return errors.Wrapf(err, "creating source repo group")
			}
		} else if err != nil {
			return errors.Wrapf(err, "updating source repo group")
		}
	}

	return nil
}

//GitOpsOptions are options for running AddToEnvironmentRepo
type GitOpsOptions struct {
	Gitter              gits.Gitter
	Verbose             bool
	Helmer              helm.Helmer
	GitProvider         gits.GitProvider
	DevEnv              *jenkinsv1.Environment
	PullRequestCloneDir string
}

// AddToEnvironmentRepo adds the prow config to the gitops environment repo
func (o *GitOpsOptions) AddToEnvironmentRepo(cfg *config.Config, plugs *plugins.Configuration, kubeClient kubernetes.Interface, namespace string) error {
	branchNameUUID, err := uuid.NewV4()
	if err != nil {
		return errors.Wrapf(err, "creating creating branch name")
	}
	validBranchName := "add-prow-config" + branchNameUUID.String()
	details := gits.PullRequestDetails{
		BranchName: validBranchName,
		Title:      fmt.Sprintf("Add Prow config"),
		Message:    fmt.Sprintf("Add Prow config generated on %s", time.Now()),
	}

	modifyChartFn := func(requirements *helm.Requirements, metadata *chart.Metadata,
		existingValues map[string]interface{},
		templates map[string]string, dir string, pullRequestDetails *gits.PullRequestDetails) error {
		prowDir := filepath.Join(dir, "prow")
		err := os.MkdirAll(prowDir, 0700)
		if err != nil {
			return errors.Wrapf(err, "creating prow dir in gitops repo %s", prowDir)
		}
		cfgBytes, err := yaml.Marshal(cfg)
		if err != nil {
			return errors.Wrapf(err, "marshaling prow config to yaml")
		}
		cfgPath := filepath.Join(prowDir, "config.yaml")
		err = ioutil.WriteFile(cfgPath, cfgBytes, 0600)
		if err != nil {
			return errors.Wrapf(err, "writing %s", cfgPath)
		}

		plugsBytes, err := yaml.Marshal(plugs)
		if err != nil {
			return errors.Wrapf(err, "marshaling prow plugins config to yaml")
		}
		plugsPath := filepath.Join(prowDir, "plugins.yaml")
		err = ioutil.WriteFile(plugsPath, plugsBytes, 0600)
		if err != nil {
			return errors.Wrapf(err, "writing %s", plugsPath)
		}
		return nil
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter:        o.Gitter,
		ModifyChartFn: modifyChartFn,
		GitProvider:   o.GitProvider,
	}

	info, err := options.Create(o.DevEnv, o.PullRequestCloneDir, &details, nil, "", false)

	if err != nil {
		return errors.Wrapf(err, "creating pr for prow config")
	}
	if info != nil {
		log.Logger().Infof("Added prow config via Pull Request %s", info.PullRequest.URL)
	}
	err = o.RegisterProwConfigUpdater(kubeClient, namespace)
	if err != nil {
		return errors.Wrapf(err, "Updating prow configmap")
	}
	return nil
}

// RegisterProwConfigUpdater Register the config updater in the plugin configmap
func (o *GitOpsOptions) RegisterProwConfigUpdater(kubeClient kubernetes.Interface, namespace string) error {
	prowOptions := prow.Options{
		KubeClient: kubeClient,
		NS:         namespace,
	}
	pluginConfig, err := prowOptions.LoadPluginConfig()
	if err != nil {
		return errors.Wrapf(err, "getting plugins configmap")
	}
	pluginConfig.ConfigUpdater.Maps = make(map[string]plugins.ConfigMapSpec)
	pluginConfig.ConfigUpdater.Maps["env/prow/config.yaml"] = plugins.ConfigMapSpec{Name: prow.ProwConfigMapName}
	pluginConfig.ConfigUpdater.Maps["env/prow/plugins.yaml"] = plugins.ConfigMapSpec{Name: prow.ProwPluginsConfigMapName}
	pluginYAML, err := yaml.Marshal(pluginConfig)
	if err != nil {
		return errors.Wrap(err, "marshaling the prow plugins")
	}

	data := make(map[string]string)
	data[prow.ProwPluginsFilename] = string(pluginYAML)
	cm := &v1.ConfigMap{
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: prow.ProwPluginsConfigMapName,
		},
	}
	_, err = kubeClient.CoreV1().ConfigMaps(namespace).Update(cm)
	if err != nil {
		err = errors.Wrapf(err, "updating the config map %q", prow.ProwPluginsConfigMapName)
	}
	return nil
}

// AddSchedulersToEnvironmentRepo adds the prow config to the gitops environment repo
func (o *GitOpsOptions) AddSchedulersToEnvironmentRepo(sourceRepositoryGroups []*jenkinsv1.SourceRepositoryGroup, sourceRepositories []*jenkinsv1.SourceRepository, schedulers map[string]*jenkinsv1.Scheduler) error {
	branchNameUUID, err := uuid.NewV4()
	if err != nil {
		return errors.Wrapf(err, "creating creating branch name")
	}
	log.Logger().Info("Saving scheduler config to environment repository")
	validBranchName := "add-pipeline-schedulers" + branchNameUUID.String()
	details := gits.PullRequestDetails{
		BranchName: validBranchName,
		Title:      fmt.Sprintf("Add pipeline schedulers"),
		Message:    fmt.Sprintf("Add pipeline schedulers generated on %s", time.Now()),
	}

	modifyChartFn := func(requirements *helm.Requirements, metadata *chart.Metadata,
		existingValues map[string]interface{},
		templates map[string]string, dir string, pullRequestDetails *gits.PullRequestDetails) error {
		templatesDir := filepath.Join(dir, "templates")
		for _, repoGroup := range sourceRepositoryGroups {
			cfgBytes, err := yaml.Marshal(repoGroup)
			if err != nil {
				return errors.Wrapf(err, "marshaling source repo group to yaml")
			}
			cfgPath := filepath.Join(templatesDir, repoGroup.Name+"-repo-group.yaml")
			err = ioutil.WriteFile(cfgPath, cfgBytes, 0600)
			if err != nil {
				return errors.Wrapf(err, "writing %s", cfgPath)
			}
		}

		for _, repo := range sourceRepositories {
			repoName := naming.ToValidName(repo.Spec.Org + "-" + repo.Spec.Repo)
			if repo.Name == "" {
				repo.Name = repoName
			}
			cfgBytes, err := yaml.Marshal(repo)
			if err != nil {
				return errors.Wrapf(err, "marshaling source repos to yaml")
			}
			cfgPath := filepath.Join(templatesDir, repo.Name+"-repo.yaml")
			err = ioutil.WriteFile(cfgPath, cfgBytes, 0600)
			if err != nil {
				return errors.Wrapf(err, "writing %s", cfgPath)
			}
		}
		for _, scheduler := range schedulers {
			cfgBytes, err := yaml.Marshal(scheduler)
			if err != nil {
				return errors.Wrapf(err, "marshaling schedulers to yaml")
			}
			cfgPath := filepath.Join(templatesDir, scheduler.Name+"-sch.yaml")
			err = ioutil.WriteFile(cfgPath, cfgBytes, 0600)
			if err != nil {
				return errors.Wrapf(err, "writing %s", cfgPath)
			}
			if scheduler.Name == "default-scheduler" {
				devEnvResource := &jenkinsv1.Environment{}
				devEnvFile := filepath.Join(templatesDir, "dev-env.yaml")
				devEnvData, err := ioutil.ReadFile(devEnvFile)
				if err != nil {
					return errors.Wrapf(err, "reading dev-env file %s", devEnvFile)
				}
				err = yaml.Unmarshal(devEnvData, devEnvResource)
				if err != nil {
					return errors.Wrapf(err, "reading dev-env yaml %s", devEnvFile)
				}
				devEnvResource.Spec.TeamSettings.DefaultScheduler.Kind = "Scheduler"
				devEnvResource.Spec.TeamSettings.DefaultScheduler.Name = scheduler.Name
				devEnvBytes, err := yaml.Marshal(devEnvResource)
				if err != nil {
					return errors.Wrapf(err, "reading dev-env resource %s", devEnvFile)
				}
				err = ioutil.WriteFile(devEnvFile, devEnvBytes, 0600)
				if err != nil {
					return errors.Wrapf(err, "writing %s", devEnvFile)
				}
			}
		}
		return nil
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter:        o.Gitter,
		ModifyChartFn: modifyChartFn,
		GitProvider:   o.GitProvider,
	}

	info, err := options.Create(o.DevEnv, o.PullRequestCloneDir, &details, nil, "", false)

	if err != nil {
		return errors.Wrapf(err, "creating pr for scheduler config")
	}
	if info != nil {
		log.Logger().Infof("Added pipeline scheduler config via Pull Request %s\n", info.PullRequest.URL)
	}
	return nil
}
