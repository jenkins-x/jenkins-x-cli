package tekton

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	clientv1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	jxClient "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineType is used to differentiate between actual build pipelines and pipelines to create the build pipelines,
// aka meta pipelines.
type PipelineType int

const (
	// BuildPipeline is the yype for the actual build pipeline
	BuildPipeline PipelineType = iota

	// MetaPipeline type for the meta pipeline used to generate the build pipeline
	MetaPipeline
)

func (s PipelineType) String() string {
	return [...]string{"build", "meta"}[s]
}

// GeneratePipelineActivity generates a initial PipelineActivity CRD so UI/get act can get an earlier notification that the jobs have been scheduled
func GeneratePipelineActivity(buildNumber string, branch string, gitInfo *gits.GitRepository, context string, pr *PullRefs) *kube.PromoteStepActivityKey {
	name := gitInfo.Organisation + "-" + gitInfo.Name + "-" + branch + "-" + buildNumber

	pipeline := gitInfo.Organisation + "/" + gitInfo.Name + "/" + branch
	log.Logger().Infof("PipelineActivity for %s", name)
	key := &kube.PromoteStepActivityKey{
		PipelineActivityKey: kube.PipelineActivityKey{
			Name:     name,
			Pipeline: pipeline,
			Build:    buildNumber,
			GitInfo:  gitInfo,
			Context:  context,
		},
	}

	if pr != nil {
		key.PullRefs = pr.ToMerge
	}

	return key
}

// CreateOrUpdateSourceResource lazily creates a Tekton Pipeline PipelineResource for the given git repository
func CreateOrUpdateSourceResource(tektonClient tektonclient.Interface, ns string, created *v1alpha1.PipelineResource) (*v1alpha1.PipelineResource, error) {
	resourceName := created.Name
	resourceInterface := tektonClient.TektonV1alpha1().PipelineResources(ns)

	_, err := resourceInterface.Create(created)
	if err == nil {
		return created, nil
	}

	answer, err2 := resourceInterface.Get(resourceName, metav1.GetOptions{})
	if err2 != nil {
		return answer, errors.Wrapf(err, "failed to get PipelineResource %s with %v after failing to create a new one", resourceName, err2)
	}
	if !reflect.DeepEqual(&created.Spec, &answer.Spec) {
		answer.Spec = created.Spec
		answer, err = resourceInterface.Update(answer)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to update PipelineResource %s", resourceName)
		}
	}
	return answer, nil
}

// CreateOrUpdateTask lazily creates a Tekton Pipeline Task
func CreateOrUpdateTask(tektonClient tektonclient.Interface, ns string, created *v1alpha1.Task) (*v1alpha1.Task, error) {
	resourceName := created.Name
	if resourceName == "" {
		return nil, fmt.Errorf("the Task must have a name")
	}
	resourceInterface := tektonClient.TektonV1alpha1().Tasks(ns)

	_, err := resourceInterface.Create(created)
	if err == nil {
		return created, nil
	}

	answer, err2 := resourceInterface.Get(resourceName, metav1.GetOptions{})
	if err2 != nil {
		return answer, errors.Wrapf(err, "failed to get PipelineResource %s with %v after failing to create a new one", resourceName, err2.Error())
	}
	if !reflect.DeepEqual(&created.Spec, &answer.Spec) || !reflect.DeepEqual(created.Annotations, answer.Annotations) || !reflect.DeepEqual(created.Labels, answer.Labels) {
		answer.Spec = created.Spec
		answer.Labels = util.MergeMaps(answer.Labels, created.Labels)
		answer.Annotations = util.MergeMaps(answer.Annotations, created.Annotations)
		answer, err = resourceInterface.Update(answer)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to update PipelineResource %s", resourceName)
		}
	}
	return answer, nil
}

