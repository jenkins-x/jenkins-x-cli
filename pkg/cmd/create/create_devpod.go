package create

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/auth"

	"github.com/jenkins-x/jx/pkg/cmd/step/git/credentials"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/rsh"
	"github.com/jenkins-x/jx/pkg/cmd/sync"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/gits"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/serviceaccount"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	optionRequestCPU    = "request-cpu"
	optionRequestMemory = "request-memory"
	devPodGoPath        = "/workspace"
	devPodContainerName = "devpod"
)

var (
	createDevPodLong = templates.LongDesc(`
		Creates a new DevPod

		For more documentation see: [https://jenkins-x.io/developing/devpods/](https://jenkins-x.io/developing/devpods/)

`)

	createDevPodExample = templates.Examples(`
		# creates a new DevPod asking the user for the label to use
		jx create devpod

		# creates a new Maven DevPod 
		jx create devpod -l maven
	`)
)

// CreateDevPodResults the results of running the command
type CreateDevPodResults struct {
	TheaServiceURL string
	ExposePortURLs []string
	PodName        string
}

// CreateDevPodOptions the options for the create spring command
type CreateDevPodOptions struct {
	options.CreateOptions
	opts.CommonDevPodOptions

	Label           string
	Suffix          string
	WorkingDir      string
	RequestCpu      string
	RequestMemory   string
	Dir             string
	Reuse           bool
	Sync            bool
	Ports           []int
	AutoExpose      bool
	Persist         bool
	ImportURL       string
	Import          bool
	TempDir         bool
	Theia           bool
	ShellCmd        string
	DockerRegistry  string
	TillerNamespace string
	ServiceAccount  string
	PullSecrets     string

	GitCredentials credentials.StepGitCredentialsOptions

	Results CreateDevPodResults
}

// NewCmdCreateDevPod creates a command object for the "create" command
func NewCmdCreateDevPod(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateDevPodOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
		GitCredentials: credentials.StepGitCredentialsOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "devpod",
		Short:   "Creates a DevPod for running builds and tests inside the cluster",
		Aliases: []string{"dpod", "buildpod"},
		Long:    createDevPodLong,
		Example: createDevPodExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Label, opts.OptionLabel, "l", "", "The label of the pod template to use")
	cmd.Flags().StringVarP(&options.Suffix, "suffix", "s", "", "The suffix to append the pod name")
	cmd.Flags().StringVarP(&options.WorkingDir, "working-dir", "w", "", "The working directory of the DevPod")
	cmd.Flags().StringVarP(&options.RequestCpu, optionRequestCPU, "c", "1", "The request CPU of the DevPod")
	cmd.Flags().StringVarP(&options.RequestMemory, optionRequestMemory, "m", "512Mi", "The request Memory of the DevPod")
	cmd.Flags().BoolVarP(&options.Reuse, "reuse", "", true, "Reuse an existing DevPod if a suitable one exists. The DevPod will be selected based on the label (or current working directory)")
	cmd.Flags().BoolVarP(&options.Sync, "sync", "", false, "Also synchronise the local file system into the DevPod")
	cmd.Flags().IntSliceVarP(&options.Ports, "ports", "p", []int{}, "Container ports exposed by the DevPod")
	cmd.Flags().BoolVarP(&options.AutoExpose, "auto-expose", "", false, "Automatically expose useful ports via ingresses such as the ide port, debug port, as well as any ports specified using --ports")
	cmd.Flags().BoolVarP(&options.Persist, "persist", "", false, "Persist changes made to the DevPod. Cannot be used with --sync")
	cmd.Flags().StringVarP(&options.ImportURL, "import-url", "u", "", "Clone a Git repository into the DevPod. Cannot be used with --sync")
	cmd.Flags().BoolVarP(&options.Import, "import", "", true, "Detect if there is a Git repository in the current directory and attempt to clone it into the DevPod. Ignored if used with --sync")
	cmd.Flags().BoolVarP(&options.TempDir, "temp-dir", "", false, "If enabled and --import-url is supplied then create a temporary directory to clone the source to detect what kind of DevPod to create")
	cmd.Flags().BoolVarP(&options.Theia, "theia", "", false, "If enabled use Eclipse Theia as the web based IDE")
	cmd.Flags().StringVarP(&options.ShellCmd, "shell", "", "", "The name of the shell to invoke in the DevPod. If nothing is specified it will use 'bash'")
	cmd.Flags().StringVarP(&options.DockerRegistry, "docker-registry", "", "", "The Docker registry to use within the DevPod. If not specified, default to the built-in registry or $DOCKER_REGISTRY")
	cmd.Flags().StringVarP(&options.TillerNamespace, "tiller-namespace", "", "", "The optional tiller namespace to use within the DevPod.")
	cmd.Flags().StringVarP(&options.ServiceAccount, "service-account", "", "", "The ServiceAccount name used for the DevPod")
	cmd.Flags().StringVarP(&options.PullSecrets, optionPullSecrets, "", "", "A list of Kubernetes secret names that will be attached to the service account (e.g. foo, bar, baz)")

	options.AddCommonDevPodFlags(cmd)
	return cmd
}

