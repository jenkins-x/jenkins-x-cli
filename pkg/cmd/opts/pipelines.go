package opts

import (
	"strconv"
	"strings"

	"github.com/jenkins-x/jx/pkg/builds"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetLatestPipelineBuildByCRD returns the latest pipeline build
func (o *CommonOptions) GetLatestPipelineBuildByCRD(pipeline string) (string, error) {
	// lets find the latest build number
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return "", err
	}
	pipelines, err := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	buildNumber := 0
	for _, p := range pipelines.Items {
		if p.Spec.Pipeline == pipeline {
			b := p.Spec.Build
			if b != "" {
				n, err := strconv.Atoi(b)
				if err == nil {
					if n > buildNumber {
						buildNumber = n
					}
				}
			}
		}
	}
	if buildNumber > 0 {
		return strconv.Itoa(buildNumber), nil
	}
	return "1", nil
}

// GetPipelineName return the pipeline name
func (o *CommonOptions) GetPipelineName(gitInfo *gits.GitRepository, pipeline string, build string, appName string) (string, string) {
	if pipeline == "" {
		pipeline = o.GetJenkinsJobName()
	}
	if build == "" {
		build = builds.GetBuildNumber()
	}
	if gitInfo != nil && pipeline == "" {
		// lets default the pipeline name from the Git repo
		branch, err := o.Git().Branch(".")
		if err != nil {
			log.Logger().Warnf("Could not find the branch name: %s", err)
		}
		if branch == "" {
			branch = "master"
		}
		pipeline = util.UrlJoin(gitInfo.Organisation, gitInfo.Name, branch)
	}
	if pipeline == "" && appName != "" {
		suffix := appName + "/master"

		// lets try deduce the pipeline name via the app name
		jxClient, ns, err := o.JXClientAndDevNamespace()
		if err == nil {
			pipelineList, err := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
			if err == nil {
				for _, pipelineResource := range pipelineList.Items {
					pipelineName := pipelineResource.Spec.Pipeline
					if strings.HasSuffix(pipelineName, suffix) {
						pipeline = pipelineName
						break
					}
				}
			}
		}
	}
	if pipeline == "" {
		// lets try find
		log.Logger().Warnf("No $JOB_NAME environment variable found so cannot record promotion activities into the PipelineActivity resources in kubernetes")
	} else if build == "" {
		// lets validate and determine the current active pipeline branch
		p, b, err := o.GetLatestPipelineBuild(pipeline)
		if err != nil {
			log.Logger().Warnf("Failed to try detect the current Jenkins pipeline for %s due to %s", pipeline, err)
			build = "1"
		} else {
			pipeline = p
			build = b
		}
	}
	return pipeline, build
}

// getLatestPipelineBuild for the given pipeline name lets try find the Jenkins Pipeline and the latest build
func (o *CommonOptions) GetLatestPipelineBuild(pipeline string) (string, string, error) {
	log.Logger().Infof("pipeline %s", pipeline)
	build := ""
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return pipeline, build, err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return pipeline, build, err
	}
	devEnv, err := kube.GetEnrichedDevEnvironment(kubeClient, jxClient, ns)
	webhookEngine := devEnv.Spec.WebHookEngine
	if webhookEngine == v1.WebHookEngineProw || webhookEngine == v1.WebHookEngineLighthouse {
		return pipeline, build, nil
	}

	jenkins, err := o.JenkinsClient()
	if err != nil {
		return pipeline, build, err
	}
	paths := strings.Split(pipeline, "/")
	job, err := jenkins.GetJobByPath(paths...)
	if err != nil {
		return pipeline, build, err
	}
	build = strconv.Itoa(job.LastBuild.Number)
	return pipeline, build, nil
}
