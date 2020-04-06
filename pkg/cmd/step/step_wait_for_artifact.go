package step

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	optionRepo              = "repo"
	optionGroup             = "group"
	optionArtifact          = "artifact"
	optionVersion           = "version"
	optionPollTime          = "poll-time"
	DefaultMavenCentralRepo = "https://repo1.maven.org/maven2/"
)

// StepWaitForArtifactOptions contains the command line flags
type StepWaitForArtifactOptions struct {
	step.StepOptions

	ArtifactURL string
	RepoURL     string
	GroupId     string
	ArtifactId  string
	Version     string
	Extension   string
	Timeout     string
	PollTime    string

	// calculated fields
	TimeoutDuration time.Duration
	PollDuration    time.Duration
}

var (
	StepWaitForArtifactLong = templates.LongDesc(`
		Waits for the given artifact to be available in a maven style repository

`)

	StepWaitForArtifactExample = templates.Examples(`
		# wait for a 
		jx step gpg credentials

		# generate the git credentials to a output file
		jx step gpg credentials -o /tmp/mycreds

`)
)

func NewCmdStepWaitForArtifact(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepWaitForArtifactOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "wait for artifact",
		Short:   "Waits for the given artifact to be available in a maven style repository",
		Long:    StepWaitForArtifactLong,
		Example: StepWaitForArtifactExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.ArtifactURL, "artifact-url", "", "", "The full URL of the artifact to wait for. If not specified it is calculated from the repository URL, group, artifact and version")
	cmd.Flags().StringVarP(&options.RepoURL, optionRepo, "r", DefaultMavenCentralRepo, "The URL of the maven style repository to query for the artifact")
	cmd.Flags().StringVarP(&options.GroupId, optionGroup, "g", "", "The group ID of the artifact to search for")
	cmd.Flags().StringVarP(&options.ArtifactId, optionArtifact, "a", "", "The artifact ID of the artifact to search for")
	cmd.Flags().StringVarP(&options.Version, optionVersion, "v", "", "The version of the artifact to search for")
	cmd.Flags().StringVarP(&options.Extension, "ext", "x", "pom", "The file extension to search for")
	cmd.Flags().StringVarP(&options.Timeout, opts.OptionTimeout, "t", "1h", "The duration before we consider this operation failed")
	cmd.Flags().StringVarP(&options.PollTime, optionPollTime, "", "10s", "The amount of time between polls for the artifact URL being present")
	return cmd
}

func (o *StepWaitForArtifactOptions) getUrlStatusOK(u string) error {
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Failed in request for %s as got status %d %s", u, res.StatusCode, res.Status)
	}
	return nil
}

func (o *StepWaitForArtifactOptions) Run() error {
	var err error
	if o.PollTime != "" {
		o.PollDuration, err = time.ParseDuration(o.PollTime)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.PollTime, optionPollTime, err)
		}
	}
	if o.Timeout != "" {
		o.TimeoutDuration, err = time.ParseDuration(o.Timeout)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.Timeout, opts.OptionTimeout, err)
		}
	}

	if o.ArtifactURL == "" {
		// lets create it from the various parts
		if o.RepoURL == "" {
			return util.MissingOption(optionRepo)
		}
		group := o.GroupId
		if group == "" {
			return util.MissingOption(optionGroup)
		}
		group = strings.Replace(group, ".", "/", -1)
		artifact := o.ArtifactId
		if artifact == "" {
			return util.MissingOption(optionArtifact)
		}
		version := o.Version
		if version == "" {
			return util.MissingOption(optionVersion)
		}
		o.ArtifactURL = util.UrlJoin(o.RepoURL, group, artifact, version, artifact+"-"+version+"."+o.Extension)
	}
	log.Logger().Infof("Waiting for artifact at %s", util.ColorInfo(o.ArtifactURL))

	fn := func() error {
		return o.getUrlStatusOK(o.ArtifactURL)
	}
	err = o.RetryQuietlyUntilTimeout(o.TimeoutDuration, o.PollDuration, fn)
	if err == nil {
		log.Logger().Infof("Found artifact at %s", util.ColorInfo(o.ArtifactURL))
		return nil
	}
	log.Logger().Warnf("Failed to find artifact at %s due to %s", o.ArtifactURL, err)
	return err
}