// Run implements this command
func (o *CreateDevPodOptions) Run() error {
	if o.Persist && o.Sync {
		return errors.New("cannot specify --persist and --sync")
	}

	if o.ImportURL != "" && o.Sync {
		return errors.New("cannot specify --import-url && --sync")
	}

	client, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return errors.Wrap(err, "creating kubernetes client")
	}
	ns, _, err := kube.GetDevNamespace(client, curNs)
	if err != nil {
		return errors.Wrap(err, "getting the dev namespce")
	}

	if o.AutoExpose && !o.isAutoExposeSecure(client, ns) {
		return errors.New("skipping creating the DevPod because auto-expose is insecure")
	}

	userName, err := o.GetUsername(o.Username)
	if err != nil {
		return errors.Wrap(err, "getting the current user")
	}

	dir := o.Dir
	importURL := o.ImportURL
	if o.TempDir {
		if dir != "" {
			return fmt.Errorf("you cannot specify --dir and --temp-dir")
		}
		dir, err = ioutil.TempDir("", username+"-")
		if err != nil {
			return errors.Wrapf(err, "creating a temp dir")
		}
		defer os.RemoveAll(dir)
		o.Dir = dir
	}
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "getting the working directory")
		}
	}
	if importURL == "" {
		gitInfo, err := o.FindGitInfo(dir)
		if err != nil {
			log.Logger().Warnf("could not find git URL in dir %s due to: %s", dir, err.Error())
		} else {
			importURL = gitInfo.HttpCloneURL()
		}
	}
	label := o.Label
	workingDir := o.WorkingDir
	if workingDir == "" {
		workingDir = "/workspace"

		if o.Sync {
			// lets check for GOPATH stuff if we are in --sync mode so that we sync into gopath
			gopath := os.Getenv("GOPATH")
			if gopath != "" {
				rel, err := filepath.Rel(gopath, dir)
				if err == nil && rel != "" {
					workingDir = filepath.Join(devPodGoPath, rel)
				}
			}
		}
	}

	setupWorkspaceCommand := "jx step create devpod workspace"
	create := true
	var pod *corev1.Pod
	var editEnv *v1.Environment
	name := ""
	podResources := client.CoreV1().Pods(ns)
	var exposeServicePorts []int

	gitLabelKey := ""
	gitLabelValue := ""
	if importURL != "" && o.Reuse {
		gitInfo, err := gits.ParseGitURL(importURL)
		if err != nil {
			log.Logger().Warnf("could not parse the git URL %s: %s", importURL, err.Error())
		} else {
			gitLabelKey = fmt.Sprintf("%s-%s-%s-%s", kube.LabelDevPodGitPrefix, gitInfo.Host, gitInfo.Organisation, gitInfo.Name)
			gitLabelValue = naming.ToValidNameWithDots(importURL)
			// lets query to see if there is a DevPod already for this URL
			matchLabels := map[string]string{
				kube.LabelDevPodUsername: userName,
				gitLabelKey:              gitLabelValue,
			}
			pod, err = o.findDevPodBySelector(podResources, matchLabels, dir)
			if err != nil {
				return errors.Wrapf(err, "finding DevPod by selector: %v", matchLabels)
			}
			if pod != nil {
				name = pod.Name
				create = false
			}
		}
	}
	idePort := int32(3000)
	ideContainerName := "ide"
	if create {
		if o.TempDir && importURL != "" {
			o.NotifyProgress(opts.LogInfo, "cloning git URL: %s\n", importURL)

			err = o.Git().ShallowClone(dir, importURL, "master", "")
			if err != nil {
				return errors.Wrapf(err, "git cloning shallow: %s into dir %s", importURL, dir)
			}
		}

		podTemplates, err := kube.LoadPodTemplates(client, ns)
		if err != nil {
			return errors.Wrapf(err, "loading the pod templates from namesapce %q", ns)
		}
		podTemplateKeys := map[string]string{}
		for k := range podTemplates {
			podTemplateKeys[k] = k
		}
		labels := util.SortedMapKeys(podTemplateKeys)

		if label == "" {
			label, err = o.guessDevPodLabel(dir, labels)
			if err != nil {
				return errors.Wrap(err, "guessing the DevPod label")
			}

			if label == "javascript" {
				label = "nodejs"
			}
		}
		if label == "" {
			label, err = util.PickName(labels, "Pick which kind of DevPod you wish to create: ", "", o.GetIOFileHandles())
			if err != nil {
				return errors.Wrap(err, "picking the kind of DevPod to create")
			}
		}
		pod = podTemplates[label]
		if pod == nil {
			return util.InvalidOption(opts.OptionLabel, label, labels)
		}

		editEnv, err = o.getOrCreateEditEnvironment()
		if err != nil {
			return errors.Wrap(err, "getting or creating the edit environment")
		}

		// If the user passed in Image Pull Secrets, patch them in to the edit env's default service account
		if o.PullSecrets != "" {
			imagePullSecrets := strings.Fields(o.PullSecrets)
			err = serviceaccount.PatchImagePullSecrets(client, editEnv.Spec.Namespace, "default", imagePullSecrets)
			if err != nil {
				return fmt.Errorf("failed to add pull secrets %s to service account default in namespace %s: %v", imagePullSecrets, editEnv.Spec.Namespace, err)
			}
		}

		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}

		name = naming.ToValidName(userName + "-" + label)
		if o.Suffix != "" {
			name += "-" + o.Suffix
		}
		names, err := kube.GetPodNames(client, ns, "")
		if err != nil {
			return errors.Wrap(err, "getting pod names")
		}

		name = uniquePodName(names, name)
		o.Results.PodName = name

		pod.Name = name
		pod.Labels[kube.LabelPodTemplate] = label
		pod.Labels[kube.LabelDevPodName] = name
		pod.Labels[kube.LabelDevPodUsername] = userName
		if gitLabelKey != "" && gitLabelValue != "" {
			pod.Labels[gitLabelKey] = gitLabelValue
		}

		if len(pod.Spec.Containers) == 0 {
			return fmt.Errorf("no containers specified for label %s with pod: %#v", label, pod)
		}
		container1 := &pod.Spec.Containers[0]

		// lets use a canonical name for the devpod container
		container1.Name = devPodContainerName

		workspaceVolumeName := "workspace-volume"
		// lets remove the default workspace volume as we don't need it
		for i, v := range pod.Spec.Volumes {
			if v.Name == workspaceVolumeName {
				pod.Spec.Volumes = append(pod.Spec.Volumes[:i], pod.Spec.Volumes[i+1:]...)
				break
			}
		}
		for ci, c := range pod.Spec.Containers {
			for i, v := range c.VolumeMounts {
				if v.Name == workspaceVolumeName {
					pod.Spec.Containers[ci].VolumeMounts = append(c.VolumeMounts[:i], c.VolumeMounts[i+1:]...)
					break
				}
			}
		}

		// Trying to reuse workspace-volume as a name seems to prevent us modifying the volumes!
		workspaceVolumeName = "ws-volume"
		var workspaceVolume corev1.Volume
		workspaceClaimName := fmt.Sprintf("%s-pvc", pod.Name)
		workspaceVolumeMount := corev1.VolumeMount{
			Name:      workspaceVolumeName,
			MountPath: "/workspace",
		}
		if o.Persist {
			workspaceVolume = corev1.Volume{
				Name: workspaceVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: workspaceClaimName,
					},
				},
			}
		} else {
			workspaceVolume = corev1.Volume{
				Name: workspaceVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}
		}

		if pod.Spec.ServiceAccountName == "" {
			sa := o.ServiceAccount
			if sa == "" {
				prow, err := o.IsProw()
				if err != nil {
					return errors.Wrap(err, "checking if prow is active")
				}

				settings, err := o.TeamSettings()
				if err != nil {
					return errors.Wrap(err, "getting the team settings")
				}

				sa = "jenkins"
				if settings.IsJenkinsXPipelines() {
					sa = "tekton-bot"
				} else if prow {
					sa = "knative-build-bot"
				}
			}
			pod.Spec.ServiceAccountName = sa
		}

		if !o.Sync {
			pod.Spec.Volumes = append(pod.Spec.Volumes, workspaceVolume)
			container1.VolumeMounts = append(container1.VolumeMounts, workspaceVolumeMount)

			cpuLimit, _ := resource.ParseQuantity("400m")
			cpuRequest, _ := resource.ParseQuantity("200m")
			memoryLimit, _ := resource.ParseQuantity("1Gi")
			memoryRequest, _ := resource.ParseQuantity("128Mi")

			// disable input for replacing the version stream git repo
			batch := o.BatchMode
			o.BatchMode = true
			resolver, err := o.GetVersionResolver()
			if err != nil {
				return errors.Wrap(err, "getting the version stream resolver")
			}
			o.BatchMode = batch

			// web IDEs  won't work in --sync mode as we can't share a volume
			if o.Theia {
				image, err := resolver.ResolveDockerImage("theiaide/theia-full")
				if err != nil {
					return errors.Wrap(err, "resolving the container image for Theia IDE")
				}
				editorContainer := corev1.Container{
					Name:  ideContainerName,
					Image: image,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: idePort,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu":    cpuLimit,
							"memory": memoryLimit,
						},
						Requests: corev1.ResourceList{
							"cpu":    cpuRequest,
							"memory": memoryRequest,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						workspaceVolumeMount,
					},
					LivenessProbe: &corev1.Probe{
						InitialDelaySeconds: 60,
						PeriodSeconds:       10,
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Port: intstr.FromInt(int(idePort)),
							},
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: func(i int64) *int64 { return &i }(0),
					},
					Command: []string{"yarn", "theia", "start", "/workspace", "--hostname=0.0.0.0"},
				}
				pod.Spec.Containers = append(pod.Spec.Containers, editorContainer)
			} else {
				setupWorkspaceCommand += " --vscode"
				idePort = 8443
				image, err := resolver.ResolveDockerImage("codercom/code-server")
				if err != nil {
					return errors.Wrap(err, "resolving the container image for VS Code")
				}
				editorContainer := corev1.Container{
					Name:  ideContainerName,
					Image: image,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: idePort,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: workingDir + "/idehome",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"cpu":    cpuLimit,
							"memory": memoryLimit,
						},
						Requests: corev1.ResourceList{
							"cpu":    cpuRequest,
							"memory": memoryRequest,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						workspaceVolumeMount,
					},
					LivenessProbe: &corev1.Probe{
						InitialDelaySeconds: 60,
						PeriodSeconds:       10,
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Port: intstr.FromInt(int(idePort)),
							},
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: func(i int64) *int64 { return &i }(0),
					},
					Args: []string{"--allow-http", "--no-auth", workingDir},
				}
				pod.Spec.Containers = append(pod.Spec.Containers, editorContainer)
			}
		}

		if o.RequestCpu != "" {
			q, err := resource.ParseQuantity(o.RequestCpu)
			if err != nil {
				return util.InvalidOptionError(optionRequestCPU, o.RequestCpu, err)
			}
			container1.Resources.Requests[corev1.ResourceCPU] = q
		}

		if o.RequestMemory != "" {
			q, err := resource.ParseQuantity(o.RequestMemory)
			if err != nil {
				return util.InvalidOptionError(optionRequestMemory, o.RequestMemory, err)
			}
			container1.Resources.Requests[corev1.ResourceMemory] = q
		}

		//Set the devpods gopath properly
		container1.Env = append(container1.Env, corev1.EnvVar{
			Name:  "GOPATH",
			Value: devPodGoPath,
		})
		pod.Annotations[kube.AnnotationWorkingDir] = workingDir
		if importURL != "" {
			gitURLs := pod.Annotations[kube.AnnotationGitURLs]
			if gitURLs == "" {
				gitURLs = importURL
			} else {
				const separator = "\n"
				slice := strings.Split(gitURLs, separator)
				if util.StringArrayIndex(slice, importURL) < 0 {
					slice = append(slice, importURL)
					gitURLs = strings.Join(slice, separator)
				}
			}
			pod.Annotations[kube.AnnotationGitURLs] = gitURLs
		}
		if o.Sync {
			pod.Annotations[kube.AnnotationLocalDir] = dir
		}

		container1.Env = append(container1.Env, corev1.EnvVar{
			Name:  "WORK_DIR",
			Value: workingDir,
		})
		container1.Stdin = true

		// If a Docker registry override was passed in, set it as an env var.
		if o.DockerRegistry != "" {
			container1.Env = append(container1.Env, corev1.EnvVar{
				Name:  "DOCKER_REGISTRY",
				Value: o.DockerRegistry,
			})
		}

		// If a tiller namespace was passed in, set it as an env var.
		if o.TillerNamespace != "" {
			container1.Env = append(container1.Env, corev1.EnvVar{
				Name:  "TILLER_NAMESPACE",
				Value: o.TillerNamespace,
			})
		}

		if editEnv != nil {
			container1.Env = append(container1.Env, corev1.EnvVar{
				Name:  "SKAFFOLD_DEPLOY_NAMESPACE",
				Value: editEnv.Spec.Namespace,
			})
		}

		// Assign the container the ports provided as input
		for _, port := range o.Ports {
			cp := corev1.ContainerPort{
				Name:          fmt.Sprintf("port-%d", port),
				ContainerPort: int32(port),
			}
			container1.Ports = append(container1.Ports, cp)
		}

		// Assign the container the ports provided automatically
		exposeServicePorts = o.Ports
		if portsStr, ok := pod.Annotations["jenkins-x.io/devpodPorts"]; ok {
			ports := strings.Split(portsStr, ", ")
			for _, portStr := range ports {
				port, _ := strconv.Atoi(portStr)
				exposeServicePorts = append(exposeServicePorts, port)
				cp := corev1.ContainerPort{
					Name:          fmt.Sprintf("port-%d", port),
					ContainerPort: int32(port),
				}
				container1.Ports = append(container1.Ports, cp)
			}
		}

		if o.Reuse {
			matchLabels := map[string]string{
				kube.LabelPodTemplate:    label,
				kube.LabelDevPodUsername: userName,
			}
			foundPod, err := o.findDevPodBySelector(podResources, matchLabels, dir)
			if err != nil {
				return err
			}
			if foundPod != nil {
				pod = foundPod
				name = pod.Name
				create = false

				// lets add the label if its not already
				if gitLabelKey != "" && gitLabelValue != "" && pod.Labels[gitLabelKey] != gitLabelValue {
					pod.Labels[gitLabelKey] = gitLabelValue
					_, err = podResources.Update(pod)
					if err != nil {
						log.Logger().Warnf("Failed to update label of pod %s: %s", pod.Name, err.Error())
					}
				}
			}
		}
	}

	ideServiceName := name + "-ide"
	if create {
		o.NotifyProgress(opts.LogInfo, "Creating a DevPod of label: %s\n", util.ColorInfo(label))
		_, err = podResources.Create(pod)
		if err != nil {
			return fmt.Errorf("failed to create pod %s\npod: %#v", err, pod)
		}

		if o.AutoExpose {
			err := o.ensureEditEnvironmentHasExposeController(editEnv)
			if err != nil {
				return errors.Wrap(err, "ensuring that edit environment has expose-controller")
			}
		}

		o.NotifyProgress(opts.LogInfo, "Created pod %s - waiting for it to be ready...\n", util.ColorInfo(name))

		err = kube.WaitForPodNameToBeReady(client, ns, name, time.Hour)
		if err != nil {
			return errors.Wrapf(err, "waiting for POD %q to be ready", name)
		}

		// Get the pod UID
		pod, err = client.CoreV1().Pods(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "getting the POD %q", name)
		}

		// Create PVC if needed
		if o.Persist {
			storageRequest, _ := resource.ParseQuantity("2Gi")
			workspaceClaimName := fmt.Sprintf("%s-pvc", pod.Name)
			pvc := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: workspaceClaimName,
					OwnerReferences: []metav1.OwnerReference{
						kube.PodOwnerRef(pod),
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": storageRequest,
						},
					},
				},
			}
			_, err = client.CoreV1().PersistentVolumeClaims(curNs).Create(&pvc)
			if err != nil {
				return errors.Wrapf(err, "creating the persistent volume claim %q in namespace %q", pvc.Name, curNs)
			}
		}

		// Create services
		var addedServices []string

		svcAnotations := map[string]string{}
		if o.AutoExpose {
			svcAnotations = map[string]string{
				"fabric8.io/expose": "true",
			}
		}
		// Create a service for every port we expose
		if len(exposeServicePorts) > 0 {
			for _, port := range exposeServicePorts {
				portName := fmt.Sprintf("%s-%d", pod.Name, port)
				servicePorts := []corev1.ServicePort{
					{
						Name:       portName,
						Port:       int32(80),
						TargetPort: intstr.FromInt(port),
					},
				}
				service := corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: svcAnotations,
						Name:        fmt.Sprintf("%s-port-%d", pod.Name, port),
						OwnerReferences: []metav1.OwnerReference{
							kube.PodOwnerRef(pod),
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: servicePorts,
						Selector: map[string]string{
							"jenkins.io/devpod": pod.Name,
						},
					},
				}
				_, err = client.CoreV1().Services(curNs).Create(&service)
				if err != nil {
					return errors.Wrapf(err, "creating service %q in namespace %q", service.Name, curNs)
				}
				addedServices = append(addedServices, service.Name)
			}
		}
		if !o.Sync {
			// Create a service for the IDE
			ideService := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: svcAnotations,
					Name:        ideServiceName,
					OwnerReferences: []metav1.OwnerReference{
						kube.PodOwnerRef(pod),
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       ideServiceName,
							Port:       80,
							TargetPort: intstr.FromInt(int(idePort)),
						},
					},
					Selector: map[string]string{
						"jenkins.io/devpod": pod.Name,
					},
				},
			}
			_, err = client.CoreV1().Services(curNs).Create(&ideService)
			if err != nil {
				return errors.Wrapf(err, "creating the service %q for IDE in namespace %q", ideService.Name, curNs)
			}
			addedServices = append(addedServices, ideServiceName)
		}

		if len(addedServices) > 0 && o.AutoExpose {
			err = o.updateExposeController(client, ns, ns, addedServices...)
			if err != nil {
				return errors.Wrapf(err, "updating the expose controller in namespace %s", ns)
			}
		}
	}

	o.NotifyProgress(opts.LogInfo, "Pod %s is now ready!\n", util.ColorInfo(pod.Name))
	log.Logger().Infof("You can open other shells into this DevPod via %s", util.ColorInfo("jx create devpod"))

	if !o.Sync {
		if o.AutoExpose {
			ideServiceURL, err := services.FindServiceURL(client, curNs, ideServiceName)
			if err != nil {
				return errors.Wrapf(err, "finding the URL for service %q", ideServiceName)
			}
			if ideServiceURL != "" {
				pod, err = client.CoreV1().Pods(curNs).Get(name, metav1.GetOptions{})
				if err != nil {
					return errors.Wrapf(err, "getting the POD %q in namespace %q", name, curNs)
				}
				pod.Annotations["jenkins-x.io/devpod_IDE_URL"] = ideServiceURL
				pod, err = client.CoreV1().Pods(curNs).Update(pod)
				if err != nil {
					return errors.Wrapf(err, "updating the POD %q in namespace %q", name, curNs)
				}
				log.Logger().Infof("\nYou can edit your app using the Web IDE at: %s", util.ColorInfo(ideServiceURL))
				o.Results.TheaServiceURL = ideServiceURL
			} else {
				o.NotifyProgress(opts.LogWarning, "Could not find service with name %s in namespace %s\n", ideServiceName, curNs)
			}
		} else {
			exposeCmd := fmt.Sprintf("kubectl port-forward svc/%s 8080:80", ideServiceName)
			log.Logger().Info("\nYou can access the Web IDE to edit your app with command:")
			log.Logger().Infof("* %s", util.ColorInfo(exposeCmd))
		}
	}

	if o.AutoExpose {
		exposePortServices, err := services.GetServiceNames(client, curNs, fmt.Sprintf("%s-port-", pod.Name))
		if err != nil {
			return errors.Wrapf(err, "getting the exposed services for POD %q in namespace %q", pod.Name, curNs)
		}
		var exposePortURLs []string
		for _, svcName := range exposePortServices {
			u, err := services.GetServiceURLFromName(client, svcName, curNs)
			if err != nil {
				return errors.Wrapf(err, "getting the service URL from service name %q in namespace %q", svcName, curNs)
			}
			exposePortURLs = append(exposePortURLs, u)
		}
		if len(exposePortURLs) > 0 {
			log.Logger().Infof("\nYou can access the DevPod from your browser via the following URLs:")
			for _, u := range exposePortURLs {
				log.Logger().Infof("* %s", util.ColorInfo(u))
			}
			log.Logger().Info("")

			o.Results.ExposePortURLs = exposePortURLs
		}
	} else {
		exposePortServices, err := services.GetServiceNames(client, curNs, fmt.Sprintf("%s-port-", pod.Name))
		if err != nil {
			return errors.Wrapf(err, "getting the exposed services for POD %q in namesapce %q", pod.Name, curNs)
		}
		localPort := 8081
		log.Logger().Info("\nYou can access the DevPod locally with the following commands:")
		for _, svcName := range exposePortServices {
			exposeCmd := fmt.Sprintf("kubectl port-forward svc/%s %d:80", svcName, localPort)
			log.Logger().Infof("* %s", util.ColorInfo(exposeCmd))
			localPort++
		}
		log.Logger().Info("")
	}

	if o.Sync {
		syncOptions := &sync.SyncOptions{
			CommonOptions: o.CommonOptions,
			Namespace:     ns,
			Pod:           pod.Name,
			Daemon:        true,
			Dir:           dir,
		}
		err = syncOptions.CreateKsync(client, ns, pod.Name, dir, workingDir, userName)
		if err != nil {
			return errors.Wrap(err, "creating ksync")
		}
	}

	var rshExec []string
	if create {
		//  Let install bash-completion to make life better
		o.NotifyProgress(opts.LogInfo, "Attempting to install Bash Completion into DevPod\n")

		rshExec = append(rshExec,
			"if which yum &> /dev/null; then yum install -q -y bash-completion bash-completion-extra; fi",
			"if which apt-get &> /dev/null; then apt-get install -qq bash-completion; fi",
			"mkdir -p ~/.jx", "jx completion bash > ~/.jx/bash", "echo \"source ~/.jx/bash\" >> ~/.bashrc",
		)

		// Only add git secrets to the Theia container when sync flag is missing (otherwise Theia container won't exist)
		if !o.Sync {
			gha, err := o.IsGitHubAppMode()
			if err != nil {
				return err
			}
			// Add Git Secrets to Theia container
			var authConfigSvc auth.ConfigService
			if gha {
				authConfigSvc, err = o.GitAuthConfigServiceGitHubAppMode("")
				if err != nil {
					return errors.Wrap(err, "when creating auth config service using GitAuthConfigServiceGitHubAppMode")
				}
			} else {
				authConfigSvc, err = o.GitAuthConfigService()
				if err != nil {
					return errors.Wrap(err, "when creating auth config service using GitAuthConfigService")
				}
			}
			gitCredentials, err := o.GitCredentials.CreateGitCredentialsFromAuthService(authConfigSvc, gha)
			if err != nil {
				return errors.Wrap(err, "creating git credentials")
			}
			data, err := o.GitCredentials.GitCredentialsFileData(gitCredentials)
			if err != nil {
				return errors.Wrap(err, "creating git credentials")
			}
			theiaRshExec := []string{
				fmt.Sprintf("echo \"%s\" >> ~/.git-credentials", string(data)),
				"git config --global credential.helper store",
			}

			// Configure remote username and email for git
			username, _ := o.Git().Username("")
			email, _ := o.Git().Email("")
			if username != "" {
				theiaRshExec = append(theiaRshExec, fmt.Sprintf("git config --global user.name \"%s\"", username))
			}
			if email != "" {
				theiaRshExec = append(theiaRshExec, fmt.Sprintf("git config --global user.email \"%s\"", email))
			}

			// remove annoying warning
			theiaRshExec = append(theiaRshExec, " git config --global push.default simple")

			options := &rsh.RshOptions{
				CommonOptions: o.CommonOptions,
				Namespace:     ns,
				Pod:           pod.Name,
				DevPod:        true,
				ExecCmd:       strings.Join(theiaRshExec, "&&"),
				Username:      userName,
				Container:     ideContainerName,
			}
			options.Args = []string{}
			err = options.Run()
			if err != nil {
				return errors.Wrapf(err, "opening a remote shell into the DevPod %q", pod.Name)
			}
		}
	}

	if !o.Sync {
		// Try to clone the right Git repo into the DevPod

		// First configure git credentials
		rshExec = append(rshExec, setupWorkspaceCommand, "jx step git credentials", "git config --global credential.helper store")

		// We only honor --import if --sync is not specified
		if o.Import {
			if importURL != "" {
				dir := regexp.MustCompile(`(?m)^.*/(.*)\.git$`).FindStringSubmatch(importURL)[1]
				rshExec = append(rshExec, fmt.Sprintf("if ! [ -d \"%s\" ]; then git clone %s; fi", dir, importURL))
				rshExec = append(rshExec, fmt.Sprintf("cd %s", dir))
			}
		}
	}

	// Only want to shell into the DevPod if the batch flag isn't set
	if !o.BatchMode {
		shellCommand := o.ShellCmd
		if shellCommand == "" {
			shellCommand = rsh.DefaultRshCommand
		}

		rshExec = append(rshExec, shellCommand)
	}

	options := &rsh.RshOptions{
		CommonOptions: o.CommonOptions,
		Namespace:     ns,
		Pod:           pod.Name,
		Container:     devPodContainerName,
		DevPod:        true,
		ExecCmd:       strings.Join(rshExec, " && "),
		Username:      userName,
	}
	options.Args = []string{}
	return options.Run()
}

