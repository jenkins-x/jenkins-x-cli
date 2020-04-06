package report

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	stepReportImageVersionLong    = templates.LongDesc(`Creates a report of a set of package versions. This command is typically used inside images to determine what tools are inside.`)
	stepReportImageVersionExample = templates.Examples(`
`)
)

// ImageVersionReport the report
type ImageVersionReport struct {
	Versions []Pair `json:"versions,omitempty"`
	Failures []Pair `json:"failures,omitempty"`
}

// StepReportImageVersionOptions contains the command line flags and other helper objects
type StepReportImageVersionOptions struct {
	StepReportOptions
	FileName              string
	VersionsDir           string
	GitURL                string
	GitBranch             string
	ServiceAccount        string
	UserName              string
	Email                 string
	Folder                string
	Includes              []string
	Excludes              []string
	BackoffLimit          int32
	ActiveDeadlineSeconds int64
	TestImage             string
	StashImage            string
	ContainerDir          string
	BatchSize             int
	NoWait                bool
	NoDeleteJob           bool
	JobWaitTimeout        time.Duration

	Report ImageVersionReport
}

// NewCmdStepReportImageVersion Creates a new Command object
func NewCmdStepReportImageVersion(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepReportImageVersionOptions{
		StepReportOptions: StepReportOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "image versions",
		Short:   "Creates a report of a set of package versions",
		Aliases: []string{"iv"},
		Long:    stepReportImageVersionLong,
		Example: stepReportImageVersionExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.StepReportOptions.AddReportFlags(cmd)

	cmd.Flags().StringVarP(&options.FileName, "name", "n", "", "The name of the file to generate")
	cmd.Flags().StringVarP(&options.VersionsDir, "dir", "d", "", "The dir of the version stream. If not specified it the version stream is cloned")
	cmd.Flags().StringVarP(&options.ServiceAccount, "service-account", "", "tekton-bot", "The Kubernetes ServiceAccount to use to run the Job")
	cmd.Flags().StringVarP(&options.UserName, "username", "u", util.DefaultGitUserName, "The user if using git storage")
	cmd.Flags().StringVarP(&options.Email, "email", "e", util.DefaultGitUserEmail, "The email if using git storage")
	cmd.Flags().Int32VarP(&options.BackoffLimit, "backoff-limit", "l", int32(1), "The backoff limit: how many times to retry the job before considering it failed) to run in the Job")
	cmd.Flags().Int64VarP(&options.ActiveDeadlineSeconds, "active-deadline-seconds", "", int64(60*60*4), "The number of seconds before the Job can be terminated")
	cmd.Flags().StringVarP(&options.GitURL, "git-url", "", "", "The git URL of the project to store the results")
	cmd.Flags().StringVarP(&options.GitBranch, "branch", "", "", "The git branch to store the results")
	cmd.Flags().StringVarP(&options.Folder, "path", "p", "reports/imageVersions", "The output path in the bucket/git repository to store the reports")
	cmd.Flags().StringArrayVarP(&options.Includes, "filter", "f", []string{"gcr.io/jenkinsxio"}, "The text to filter image names")
	cmd.Flags().StringArrayVarP(&options.Excludes, "exclude", "x", []string{"machine-learning"}, "The text strings to exclude on the image names")
	cmd.Flags().StringVarP(&options.TestImage, "test-image", "", "", "Override the actual image used in the container jobs so we can test out changes to the jx steps before they make it into the builders")
	cmd.Flags().StringVarP(&options.StashImage, "stash-image", "", "gcr.io/jenkinsxio/builder-go:latest", "The container image used to stash the results")
	cmd.Flags().StringVarP(&options.ContainerDir, "container-dir", "", "/workspace/reports", "the report directory of the reports")
	cmd.Flags().BoolVarP(&options.NoWait, "no-wait", "", false, "Should we not wait for the Job to complete?")
	cmd.Flags().BoolVarP(&options.NoDeleteJob, "no-delete-job", "", false, "Should we not delete the Job?")
	cmd.Flags().IntVarP(&options.BatchSize, "batch-size", "", 10, "Number of images to process per Job")
	cmd.Flags().DurationVarP(&options.JobWaitTimeout, "wait-timeout", "", 60*time.Minute, "Amount of time to wait for the Job to complete before failing")
	return cmd
}

// Run generates the report
func (o *StepReportImageVersionOptions) Run() error {
	if o.VersionsDir == "" {
		resolver, err := o.GetVersionResolver()
		if err != nil {
			return err
		}
		o.VersionsDir = resolver.VersionsDir
	}
	dir := filepath.Join(o.VersionsDir, "docker")
	exists, err := util.DirExists(dir)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("directory does not exist %s", dir)
	}
	err = o.generateReport(dir)
	if err != nil {
		return err
	}
	return nil
}

