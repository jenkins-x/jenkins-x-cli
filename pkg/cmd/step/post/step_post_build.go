package post

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/kube/services"

	"os"

	"os/exec"
	"strings"

	"bufio"

	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	defaultAnchoreName = "anchore"
)

// StepPostBuildOptions contains the command line flags
type StepPostBuildOptions struct {
	step.StepOptions
	FullImageName string
	OutputFile    string
}

type anchoreDetails struct {
	URL      string
	Username string
	Password string
}

var (
	StepPostBuildLong = templates.LongDesc(`
		This pipeline step performs post build actions such as CVE analysis
`)

	StepPostBuildExample = templates.Examples(`
		jx step post build
`)
)

const podAnnotations = `
podAnnotations:
  jenkins-x.io/cve-image-id: %s
`

func NewCmdStepPostBuild(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepPostBuildOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "build",
		Short:   "Performs post build actions in a pipeline",
		Long:    StepPostBuildLong,
		Example: StepPostBuildExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.FullImageName, "image", "", "", "The full image name to be analysed including the registry prefix")

	return cmd
}

func (o *StepPostBuildOptions) Run() error {
	_, err := o.KubeClient()
	if err != nil {
		return fmt.Errorf("error connecting to Kubernetes cluster: %v", err)
	}

	// let's try and add image to CVE provider
	err = o.addImageCVEProvider()
	if err != nil {
		return fmt.Errorf("error adding image to CVE provider: %v", err)
	}

	return nil
}
func (o *StepPostBuildOptions) addImageCVEProvider() error {
	if o.FullImageName == "" {
		return util.MissingOption("image")
	}

	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	present, err := services.IsServicePresent(client, kube.AddonServices[defaultAnchoreName], currentNamespace)
	if err != nil || !present {
		log.Logger().Infof("no CVE provider running in the current %s namespace so skip adding image to be analysed", currentNamespace)
		return nil
	}

	cveProviderHost := os.Getenv("JENKINS_X_DOCKER_REGISTRY_SERVICE_HOST")
	if cveProviderHost == "" {
		return fmt.Errorf("no JENKINS_X_DOCKER_REGISTRY_SERVICE_HOST env var found")
	}

	cveProviderPort := os.Getenv("JENKINS_X_DOCKER_REGISTRY_SERVICE_PORT")
	if cveProviderPort == "" {
		return fmt.Errorf("no JENKINS_X_DOCKER_REGISTRY_SERVICE_PORT env var found")
	}

	log.Logger().Infof("adding image %s to CVE provider", o.FullImageName)

	imageID, err := o.addImageToAnchore()
	if err != nil {
		return fmt.Errorf("failed to add image %s to anchore engine: %v\n", o.FullImageName, err)
	}

	err = o.addImageIDtoHelmValues(imageID)
	if err != nil {
		return fmt.Errorf("failed to add image id %s to helm values: %v\n", imageID, err)
	}

	// todo use image id to annotate pods during environments helm install / upgrade
	// todo then we can use `jx get cve --env staging` and list all CVEs for an environment
	log.Logger().Infof("anchore image is %s ", imageID)
	return nil
}

func (o *StepPostBuildOptions) addImageToAnchore() (string, error) {

	a, err := o.getAnchoreDetails()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("anchore-cli", "image", "add", o.FullImageName) // #nosec
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "ANCHORE_CLI_USER="+a.Username)
	cmd.Env = append(cmd.Env, "ANCHORE_CLI_PASS="+a.Password)
	cmd.Env = append(cmd.Env, "ANCHORE_CLI_URL="+a.URL)
	data, err := cmd.CombinedOutput()
	text := string(data)

	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	var imageID string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "Image ID:") {
			imageID = strings.Replace(scanner.Text(), "Image ID:", "", -1)
			imageID = strings.TrimSpace(imageID)
			break
		}
	}
	if imageID == "" {
		return "", fmt.Errorf("no Image ID returned from Anchore")
	}
	return imageID, nil
}

func (o *StepPostBuildOptions) getAnchoreDetails() (anchoreDetails, error) {
	var a anchoreDetails
	anchoreAuthSvc, err := o.AddonAuthConfigService(kube.ValueKindCVE)
	if err != nil {
		return a, errors.Wrap(err, "creating the auth config service for anchore")
	}

	cfg := anchoreAuthSvc.Config()
	if cfg == nil {
		return a, errors.New("no auth config found anchore")
	}
	server := cfg.CurrentAuthServer()
	if server == nil {
		return a, errors.New("no server auth config found for anchore")
	}

	auth := server.CurrentAuth()
	if auth == nil {
		return a, fmt.Errorf("no auth configuration found for server %q", server.URL)
	}
	a.URL = server.URL
	a.Username = auth.Username
	a.Password = auth.Password
	if a.Password == "" {
		a.Password = auth.ApiToken
	}
	if a.Password == "" {
		a.Password = auth.BearerToken
	}
	return a, nil
}

func (o *StepPostBuildOptions) addImageIDtoHelmValues(imageID string) error {

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	charts := filepath.Join(pwd, "charts")
	exists, err := util.DirExists(charts)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("no charts folder found are you in the root folder of your project?")
	}

	// loop through all directories and if there's a values.yaml add image id to the end
	err = filepath.Walk(charts, func(path string, f os.FileInfo, err error) error {

		if f.IsDir() {
			values := filepath.Join(path, "values.yaml")
			valuesExist, err := util.FileExists(values)
			if err != nil {
				return err
			}
			if valuesExist {
				f, err := os.OpenFile(values, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}

				defer f.Close()

				if _, err = f.WriteString(fmt.Sprintf(podAnnotations, imageID)); err != nil {
					return err
				}
			}

		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