func (o *CreateDevPodOptions) isAutoExposeSecure(client kubernetes.Interface, ns string) bool {
	ingressConfig, err := kube.GetIngressConfig(client, ns)
	tlsEnabled := false
	if err == nil {
		tlsEnabled = ingressConfig.TLS && ingressConfig.Issuer != ""
	}
	if tlsEnabled {
		return true
	}

	if o.BatchMode {
		return false
	}

	help := "TLS doesn't seem to be enabled in the ingress-config. This setup is insecure to use with a basic auth password."
	message := "Do you want to use an insecure connection to expose the DevPod?"
	confirmed, err := util.Confirm(message, false, help, o.GetIOFileHandles())
	if confirmed && err == nil {
		return true
	}
	return false
}

func (o *CreateDevPodOptions) getOrCreateEditEnvironment() (*v1.Environment, error) {
	var env *v1.Environment

	kubeClient, err := o.KubeClient()
	if err != nil {
		return env, errors.Wrap(err, "creating kubernetes client")
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return env, errors.Wrap(err, "crating jx client")
	}
	userName, err := o.GetUsername(o.Username)
	if err != nil {
		return env, errors.Wrap(err, "getting the current username")
	}
	env, err = kube.EnsureEditEnvironmentSetup(kubeClient, jxClient, ns, userName)
	if err != nil {
		return env, errors.Wrapf(err, "ensuring the edit environment in the namespace %q", ns)
	}
	return env, err
}