func (o *StepReportImageVersionOptions) generateReport(imagesDir string) error {
	m, err := LoadImageMap(imagesDir)
	if err != nil {
		return err
	}

	if len(m.Images) == 0 {
		return fmt.Errorf("no images found")
	}

	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}

	batches := o.batchImages(m.Images, o.BatchSize)

	if len(batches) == 0 {
		images := []string{}
		for k := range m.Images {
			images = append(images, k)
		}
		sort.Strings(images)
		message := fmt.Sprintf("no container images matched includes %s and excludes %s. Have image names: %s",
			strings.Join(o.Includes, " "), strings.Join(o.Excludes, " "), strings.Join(images, ", "))
		log.Logger().Warn(message)
		return fmt.Errorf(message)
	}
	for _, batchImages := range batches {
		id, err := uuid.NewV4()
		if err != nil {
			return err
		}

		volumeMounts := []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
			},
		}
		initContainers := []corev1.Container{}
		containers := []corev1.Container{o.createStashContainer(volumeMounts)}
		counter := 0
		images := []string{}
		for image, versions := range batchImages {
			images = append(images, image)
			for version, source := range versions {
				log.Logger().Infof("processing image %s version %s from %s", image, version, source)

				counter++
				name := "ic" + strconv.Itoa(counter)
				initContainers = append(initContainers, o.createImageVersionContainer(name, image, version, source, volumeMounts))
			}
		}
		if len(containers) == 0 {
			break
		}

		name := naming.ToValidName("jx-report-image-version-" + id.String())
		job := &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: "batch/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
				Labels: map[string]string{
					kube.LabelKind:      "job",
					"jenkins.io/report": "image-versions",
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Now(),
					},
					Spec: corev1.PodSpec{
						InitContainers:     initContainers,
						Containers:         containers,
						RestartPolicy:      corev1.RestartPolicyNever,
						ServiceAccountName: o.ServiceAccount,
						Volumes: []corev1.Volume{
							{
								Name: "workspace",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						ActiveDeadlineSeconds: &o.ActiveDeadlineSeconds,
					},
				},
				BackoffLimit:          &o.BackoffLimit,
				ActiveDeadlineSeconds: &o.ActiveDeadlineSeconds,
			},
		}
		pod, _ := kube.GetCurrentPod(kubeClient, ns)
		if pod != nil && pod.Name != "" {
			job.OwnerReferences = []metav1.OwnerReference{
				kube.PodOwnerRef(pod),
			}
		}

		jobs := kubeClient.BatchV1().Jobs(ns)
		_, err = jobs.Create(job)
		if err != nil {
			data, err2 := yaml.Marshal(job)
			if err2 == nil {
				log.Logger().Warnf("failed to create job %s %s", err.Error(), string(data))
			}
			return errors.Wrapf(err, "failed to create job %s", job.Name)
		}
		log.Logger().Infof("created Job %s", util.ColorInfo(job.Name))

		if o.NoWait {
			return nil
		}
		log.Logger().Infof("waiting for Job %s to complete", util.ColorInfo(job.Name))
		err = kube.WaitForJobToFinish(kubeClient, ns, job.Name, o.JobWaitTimeout, true)
		if err != nil {
			return errors.Wrapf(err, "failed waiting for Job %s to complete", job.Name)
		}
		if o.NoDeleteJob {
			return nil
		}
		if !o.BatchMode {
			if answer, err := util.Confirm(fmt.Sprintf("are you ready to delete the Job %s", job.Name), true,
				"we should delete the job when it is finished", o.GetIOFileHandles()); !answer {
				return err
			}
		}
		err = jobs.Delete(job.Name, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to delete Job %s", job.Name)
		}
	}
	return nil
}

