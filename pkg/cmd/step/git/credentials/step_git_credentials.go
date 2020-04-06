package credentials

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/gits/credentialhelper"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	optionOutputFile = "output"
	// Deprecated
	optionGitHubAppOwner = "github-app-owner"
	optionRepoOwner      = "repo-owner"
)

// StepGitCredentialsOptions contains the command line flags
type StepGitCredentialsOptions struct {
	step.StepOptions

	OutputFile string
	// Deprecated
	GitHubAppOwner    string
	RepoOwner         string
	GitKind           string
	CredentialsSecret string
	CredentialHelper  bool
}

var (
	StepGitCredentialsLong = templates.LongDesc(`
		This pipeline step generates a Git credentials file for the current Git provider secrets

`)

	StepGitCredentialsExample = templates.Examples(`
		# generate the Git credentials file in the canonical location
		jx step git credentials

		# generate the Git credentials to a output file
		jx step git credentials -o /tmp/mycreds

		# respond to a gitcredentials request
		jx step git credentials --credential-helper
`)
)

func NewCmdStepGitCredentials(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGitCredentialsOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "credentials",
		Short:   "Creates the Git credentials file for the current pipeline",
		Long:    StepGitCredentialsLong,
		Example: StepGitCredentialsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.OutputFile, optionOutputFile, "o", "", "The output file name")
	cmd.Flags().StringVarP(&options.GitHubAppOwner, optionGitHubAppOwner, "g", "", "Deprecated - The owner (organisation or user name) if using GitHub App based tokens")
	cmd.Flags().StringVarP(&options.RepoOwner, optionRepoOwner, "r", "", "The owner (organisation or user name) if using GitHub App based tokens")

	cmd.Flags().StringVarP(&options.CredentialsSecret, "credentials-secret", "s", "", "The secret name to read the credentials from")
	cmd.Flags().StringVarP(&options.GitKind, "git-kind", "", "", "The git kind. e.g. github, bitbucketserver etc")
	cmd.Flags().BoolVar(&options.CredentialHelper, "credential-helper", false, "respond to a gitcredentials request")

	return cmd
}

func (o *StepGitCredentialsOptions) Run() error {
	if os.Getenv("JX_CREDENTIALS_FROM_SECRET") != "" {
		log.Logger().Infof("Overriding CredentialsSecret from env var JX_CREDENTIALS_FROM_SECRET")
		o.CredentialsSecret = os.Getenv("JX_CREDENTIALS_FROM_SECRET")
	}

	outFile, err := o.determineOutputFile()
	if err != nil {
		return err
	}

	if o.CredentialsSecret != "" {
		// get secret
		kubeClient, ns, err := o.KubeClientAndDevNamespace()
		if err != nil {
			return err
		}

		secret, err := kubeClient.CoreV1().Secrets(ns).Get(o.CredentialsSecret, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to find secret '%s' in namespace '%s'", o.CredentialsSecret, ns)
			}
			return errors.Wrapf(err, "failed to read secret '%s' in namespace '%s'", o.CredentialsSecret, ns)
		}

		creds, err := credentialhelper.CreateGitCredentialFromURL(string(secret.Data["url"]), string(secret.Data["token"]), string(secret.Data["user"]))
		if err != nil {
			return errors.Wrap(err, "failed to create git credentials")
		}

		return o.createGitCredentialsFile(outFile, []credentialhelper.GitCredential{creds})
	}

	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return err
	}

	if o.RepoOwner == "" && o.GitHubAppOwner != "" {
		log.Logger().Warnf("The flag --%s is deprecated, use --%s instead", optionGitHubAppOwner, optionRepoOwner)
		o.RepoOwner = o.GitHubAppOwner
	}

	if gha && o.RepoOwner == "" {
		log.Logger().Infof("this command does nothing if using github app mode and no %s option specified", optionRepoOwner)
		return nil
	}

	var authConfigSvc auth.ConfigService
	if gha {
		authConfigSvc, err = o.GitAuthConfigServiceGitHubAppMode(o.GitKind)
		if err != nil {
			return errors.Wrap(err, "when creating auth config service using GitAuthConfigServiceGitHubAppMode")
		}
	} else {
		authConfigSvc, err = o.GitAuthConfigService()
		if err != nil {
			return errors.Wrap(err, "when creating auth config service using GitAuthConfigService")
		}
	}

	credentials, err := o.CreateGitCredentialsFromAuthService(authConfigSvc, gha)
	if err != nil {
		return errors.Wrap(err, "creating git credentials")
	}

	if o.CredentialHelper {
		helper, err := credentialhelper.CreateGitCredentialsHelper(os.Stdin, os.Stdout, credentials)
		if err != nil {
			return errors.Wrap(err, "unable to create git credential helper")
		}
		// the credential helper operation (get|store|remove) is passed as last argument to the helper
		err = helper.Run(os.Args[len(os.Args)-1])
		if err != nil {
			return err
		}
		return nil
	}

	outFile, err = o.determineOutputFile()
	if err != nil {
		return errors.Wrap(err, "unable to determine for git credentials")
	}

	return o.createGitCredentialsFile(outFile, credentials)
}