func (o *CreateDevPodOptions) ensureEditEnvironmentHasExposeController(env *v1.Environment) error {
	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "getting the kubernetes client")
	}
	// lets ensure that we've installed the exposecontroller service in the namespace
	var flag bool
	editNs := env.Spec.Namespace
	flag, err = kube.IsDeploymentRunning(kubeClient, kube.DeploymentExposecontrollerService, editNs)
	if !flag || err != nil {
		o.NotifyProgress(opts.LogInfo, "Installing the ExposecontrollerService in the namespace: %s\n", util.ColorInfo(editNs))
		releaseName := editNs + "-es"
		err = o.InstallChartWithOptions(helm.InstallChartOptions{
			ReleaseName: releaseName,
			Chart:       kube.ChartExposecontrollerService,
			Version:     "",
			Ns:          editNs,
			HelmUpdate:  true,
			SetValues:   nil,
		})
	}
	if err != nil {
		return errors.Wrapf(err, "installing chart %q in namespace %q", kube.ChartExposecontrollerService, editNs)
	}
	return err
}

func (o *CreateDevPodOptions) guessDevPodLabel(dir string, labels []string) (string, error) {
	root, _, err := o.Git().FindGitConfigDir(o.Dir)
	if err != nil {
		log.Logger().Warnf("Could not find a .git directory: %s", err)
	}
	answer := ""
	if root != "" {
		projectConfig, _, err := config.LoadProjectConfig(root)
		if err != nil {
			return answer, err
		}
		args := &opts.InvokeDraftPack{
			Dir:                     root,
			ProjectConfig:           projectConfig,
			DisableAddFiles:         true,
			DisableJenkinsfileCheck: true,
		}
		answer, err = o.InvokeDraftPack(args)
		if err != nil {
			return answer, errors.Wrapf(err, "discovering the task pack in dir %s", o.Dir)
		}
		if answer != "" {
			return answer, nil
		}
		jenkinsfile := filepath.Join(root, "Jenkinsfile")
		exists, err := util.FileExists(jenkinsfile)
		if err != nil {
			return answer, errors.Wrapf(err, "could not find file: %s", jenkinsfile)
		} else if exists {
			answer, err = FindDevPodLabelFromJenkinsfile(jenkinsfile, labels)
			if err != nil {
				return answer, errors.Wrapf(err, "could not extract the pod template label from file: %s", jenkinsfile)
			}
		}
	}
	return answer, nil
}

