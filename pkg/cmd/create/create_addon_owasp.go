package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	createAddonOwaspLong = templates.LongDesc(`
		Creates the Owasp dynamic security testing addon
`)

	createAddonOwaspExample = templates.Examples(`
		# Create the owasp addon
		jx create addon owasp-zap
	`)
)

type CreateAddonOwaspOptions struct {
	CreateAddonOptions
	BackoffLimit int32
	Image        string
}

func NewCmdCreateAddonOwasp(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonOwaspOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "owasp-zap",
		Short:   "Create the OWASP Zed Attack Proxy addon for dynamic security checks against running apps",
		Aliases: []string{"env"},
		Long:    createAddonOwaspLong,
		Example: createAddonOwaspExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().Int32VarP(&options.BackoffLimit, "backoff-limit", "l", int32(2), "The backoff limit: how many times to retry the job before considering it failed) to run in the Job")
	cmd.Flags().StringVarP(&options.Image, "image", "i", "owasp/zap2docker-live:latest", "The OWASP image to use to run the ZA Proxy baseline scan")

	return cmd
}

// Create the addon
func (o *CreateAddonOwaspOptions) Run() error {
	name := "owasp-zap"
	commands := []string{"zap-baseline.py", "-I", "-t", "$(JX_PREVIEW_URL)"}
	image := o.Image
	if name == "" {
		return util.MissingOption(optionName)
	}
	if image == "" {
		return util.MissingOption(optionImage)
	}
	labels := map[string]string{
		kube.LabelCreatedBy: kube.ValueCreatedByJX,
		kube.LabelJobKind:   kube.ValueJobKindPostPreview,
	}

	firstContainer := corev1.Container{
		Name:    name,
		Image:   image,
		Command: commands,
	}

	callback := func(env *v1.Environment) error {
		settings := &env.Spec.TeamSettings
		for i := range settings.PostPreviewJobs {
			job := &settings.PostPreviewJobs[i]
			if job.Name == name {
				podSpec := &job.Spec.Template.Spec
				if len(podSpec.Containers) == 0 {
					podSpec.Containers = []corev1.Container{firstContainer}
				} else {
					container := &podSpec.Containers[0]
					container.Name = name
					container.Image = image
					container.Command = commands
				}
				job.Spec.BackoffLimit = &o.BackoffLimit
				log.Logger().Infof("Updating the post Preview Job: %s", util.ColorInfo(name))
				return nil
			}
		}
		settings.PostPreviewJobs = append(settings.PostPreviewJobs, batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:              name,
				Labels:            labels,
				CreationTimestamp: metav1.Now(),
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Now(),
					},
					Spec: corev1.PodSpec{
						Containers:    []corev1.Container{firstContainer},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
				BackoffLimit: &o.BackoffLimit,
			},
		})
		log.Logger().Infof("Added the post Preview Job: %s", util.ColorInfo(name))
		return nil
	}
	return o.ModifyDevEnvironment(callback)
}