// GitCredentialsFileData takes the given git credentials and writes them into a byte array.
func (o *StepGitCredentialsOptions) GitCredentialsFileData(credentials []credentialhelper.GitCredential) ([]byte, error) {
	var buffer bytes.Buffer
	for _, gitCredential := range credentials {
		u, err := gitCredential.URL()
		if err != nil {
			log.Logger().Warnf("Ignoring incomplete git credentials %q", gitCredential)
			continue
		}

		buffer.WriteString(u.String() + "\n")
		// Write the https protocol in case only https is set for completeness
		if u.Scheme == "http" {
			u.Scheme = "https"
			buffer.WriteString(u.String() + "\n")
		}
	}

	return buffer.Bytes(), nil
}

func (o *StepGitCredentialsOptions) determineOutputFile() (string, error) {
	outFile := o.OutputFile
	if outFile == "" {
		outFile = util.GitCredentialsFile()
	}

	dir, _ := filepath.Split(outFile)
	if dir != "" {
		err := os.MkdirAll(dir, util.DefaultWritePermissions)
		if err != nil {
			return "", err
		}
	}
	return outFile, nil
}

// CreateGitCredentialsFileFromUsernameAndToken creates the git credentials into file using the provided username, token & url
func (o *StepGitCredentialsOptions) createGitCredentialsFile(fileName string, credentials []credentialhelper.GitCredential) error {
	data, err := o.GitCredentialsFileData(credentials)
	if err != nil {
		return errors.Wrap(err, "creating git credentials")
	}

	if err := ioutil.WriteFile(fileName, data, util.DefaultWritePermissions); err != nil {
		return fmt.Errorf("failed to write to %s: %s", fileName, err)
	}
	log.Logger().Infof("Generated Git credentials file %s", util.ColorInfo(fileName))
	return nil
}

// CreateGitCredentialsFromAuthService creates the git credentials using the auth config service
func (o *StepGitCredentialsOptions) CreateGitCredentialsFromAuthService(authConfigSvc auth.ConfigService, githubAppEnabled bool) ([]credentialhelper.GitCredential, error) {
	var credentialList []credentialhelper.GitCredential

	cfg := authConfigSvc.Config()
	if cfg == nil {
		return nil, errors.New("no git auth config found")
	}

	for _, server := range cfg.Servers {
		var auths []*auth.UserAuth
		if githubAppEnabled && o.RepoOwner != "" {
			auths = server.Users
		} else {
			gitAuth := server.CurrentAuth()
			if gitAuth == nil {
				continue
			} else {
				auths = append(auths, gitAuth)
			}
		}
		for _, gitAuth := range auths {
			if githubAppEnabled && o.RepoOwner != "" && gitAuth.GithubAppOwner != o.RepoOwner {
				continue
			}
			username := gitAuth.Username
			password := gitAuth.ApiToken
			if password == "" {
				password = gitAuth.BearerToken
			}
			if password == "" {
				password = gitAuth.Password
			}
			if username == "" || password == "" {
				log.Logger().Warnf("Empty auth config for git service URL %q", server.URL)
				continue
			}

			credential, err := credentialhelper.CreateGitCredentialFromURL(server.URL, username, password)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid git auth information")
			}

			credentialList = append(credentialList, credential)
		}
	}
	return credentialList, nil
}