func nextBuildNumberFromActivity(activityInterface clientv1.PipelineActivityInterface, gitInfo *gits.GitRepository, branch string) (string, error) {
	labelMap := labels.Set{
		"owner":      gitInfo.Organisation,
		"repository": gitInfo.Name,
		"branch":     branch,
	}

	activityList, err := kube.ListSelectedPipelineActivities(activityInterface, labelMap.AsSelector(), nil)
	if err != nil {
		return "", errors.Wrapf(err, "Unable to list pipeline activities for %s/%s/%s", gitInfo.Organisation, gitInfo.Name, branch)
	}
	if len(activityList.Items) == 0 {
		return "1", nil
	}
	sort.Slice(activityList.Items, func(i, j int) bool {
		iBuildNum, err := strconv.Atoi(activityList.Items[i].Spec.Build)
		if err != nil {
			iBuildNum = 0
		}
		jBuildNum, err := strconv.Atoi(activityList.Items[j].Spec.Build)
		if err != nil {
			jBuildNum = 0
		}
		return iBuildNum >= jBuildNum
	})
	// Iterate over the sorted (highest to lowest build number) list of activities, returning a new build number
	// as soon as we reach one we can parse to an int and add one to.
	for _, activity := range activityList.Items {
		actBuildNum, err := strconv.Atoi(activity.Spec.Build)
		if err != nil {
			continue
		}
		return strconv.Itoa(actBuildNum + 1), nil
	}
	// If we couldn't parse any build numbers, just set the next build number to 1.
	return "1", nil
}

func nextBuildNumberFromSourceRepo(tektonClient tektonclient.Interface, jxClient jxClient.Interface, ns string, gitInfo *gits.GitRepository, branch string, context string) (string, error) {
	resourceInterface := jxClient.JenkinsV1().SourceRepositories(ns)
	// TODO: How does SourceRepository handle name overlap?
	sourceRepoName := naming.ToValidName(gitInfo.Organisation + "-" + gitInfo.Name)

	lastBuildNumber := 0
	sourceRepo, err := kube.GetOrCreateSourceRepository(jxClient, ns, gitInfo.Name, gitInfo.Organisation, gitInfo.ProviderURL())
	if err != nil {
		return "", errors.Wrapf(err, "Unable to generate next build number for %s/%s", sourceRepoName, branch)
	}
	sourceRepoName = sourceRepo.Name
	if sourceRepo.Annotations == nil {
		sourceRepo.Annotations = make(map[string]string, 1)
	}
	annKey := LastBuildNumberAnnotationPrefix + naming.ToValidName(branch)
	annVal := sourceRepo.Annotations[annKey]
	if annVal != "" {
		lastBuildNumber, err = strconv.Atoi(annVal)
		if err != nil {
			return "", errors.Wrapf(err, "Expected number but SourceRepository %s has annotation %s with value %s\n", sourceRepoName, annKey, annVal)
		}
	}

	for nextNumber := lastBuildNumber + 1; true; nextNumber++ {
		// lets check there is not already a PipelineRun for this number
		buildIdentifier := strconv.Itoa(nextNumber)

		labelSelector := fmt.Sprintf("owner=%s,repo=%s,branch=%s,build=%s", gitInfo.Organisation, gitInfo.Name, branch, buildIdentifier)
		if context != "" {
			labelSelector += fmt.Sprintf(",context=%s", context)
		}

		prs, err := tektonClient.TektonV1alpha1().PipelineRuns(ns).List(metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err == nil && len(prs.Items) > 0 {
			// lets try make another build number as there's already a PipelineRun
			// which could be due to name clashes
			continue
		}
		if sourceRepo != nil {
			sourceRepo.Annotations[annKey] = buildIdentifier
			if _, err := resourceInterface.Update(sourceRepo); err != nil {
				return "", err
			}
		}

		return buildIdentifier, nil
	}
	// We've somehow gotten here without determining the next build number, so let's error.
	return "", fmt.Errorf("couldn't determine next build number for %s/%s/%s", gitInfo.Organisation, gitInfo.Name, branch)
}

// GenerateNextBuildNumber generates a new build number for the given project.
func GenerateNextBuildNumber(tektonClient tektonclient.Interface, jxClient jxClient.Interface, ns string, gitInfo *gits.GitRepository, branch string, duration time.Duration, context string, useActivity bool) (string, error) {
	nextBuildNumber := ""
	activityInterface := jxClient.JenkinsV1().PipelineActivities(ns)

	f := func() error {
		if useActivity {
			bn, err := nextBuildNumberFromActivity(activityInterface, gitInfo, branch)
			if err != nil {
				return err
			}
			nextBuildNumber = bn
		} else {
			bn, err := nextBuildNumberFromSourceRepo(tektonClient, jxClient, ns, gitInfo, branch, context)
			if err != nil {
				return err
			}
			nextBuildNumber = bn
		}
		return nil
	}

	err := util.Retry(duration, f)
	if err != nil {
		return "", err
	}
	return nextBuildNumber, nil
}

// GenerateSourceRepoResource generates the PipelineResource for the git repository we are operating on.
func GenerateSourceRepoResource(name string, gitInfo *gits.GitRepository, revision string) *pipelineapi.PipelineResource {
	if gitInfo == nil || gitInfo.HttpsURL() == "" {
		return nil

	}

	// lets use the URL property as this preserves any provider specific paths; e.g. `/scm` on bitbucket server
	u := gitInfo.URL
	if u == "" {
		u = gitInfo.HttpsURL()
	}
	resource := &pipelineapi.PipelineResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: syntax.TektonAPIVersion,
			Kind:       "PipelineResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: pipelineapi.PipelineResourceSpec{
			Type: pipelineapi.PipelineResourceTypeGit,
			Params: []pipelineapi.ResourceParam{
				{
					Name:  "revision",
					Value: revision,
				},
				{
					Name:  "url",
					Value: u,
				},
			},
		},
	}

	return resource
}