// updateExposeController lets update the exposecontroller to expose any new Service resources created for this devpod
func (o *CreateDevPodOptions) updateExposeController(client kubernetes.Interface, devNs string, ns string, serviceNames ...string) error {
	ic, err := kube.GetIngressConfig(client, devNs)
	if err != nil {
		return errors.Wrapf(err, "loading the ingress-config in namespace %s", devNs)
	}

	if err := services.AnnotateServicesWithBasicAuth(client, ns, serviceNames...); err != nil {
		return errors.Wrapf(err, "annotating the exposed services to enable basic authentication")
	}

	if ic.TLS && ic.Issuer != "" {
		if _, err := services.AnnotateServicesWithCertManagerIssuer(client, ns, ic.Issuer, ic.ClusterIssuer, serviceNames...); err != nil {
			return errors.Wrapf(err, "annotating the exposed services with cert-manager issuer")
		}
	}

	err = o.RunExposecontroller(ns, ns, ic, serviceNames...)
	if err != nil {
		return errors.Wrapf(err, "running the expose controller in the namespace %q", ns)
	}
	return nil
}

func (o *CreateDevPodOptions) findDevPodBySelector(podResources v12.PodInterface, matchLabels map[string]string, dir string) (*corev1.Pod, error) {
	var pod *corev1.Pod
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: matchLabels})
	if err != nil {
		return pod, errors.Wrapf(err, "converting label selector: %v", matchLabels)
	}
	options := metav1.ListOptions{
		LabelSelector: selector.String(),
	}
	podsList, err := podResources.List(options)
	if err != nil {
		return pod, errors.Wrap(err, "listing PODs")
	}
	for _, p := range podsList.Items {
		ann := p.Annotations
		if ann == nil {
			ann = map[string]string{}
		}
		// if syncing only match DevPods using the same local dir otherwise ignore any devpods with a local dir sync
		matchDir := dir
		if !o.Sync {
			matchDir = ""
		}
		if p.DeletionTimestamp == nil && ann[kube.AnnotationLocalDir] == matchDir {
			pod = &p
			o.NotifyProgress(opts.LogInfo, "Reusing pod %s - waiting for it to be ready...\n", util.ColorInfo(pod.Name))
			return pod, nil
		}
	}
	return pod, nil
}

