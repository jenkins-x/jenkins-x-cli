package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/jenkins-x/jx/pkg/surveyutils"

	"github.com/ghodss/yaml"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

// AddAppOptions the options for the create spring command
type AddAppOptions struct {
	AddOptions

	GitOps bool
	DevEnv *jenkinsv1.Environment

	Repo     string
	Username string
	Password string
	Alias    string

	// for testing
	FakePullRequests CreateEnvPullRequestFn

	// allow git to be configured externally before a PR is created
	ConfigureGitCallback ConfigureGitFolderFn

	Namespace   string
	Version     string
	ReleaseName string
	SetValues   []string
	ValueFiles  []string
	HelmUpdate  bool
}

const (
	optionHelmUpdate = "helm-update"
	optionValues     = "value"
	optionSet        = "set"
	optionAlias      = "alias"
)

// NewCmdAddApp creates a command object for the "create" command
func NewCmdAddApp(f Factory, in terminal.FileReader, out terminal.FileWriter, errOut io.Writer) *cobra.Command {
	options := &AddAppOptions{
		AddOptions: AddOptions{
			CommonOptions: CommonOptions{
				Factory: f,
				In:      in,
				Out:     out,
				Err:     errOut,
			},
		},
	}

	cmd := &cobra.Command{
		Use:   "app",
		Short: "Adds an app",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
	}

	options.addFlags(cmd, kube.DefaultNamespace, "", "")
	return cmd
}