// CreatePipelineRun creates the PipelineRun struct.
func CreatePipelineRun(resources []*pipelineapi.PipelineResource,
	name string,
	apiVersion string,
	labels map[string]string,
	serviceAccount string,
	pipelineParams []pipelineapi.Param,
	timeout *metav1.Duration,
	affinity *corev1.Affinity,
	tolerations []corev1.Toleration) *pipelineapi.PipelineRun {
	var resourceBindings []pipelineapi.PipelineResourceBinding
	for _, resource := range resources {
		resourceBindings = append(resourceBindings, pipelineapi.PipelineResourceBinding{
			Name: resource.Name,
			ResourceRef: pipelineapi.PipelineResourceRef{
				Name:       resource.Name,
				APIVersion: resource.APIVersion,
			},
		})
	}

	if timeout == nil {
		timeout = &metav1.Duration{Duration: 240 * time.Hour}
	}

	pipelineRun := &pipelineapi.PipelineRun{
		TypeMeta: metav1.TypeMeta{
			APIVersion: syntax.TektonAPIVersion,
			Kind:       "PipelineRun",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: util.MergeMaps(labels),
		},
		Spec: pipelineapi.PipelineRunSpec{
			ServiceAccountName: serviceAccount,
			PipelineRef: pipelineapi.PipelineRef{
				Name:       name,
				APIVersion: apiVersion,
			},
			Resources: resourceBindings,
			Params:    pipelineParams,
			// TODO: We shouldn't have to set a default timeout in the first place. See https://github.com/tektoncd/pipeline/issues/978
			Timeout: timeout,
			PodTemplate: pipelineapi.PodTemplate{
				Affinity:    affinity,
				Tolerations: tolerations,
			},
		},
	}

	return pipelineRun
}

// ApplyPipelineRun lazily creates a Tekton PipelineRun.
func ApplyPipelineRun(tektonClient tektonclient.Interface, ns string, run *v1alpha1.PipelineRun) (*v1alpha1.PipelineRun, error) {
	resourceName := run.Name
	resourceInterface := tektonClient.TektonV1alpha1().PipelineRuns(ns)

	answer, err := resourceInterface.Create(run)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create PipelineRun %s", resourceName)
	}
	return answer, nil
}