// FindDevPodLabelFromJenkinsfile finds pod labels from a Jenkinsfile
func FindDevPodLabelFromJenkinsfile(filename string, labels []string) (string, error) {
	answer := ""
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return answer, errors.Wrapf(err, "reading file %q", filename)
	}
	r, err := regexp.Compile(`label\s+\"(.+)\"`)
	if err != nil {
		return answer, errors.Wrap(err, "compiling the regexp")
	}

	jenkinsXLabelPrefix := "jenkins-"
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		text := strings.TrimSpace(line)
		arr := r.FindStringSubmatch(text)
		if len(arr) > 1 {
			a := arr[1]
			if a != "" {
				if util.StringArrayIndex(labels, a) >= 0 {
					return a, nil
				}
				if strings.HasPrefix(a, jenkinsXLabelPrefix) {
					a = strings.TrimPrefix(a, jenkinsXLabelPrefix)
					if util.StringArrayIndex(labels, a) >= 0 {
						return a, nil
					}
				}
				return answer, fmt.Errorf("cannot find pipeline agent %s in the list of available DevPods: %s", a, strings.Join(labels, ", "))
			}
		}
	}
	return answer, nil
}

func uniquePodName(names []string, prefix string) string {
	count := 1
	for {
		name := prefix
		if count > 1 {
			name += strconv.Itoa(count)
		}
		if util.StringArrayIndex(names, name) < 0 {
			return name
		}
		count++
	}
}