// batchImages batches things up so we don't process too many images per Job
func (o *StepReportImageVersionOptions) batchImages(images map[string]map[string]string, batchSize int) []map[string]map[string]string {
	var answer []map[string]map[string]string
	count := 0
	var lastMap map[string]map[string]string
	for image, v := range images {
		if util.StringContainsAny(image, o.Includes, o.Excludes) {
			if lastMap == nil {
				lastMap = map[string]map[string]string{}
				answer = append(answer, lastMap)
			}
			lastMap[image] = v
			count++
			if batchSize > 0 && count >= batchSize {
				count = 0
				lastMap = nil
			}
		}
	}
	return answer
}

func (o *StepReportImageVersionOptions) createImageVersionContainer(name string, image string, version string, source string, volumeMounts []corev1.VolumeMount) corev1.Container {
	fullImage := image + ":" + version
	if o.TestImage != "" {
		fullImage = o.TestImage
	}
	file := strings.Replace(image, "/", "-", -1) + "-" + version + ".yml"
	args := fmt.Sprintf(` -c reports --to-path="%s"`, o.Folder)
	if o.GitURL != "" {
		args += " --git-url " + o.GitURL
	}
	if o.GitBranch != "" {
		args += " --git-branch " + o.GitBranch
	}
	commands := "mkdir -p " + o.ContainerDir + ";\n" +
		"cd " + o.ContainerDir + ";\n" +
		"jx step report version -n " + file

	return corev1.Container{
		Name:         name,
		Image:        fullImage,
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{commands},
		VolumeMounts: volumeMounts,
	}
}

func (o *StepReportImageVersionOptions) createStashContainer(volumeMounts []corev1.VolumeMount) corev1.Container {
	fullImage := o.StashImage
	args := fmt.Sprintf(` -c reports --to-path="%s"`, o.Folder)
	if o.GitURL != "" {
		args += " --git-url " + o.GitURL
	}
	if o.GitBranch != "" {
		args += " --git-branch " + o.GitBranch
	}
	commands := "cd " + o.ContainerDir + ";\n" +
		"echo created reports;\n" +
		"ls -al;\n" +
		`jx step stash -c reports -p "*.yml"` + args

	envVars := []corev1.EnvVar{}
	if o.GitURL != "" {
		commands = "git config --global credential.helper store;\njx step git validate;\njx step git credentials;\n" + commands
		envVars = append(envVars, corev1.EnvVar{
			Name:  "XDG_CONFIG_HOME",
			Value: "/root",
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "JX_BUILD_NUMBER",
			Value: "1",
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_AUTHOR_NAME",
			Value: o.UserName,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_AUTHOR_EMAIL",
			Value: o.Email,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_COMMITTER_NAME",
			Value: o.UserName,
		})
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_COMMITTER_EMAIL",
			Value: o.Email,
		})
	}
	return corev1.Container{
		Name:         "stash",
		Image:        fullImage,
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{commands},
		Env:          envVars,
		VolumeMounts: volumeMounts,
	}
}

func (o *StepReportImageVersionOptions) getPackageVersion(name string) (string, error) {
	args := []string{"version"}
	switch name {
	case "jx":
		args = []string{"--version"}
	case "kubectl":
		args = append(args, "--client", "--short")
	case "helm":
		args = append(args, "--client", "--short")
	case "helm3":
		args = append(args, "--short")
	}
	version, err := o.GetCommandOutput("", name, args...)

	// lets trim non-numeric prefixes such as for `git version` returning `git version 1.2.3`
	idxs := numberRegex.FindStringIndex(version)
	if len(idxs) > 0 {
		return version[idxs[0]:], err
	}
	return version, err
}