// CreateOrUpdatePipeline lazily creates a Tekton Pipeline for the given git repository, branch and context
func CreateOrUpdatePipeline(tektonClient tektonclient.Interface, ns string, created *v1alpha1.Pipeline) (*v1alpha1.Pipeline, error) {
	resourceName := created.Name
	resourceInterface := tektonClient.TektonV1alpha1().Pipelines(ns)

	answer, err := resourceInterface.Create(created)
	if err == nil {
		return answer, nil
	}

	answer, err = resourceInterface.Get(resourceName, metav1.GetOptions{})
	if err != nil {
		return answer, errors.Wrapf(err, "failed to get Pipeline %s after failing to create a new one", resourceName)
	}

	if !reflect.DeepEqual(&created.Spec, &answer.Spec) || !reflect.DeepEqual(created.Labels, answer.Labels) {
		answer.Annotations = util.MergeMaps(answer.Annotations, created.Annotations)
		answer.Spec = created.Spec
		answer, err = resourceInterface.Update(answer)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to update Pipeline %s", resourceName)
		}
	}
	return answer, nil
}

// PipelineResourceNameFromGitInfo returns the pipeline resource name for the given git repository, branch and context
func PipelineResourceNameFromGitInfo(gitInfo *gits.GitRepository, branch string, context string, pipelineType string) string {
	return PipelineResourceName(gitInfo.Organisation, gitInfo.Name, branch, context, pipelineType)
}

// PipelineResourceName returns the pipeline resource name for the given git org, repo name, branch and context. It will always be unique.
func PipelineResourceName(organisation string, name string, branch string, context string, pipelineType string) string {
	return possiblyUniquePipelineResourceName(organisation, name, branch, context, pipelineType, true)
}

// possiblyUniquePipelineResourceName returns the pipeline resource name for the given git org, repo name, branch and context, possibly forcing it to be unique
func possiblyUniquePipelineResourceName(organisation string, name string, branch string, context string, pipelineType string, forceUnique bool) string {
	dirtyName := organisation + "-" + name + "-" + branch
	if context != "" {
		dirtyName += "-" + context
	}

	if pipelineType == MetaPipeline.String() {
		dirtyName = pipelineType + "-" + dirtyName
	}
	resourceName := naming.ToValidNameTruncated(dirtyName, 31)

	if forceUnique {
		return resourceName + "-" + rand.String(5)
	}
	return resourceName
}

// ApplyPipeline applies the tasks and pipeline to the cluster
// and creates and applies a PipelineResource for their source repo and a pipelineRun
// to execute them.
func ApplyPipeline(jxClient versioned.Interface, tektonClient tektonclient.Interface, crds *CRDWrapper, ns string, activityKey *kube.PromoteStepActivityKey) error {
	info := util.ColorInfo

	var activityOwnerReference *metav1.OwnerReference

	if activityKey != nil {
		activity, _, err := activityKey.GetOrCreate(jxClient, crds.Pipeline().Namespace)
		if err != nil {
			return err
		}

		activityOwnerReference = &metav1.OwnerReference{
			APIVersion: jenkinsio.GroupAndVersion,
			Kind:       "PipelineActivity",
			Name:       activity.Name,
			UID:        activity.UID,
		}
	}

	for _, resource := range crds.Resources() {
		if activityOwnerReference != nil {
			resource.OwnerReferences = []metav1.OwnerReference{*activityOwnerReference}
		}
		_, err := CreateOrUpdateSourceResource(tektonClient, ns, resource)
		if err != nil {
			return errors.Wrapf(err, "failed to create/update PipelineResource %s in namespace %s", resource.Name, ns)
		}
		if resource.Spec.Type == pipelineapi.PipelineResourceTypeGit {
			gitURL := activityKey.GitInfo.HttpCloneURL()
			log.Logger().Infof("upserted PipelineResource %s for the git repository %s", info(resource.Name), info(gitURL))
		} else {
			log.Logger().Infof("upserted PipelineResource %s", info(resource.Name))
		}
	}

	for _, task := range crds.Tasks() {
		if activityOwnerReference != nil {
			task.OwnerReferences = []metav1.OwnerReference{*activityOwnerReference}
		}
		_, err := CreateOrUpdateTask(tektonClient, ns, task)
		if err != nil {
			return errors.Wrapf(err, "failed to create/update the task %s in namespace %s", task.Name, ns)
		}
		log.Logger().Infof("upserted Task %s", info(task.Name))
	}

	if activityOwnerReference != nil {
		crds.Pipeline().OwnerReferences = []metav1.OwnerReference{*activityOwnerReference}
	}

	pipeline, err := CreateOrUpdatePipeline(tektonClient, ns, crds.Pipeline())
	if err != nil {
		return errors.Wrapf(err, "failed to create/update the pipeline in namespace %s", ns)
	}
	log.Logger().Infof("upserted Pipeline %s", info(pipeline.Name))

	pipelineOwnerReference := metav1.OwnerReference{
		APIVersion: syntax.TektonAPIVersion,
		Kind:       "pipeline",
		Name:       pipeline.Name,
		UID:        pipeline.UID,
	}

	crds.structure.OwnerReferences = []metav1.OwnerReference{pipelineOwnerReference}

	_, err = ApplyPipelineRun(tektonClient, ns, crds.PipelineRun())
	if err != nil {
		return errors.Wrapf(err, "failed to create the pipelineRun in namespace %s", ns)
	}
	log.Logger().Infof("created PipelineRun %s", info(crds.PipelineRun().Name))

	if crds.Structure() != nil {
		crds.Structure().PipelineRunRef = &crds.PipelineRun().Name

		structuresClient := jxClient.JenkinsV1().PipelineStructures(ns)

		// Reset the structure name to be the run's name and set the PipelineRef and PipelineRunRef
		if crds.Structure().PipelineRef == nil {
			crds.Structure().PipelineRef = &pipeline.Name
		}
		crds.Structure().Name = crds.PipelineRun().Name
		crds.Structure().PipelineRunRef = &crds.PipelineRun().Name

		if _, structErr := structuresClient.Create(crds.Structure()); structErr != nil {
			return errors.Wrapf(structErr, "failed to create the PipelineStructure in namespace %s", ns)
		}
		log.Logger().Infof("created PipelineStructure %s", info(crds.Structure().Name))
	}

	return nil
}

