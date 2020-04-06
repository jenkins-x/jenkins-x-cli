package gc

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/deletecmd"
	"github.com/jenkins-x/jx/pkg/cmd/preview"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/promote"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"strconv"

	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
)

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type GCPreviewsOptions struct {
	*opts.CommonOptions

	DisableImport bool
	OutDir        string
}

var (
	GCPreviewsLong = templates.LongDesc(`
		Garbage collect Jenkins X preview environments.  If a pull request is merged or closed the associated preview
		environment will be deleted.

`)

	GCPreviewsExample = templates.Examples(`
		jx garbage collect previews
		jx gc previews
`)
)

// NewCmd s a command object for the "step" command
func NewCmdGCPreviews(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GCPreviewsOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "previews",
		Short:   "garbage collection for preview environments",
		Long:    GCPreviewsLong,
		Example: GCPreviewsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

// Run implements this command
func (o *GCPreviewsOptions) Run() error {
	client, currentNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	// cannot use field selectors like `spec.kind=Preview` on CRDs so list all environments
	envs, err := client.JenkinsV1().Environments(currentNs).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(envs.Items) == 0 {
		// no environments found so lets return gracefully
		log.Logger().Debug("no environments found")
		return nil
	}

	var previewFound bool
	for _, e := range envs.Items {
		if e.Spec.Kind == v1.EnvironmentKindTypePreview {
			previewFound = true
			gitInfo, err := gits.ParseGitURL(e.Spec.Source.URL)
			if err != nil {
				return err
			}
			// we need pull request info to include
			authConfigSvc, err := o.GitAuthConfigService()
			if err != nil {
				return err
			}

			gitKind, err := o.GitServerKind(gitInfo)
			if err != nil {
				return err
			}

			ghOwner, err := o.GetGitHubAppOwner(gitInfo)
			if err != nil {
				return err
			}
			gitProvider, err := gitInfo.CreateProvider(o.InCluster(), authConfigSvc, gitKind, ghOwner, o.Git(), o.BatchMode, o.GetIOFileHandles())
			if err != nil {
				return err
			}
			prNum, err := strconv.Atoi(e.Spec.PreviewGitSpec.Name)
			if err != nil {
				log.Logger().Warn("Unable to convert PR " + e.Spec.PreviewGitSpec.Name + " to a number")
			}
			pullRequest, err := gitProvider.GetPullRequest(gitInfo.Organisation, gitInfo, prNum)
			if err != nil {
				log.Logger().Warnf("Can not get pull request %s, skipping: %s", e.Spec.PreviewGitSpec.Name, err)
				continue
			}

			lowerState := strings.ToLower(*pullRequest.State)

			if strings.HasPrefix(lowerState, "clos") || strings.HasPrefix(lowerState, "merged") || strings.HasPrefix(lowerState, "superseded") || strings.HasPrefix(lowerState, "declined") {
				// lets delete the preview environment
				deleteOpts := deletecmd.DeletePreviewOptions{
					PreviewOptions: preview.PreviewOptions{
						PromoteOptions: promote.PromoteOptions{
							CommonOptions: o.CommonOptions,
						},
					},
				}
				err = deleteOpts.DeletePreview(e.Name)
				if err != nil {
					return fmt.Errorf("failed to delete preview environment %s: %v\n", e.Name, err)
				}
			}
		}
	}
	if !previewFound {
		log.Logger().Debug("no preview environments found")
	}
	return nil
}