func (o *AddAppOptions) addFlags(cmd *cobra.Command, defaultNamespace string, defaultOptionRelease string, defaultVersion string) {

	// Common flags

	cmd.Flags().StringVarP(&o.Version, "version", "v", defaultVersion,
		"The chart version to install")
	cmd.Flags().StringVarP(&o.Repo, "repository", "", "",
		"The repository from which the app should be installed (default specified in your dev environment)")
	cmd.Flags().StringVarP(&o.Username, "username", "", "",
		"The username for the repository")
	cmd.Flags().StringVarP(&o.Password, "password", "", "",
		"The password for the repository")
	cmd.Flags().BoolVarP(&o.BatchMode, optionBatchMode, "b", false, "In batch mode the command never prompts for user input")
	cmd.Flags().BoolVarP(&o.Verbose, optionVerbose, "", false, "Enable verbose logging")
	cmd.Flags().StringVarP(&o.Alias, optionAlias, "", "",
		"An alias to use for the app (available when using GitOps for your dev environment)")
	cmd.Flags().StringVarP(&o.ReleaseName, optionRelease, "r", defaultOptionRelease,
		"The chart release name (available when NOT using GitOps for your dev environment)")
	cmd.Flags().BoolVarP(&o.HelmUpdate, optionHelmUpdate, "", true,
		"Should we run helm update first to ensure we use the latest version (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringVarP(&o.Namespace, optionNamespace, "n", defaultNamespace, "The Namespace to install into (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringArrayVarP(&o.ValueFiles, optionValues, "f", []string{}, "List of locations for values files, "+
		"can be local files or URLs (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringArrayVarP(&o.SetValues, optionSet, "s", []string{},
		"The chart set values (can specify multiple or separate values with commas: key1=val1,key2=val2) (available when NOT using GitOps for your dev environment)")

}

// Run implements this command
func (o *AddAppOptions) Run() error {
	o.GitOps, o.DevEnv = o.GetDevEnv()
	if o.Repo == "" {
		o.Repo = o.DevEnv.Spec.TeamSettings.AppsRepository
	}

	if o.GitOps {
		msg := "Unable to specify --%s when using GitOps for your dev environment"
		if o.ReleaseName != "" {
			return util.InvalidOptionf(optionRelease, o.ReleaseName, msg, optionRelease)
		}
		if !o.HelmUpdate {
			return util.InvalidOptionf(optionHelmUpdate, o.ReleaseName, msg, optionHelmUpdate)
		}
		if o.Namespace != "" {
			return util.InvalidOptionf(optionNamespace, o.ReleaseName, msg, optionNamespace)
		}
		if len(o.ValueFiles) > 0 {
			return util.InvalidOptionf(optionValues, o.ReleaseName, msg, optionValues)
		}
		if len(o.SetValues) > 0 {
			return util.InvalidOptionf(optionSet, o.ReleaseName, msg, optionSet)
		}
	}
	if !o.GitOps {
		if o.Alias != "" {
			return util.InvalidOptionf(optionAlias, o.ReleaseName,
				"Unable to specify --%s when NOT using GitOps for your dev environment", optionAlias)
		}
	}

	/*err = o.GenerateValues()
	if err != nil {
		return err
	}*/

	args := o.Args
	if len(args) == 0 {
		return o.Cmd.Help()
	}
	if len(args) > 1 {
		return o.Cmd.Help()
	}

	if o.Repo == "" {
		return fmt.Errorf("must specify a repository")
	}

	for _, app := range args {
		version := o.Version
		var schema []byte
		err := helm.InspectChart(app, o.Repo, o.Username, o.Password, o.Helm(), func(dir string) error {
			_, v, err := helm.LoadChartNameAndVersion(filepath.Join(dir, "Chart.yaml"))
			if err != nil {
				return err
			}
			if version == "" {
				version = v
				if o.Verbose {
					log.Infof("No version specified so using latest version which is %s\n", util.ColorInfo(version))
				}
			}
			schemaFile := filepath.Join(dir, "values.schema.json")
			if _, err := os.Stat(schemaFile); !os.IsNotExist(err) {
				schema, err = ioutil.ReadFile(schemaFile)
				if err != nil {
					return err
				}
			}

			secrets := make([]*corev1.Secret, 0)
			schemaOptions := surveyutils.JSONSchemaOptions{
				CreateSecret: func(name string, key string, value string) (*jenkinsv1.ResourceReference, error) {
					secret := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Secret",
							APIVersion: corev1.SchemeGroupVersion.Version,
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
						},
						Data: map[string][]byte{
							key: []byte(value),
						},
					}
					secrets = append(secrets, secret)
					return &jenkinsv1.ResourceReference{
						Name: name,
						Kind: "Secret",
					}, nil

				},
			}
			values, err := schemaOptions.GenerateValues(schema, []string{app}, o.In, o.Out, o.Err)
			if err != nil {
				return err
			}
			valuesYaml, err := yaml.JSONToYAML(values)
			if err != nil {
				return err
			}
			if o.Verbose {
				log.Infof("Generated values.yaml:\n\n%v\n", util.ColorInfo(string(valuesYaml)))
			}

			// For each secret, we write a file into the chart
			templatesDir := filepath.Join(dir, "templates")
			err = os.MkdirAll(templatesDir, 0700)
			if err != nil {
				return err
			}
			for _, secret := range secrets {
				file, err := ioutil.TempFile(templatesDir, fmt.Sprintf("%s-*.yaml", secret.Name))
				defer func() {
					err = file.Close()
					if err != nil {
						log.Warnf("Error closing %s because %v\n", file.Name(), err)
					}
				}()
				if err != nil {
					return err
				}
				bs, err := json.Marshal(secret)
				if err != nil {
					return err
				}
				ybs, err := yaml.JSONToYAML(bs)
				if err != nil {
					return err
				}
				_, err = file.Write(ybs)
				if err != nil {
					return err
				}
				if o.Verbose {
					log.Infof("Added secret %s\n\n%v\n", secret.Name, util.ColorInfo(string(ybs)))
				}
				if err != nil {
					return err
				}
			}

			if o.GitOps {
				err := o.createPR(app, version, schema)
				if err != nil {
					return err
				}
			} else {
				if len(o.ValueFiles) > 0 && schema != nil {
					log.Warnf("values.yaml specified by --valuesFiles will be used despite presence of schema in app")
				} else if schema != nil {
					valuesFile, err := ioutil.TempFile("", fmt.Sprintf("%s-values.yaml", app))
					defer func() {
						err = valuesFile.Close()
						if err != nil {
							log.Warnf("Error closing %s because %v\n", valuesFile.Name(), err)
						}
					}()
					if err != nil {
						return err
					}
					_, err = valuesFile.Write(valuesYaml)
					if err != nil {
						return err
					}
					o.ValueFiles = []string{
						valuesFile.Name(),
					}
				}

				if err != nil {
					return err
				}
				err := o.installApp(app, dir, version)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *AddAppOptions) createPR(app string, version string, schema []byte) error {

	modifyRequirementsFn := func(requirements *helm.Requirements) error {
		// See if the app already exists in requirements
		found := false
		for _, d := range requirements.Dependencies {
			if d.Name == app && d.Alias == o.Alias {
				// App found
				log.Infof("App %s already installed.\n", util.ColorWarning(app))
				if version != d.Version {
					log.Infof("To upgrade the app use %s or %s\n",
						util.ColorInfo("jx upgrade app <app>"),
						util.ColorInfo("jx upgrade apps --all"))
				}
				found = true
				break
			}
		}
		// If app not found, add it
		if !found {
			requirements.Dependencies = append(requirements.Dependencies, &helm.Dependency{
				Alias:      o.Alias,
				Repository: o.Repo,
				Name:       app,
				Version:    version,
			})
		}
		return nil
	}
	branchNameText := "add-app-" + app + "-" + version
	title := fmt.Sprintf("Add %s %s", app, version)
	message := fmt.Sprintf("Add app %s %s", app, version)
	var pullRequestInfo *gits.PullRequestInfo
	if o.FakePullRequests != nil {
		var err error
		pullRequestInfo, err = o.FakePullRequests(o.DevEnv, modifyRequirementsFn, branchNameText, title, message,
			nil)
		if err != nil {
			return err
		}
	} else {
		var err error
		pullRequestInfo, err = o.createEnvironmentPullRequest(o.DevEnv, modifyRequirementsFn, &branchNameText, &title,
			&message,
			nil, o.ConfigureGitCallback)
		if err != nil {
			return err
		}
	}
	log.Infof("Added app via Pull Request %s\n", pullRequestInfo.PullRequest.URL)
	return nil
}

func (o *AddAppOptions) installApp(name string, chart string, version string) error {
	err := o.ensureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that helm is present")
	}
	setValues := make([]string, 0)
	for _, vs := range o.SetValues {
		setValues = append(setValues, strings.Split(vs, ",")...)
	}

	err = o.installChartOptions(helm.InstallChartOptions{
		ReleaseName: name,
		Chart:       chart,
		Version:     version,
		Ns:          o.Namespace,
		HelmUpdate:  o.HelmUpdate,
		SetValues:   setValues,
		ValueFiles:  o.ValueFiles,
		Repository:  o.Repo,
		Username:    o.Username,
		Password:    o.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to install name %s: %v", name, err)
	}
	// Attach the secrets to the name CRD

	return o.OnAppInstall(name, version)
}