// StructureForPipelineRun finds the PipelineStructure for the given PipelineRun, trying its name first and then its
// Pipeline name, returning an error if no PipelineStructure can be found.
func StructureForPipelineRun(jxClient versioned.Interface, ns string, run *pipelineapi.PipelineRun) (*v1.PipelineStructure, error) {
	// Use the Pipeline name for this run.
	pipelineName := run.Labels[pipeline.GroupName+pipeline.PipelineLabelKey]
	// Fall back on the PipelineRef.Name if there isn't a label.
	if pipelineName == "" {
		pipelineName = run.Spec.PipelineRef.Name
	}
	// If we still have no name, error out.
	if pipelineName == "" {
		return nil, fmt.Errorf("couldn't find a Pipeline name for PipelineRun %s", run.Name)
	}
	structure, err := jxClient.JenkinsV1().PipelineStructures(ns).Get(pipelineName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "getting PipelineStructure with Pipeline name %s for PipelineRun %s", pipelineName, run.Name)
	}
	return structure, nil
}

// PipelineRunIsNotPending returns true if the PipelineRun has completed or has running steps.
func PipelineRunIsNotPending(pr *pipelineapi.PipelineRun) bool {
	if pr.Status.CompletionTime != nil {
		return true
	}
	if len(pr.Status.TaskRuns) > 0 {
		for _, v := range pr.Status.TaskRuns {
			if v.Status != nil {
				for _, stepState := range v.Status.Steps {
					if stepState.Waiting == nil || stepState.Waiting.Reason == "PodInitializing" {
						return true
					}
				}
			}
		}
	}
	return false
}

// PipelineRunIsComplete returns true if the PipelineRun has completed or has running steps.
func PipelineRunIsComplete(pr *pipelineapi.PipelineRun) bool {
	if pr.Status.CompletionTime != nil {
		return true
	}
	return false
}

// CancelPipelineRun cancels a Pipeline
func CancelPipelineRun(tektonClient tektonclient.Interface, ns string, pr *pipelineapi.PipelineRun) error {
	pr.Spec.Status = pipelineapi.PipelineRunSpecStatusCancelled
	_, err := tektonClient.TektonV1alpha1().PipelineRuns(ns).Update(pr)
	if err != nil {
		return errors.Wrapf(err, "failed to update PipelineRun %s in namespace %s to mark it as cancelled", pr.Name, ns)
	}
	return nil
}
