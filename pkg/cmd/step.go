package cmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/config"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	step2 "github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/bdd"
	"github.com/jenkins-x/jx/pkg/cmd/step/boot"
	"github.com/jenkins-x/jx/pkg/cmd/step/buildpack"
	"github.com/jenkins-x/jx/pkg/cmd/step/cluster"
	"github.com/jenkins-x/jx/pkg/cmd/step/create"
	"github.com/jenkins-x/jx/pkg/cmd/step/e2e"
	"github.com/jenkins-x/jx/pkg/cmd/step/env"
	"github.com/jenkins-x/jx/pkg/cmd/step/expose"
	"github.com/jenkins-x/jx/pkg/cmd/step/get"
	"github.com/jenkins-x/jx/pkg/cmd/step/git"
	"github.com/jenkins-x/jx/pkg/cmd/step/helm"
	"github.com/jenkins-x/jx/pkg/cmd/step/nexus"
	"github.com/jenkins-x/jx/pkg/cmd/step/post"
	"github.com/jenkins-x/jx/pkg/cmd/step/pr"
	"github.com/jenkins-x/jx/pkg/cmd/step/pre"
	"github.com/jenkins-x/jx/pkg/cmd/step/report"
	"github.com/jenkins-x/jx/pkg/cmd/step/restore"
	"github.com/jenkins-x/jx/pkg/cmd/step/scheduler"
	"github.com/jenkins-x/jx/pkg/cmd/step/syntax"
	"github.com/jenkins-x/jx/pkg/cmd/step/update"
	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	"github.com/spf13/cobra"
)

// NewCmdStep Steps a command object for the "step" command
func NewCmdStep(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &step2.StepOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "step",
		Short:   "pipeline steps",
		Aliases: []string{"steps"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(boot.NewCmdStepBoot(commonOpts))
	cmd.AddCommand(buildpack.NewCmdStepBuildPack(commonOpts))
	cmd.AddCommand(bdd.NewCmdStepBDD(commonOpts))
	cmd.AddCommand(e2e.NewCmdStepE2E(commonOpts))
	cmd.AddCommand(step.NewCmdStepBlog(commonOpts))
	cmd.AddCommand(step.NewCmdStepChangelog(commonOpts))
	cmd.AddCommand(cluster.NewCmdStepCluster(commonOpts))
	cmd.AddCommand(step.NewCmdStepCredential(commonOpts))
	cmd.AddCommand(create.NewCmdStepCreate(commonOpts))
	cmd.AddCommand(step.NewCmdStepCustomPipeline(commonOpts))
	cmd.AddCommand(env.NewCmdStepEnv(commonOpts))
	cmd.AddCommand(expose.NewCmdStepExpose(commonOpts))
	cmd.AddCommand(get.NewCmdStepGet(commonOpts))
	cmd.AddCommand(git.NewCmdStepGit(commonOpts))
	cmd.AddCommand(step.NewCmdStepGpgCredentials(commonOpts))
	cmd.AddCommand(helm.NewCmdStepHelm(commonOpts))
	cmd.AddCommand(step.NewCmdStepLinkServices(commonOpts))
	cmd.AddCommand(nexus.NewCmdStepNexus(commonOpts))
	cmd.AddCommand(step.NewCmdStepNextVersion(commonOpts))
	cmd.AddCommand(step.NewCmdStepNextBuildNumber(commonOpts))
	cmd.AddCommand(pre.NewCmdStepPre(commonOpts))
	cmd.AddCommand(pr.NewCmdStepPR(commonOpts))
	cmd.AddCommand(post.NewCmdStepPost(commonOpts))
	cmd.AddCommand(step.NewCmdStepRelease(commonOpts))
	cmd.AddCommand(step.NewCmdStepReplicate(commonOpts))
	cmd.AddCommand(step.NewCmdStepSplitMonorepo(commonOpts))
	cmd.AddCommand(syntax.NewCmdStepSyntax(commonOpts))
	cmd.AddCommand(step.NewCmdStepTag(commonOpts))
	cmd.AddCommand(step.NewCmdStepValidate(commonOpts))
	cmd.AddCommand(verify.NewCmdStepVerify(commonOpts))
	cmd.AddCommand(step.NewCmdStepWaitForArtifact(commonOpts))
	cmd.AddCommand(step.NewCmdStepWaitForChart(commonOpts))
	cmd.AddCommand(step.NewCmdStepStash(commonOpts))
	cmd.AddCommand(step.NewCmdStepUnstash(commonOpts))
	cmd.AddCommand(step.NewCmdStepValuesSchemaTemplate(commonOpts))
	cmd.AddCommand(scheduler.NewCmdStepScheduler(commonOpts))
	cmd.AddCommand(config.NewCmdStepPatchConfigMap(commonOpts))
	cmd.AddCommand(update.NewCmdStepUpdate(commonOpts))
	cmd.AddCommand(report.NewCmdStepReport(commonOpts))
	cmd.AddCommand(step.NewCmdStepOverrideRequirements(commonOpts))
	cmd.AddCommand(restore.NewCmdStepRestore(commonOpts))

	return cmd
}
