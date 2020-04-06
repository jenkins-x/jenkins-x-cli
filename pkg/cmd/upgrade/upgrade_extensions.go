package upgrade

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/extensions"

	"github.com/pkg/errors"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/log"

	typev1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/ghodss/yaml"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

//const upstreamExtensionsRepositoryGitHub = "https://raw.githubusercontent.com/jenkins-x/jenkins-x-extensions/master/jenkins-x-extensions-repository.lock.yaml"
const upstreamExtensionsRepositoryGitHub = "github.com/jenkins-x/jenkins-x-extensions"

var (
	upgradeExtensionsLong = templates.LongDesc(`
		Upgrades the Jenkins X extensions available to this Jenkins X install if there are new versions available
`)

	upgradeExtensionsExample = templates.Examples(`
		
		# upgrade extensions
		jx upgrade extensions

	`)
)

type UpgradeExtensionsOptions struct {
	options.CreateOptions
	Filter                   string
	ExtensionsRepositoryFile string
}

func NewCmdUpgradeExtensions(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeExtensionsOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "extensions",
		Short:   "Upgrades the Jenkins X extensions available to this Jenkins X install if there are new versions available",
		Long:    upgradeExtensionsLong,
		Example: upgradeExtensionsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdUpgradeExtensionsRepository(commonOpts))
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryFile, "extensions-repository-file", "", "", "Specify the extensions repository yaml file to read from")
	return cmd
}

func (o *UpgradeExtensionsOptions) Run() error {

	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterExtensionCRD(apisClient)
	if err != nil {
		return err
	}

	// Let's register the release CRD as charts built using Jenkins X use it, and it's very likely that people installing
	// apps are using Helm
	err = kube.RegisterReleaseCRD(apisClient)
	if err != nil {
		return err
	}

	kubeClient, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	extensionsList, err := (&jenkinsv1.ExtensionConfigList{}).LoadFromConfigMap(extensions.ExtensionsConfigDefaultConfigMap, kubeClient, curNs)
	if err != nil {
		return err
	}

	if len(extensionsList.Extensions) > 0 {
		log.Logger().Debugf("These extensions are configured for the team:")
		for _, e := range extensionsList.Extensions {
			log.Logger().Debugf("  %s", util.ColorInfo(e.FullyQualifiedName()))
		}
	} else {
		log.Logger().Warnf("No extensions are configured for the team")

	}

	extensionsRepository := jenkinsv1.ExtensionRepositoryLockList{}
	var bs []byte

	if o.ExtensionsRepositoryFile != "" {
		path := o.ExtensionsRepositoryFile
		// if it starts with a ~ it's the users homedir
		if strings.HasPrefix(path, "~") {
			usr, err := user.Current()
			if err == nil {
				path = filepath.Join(usr.HomeDir, strings.TrimPrefix(path, "~"))
			}
		}
		// Perhaps it's an absolute file path
		bs, err = ioutil.ReadFile(path)
		if err != nil {
			// Perhaps it's a relative path
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			log.Logger().Infof("Updating extensions from %s", path)
			bs, err = ioutil.ReadFile(filepath.Join(cwd, path))
			if err != nil {
				return errors.New(fmt.Sprintf("Unable to open Extensions Repository at %s", path))
			}
		}
	} else {
		extensionsConfig, err := extensions.GetOrCreateExtensionsConfig(kubeClient, curNs)
		if err != nil {
			return err
		}
		current := jenkinsv1.ExtensionRepositoryReference{}
		err = yaml.Unmarshal([]byte(extensionsConfig.Data[jenkinsv1.ExtensionsConfigRepository]), &current)
		if err != nil {
			return err
		}
		if current.Chart.Name != "" {
			unpackDir, err := ioutil.TempDir("", "jenkins-x-extensions-chart")
			if err != nil {
				return err
			}
			log.Logger().Debugf("Using %s to unpack Helm Charts", util.ColorInfo(unpackDir))
			chart := fmt.Sprintf("%s/%s", current.Chart.RepoName, current.Chart.Name)
			log.Logger().Infof("Updating extensions from Helm Chart %s repo %s ", util.ColorInfo(chart), util.ColorInfo(current.Chart.Repo))
			err = o.Helm().FetchChart(chart, "", true, unpackDir, "", "", "")
			if err != nil {
				return err
			}
			path := filepath.Join(unpackDir, current.Chart.Name, "repository", "jenkins-x-extensions-repository.lock.yaml")
			bs, err = ioutil.ReadFile(path)
			log.Logger().Debugf("Extensions Repository Lock located at %s", util.ColorInfo(path))
			if err != nil {
				return fmt.Errorf("Unable to fetch Extensions Repository Helm Chart %s/%s because %v", current.Chart.RepoName, current.Chart.Name, err)
			}
		} else {
			extensionsRepositoryUrl := current.Url
			if extensionsRepositoryUrl == "" {
				extensionsRepositoryUrl = upstreamExtensionsRepositoryGitHub
			}
			if current.GitHub != "" {
				_, repoInfo, err := o.CreateGitProviderForURLWithoutKind(current.GitHub)
				if err != nil {
					return err
				}
				resolvedTag, err := util.GetLatestReleaseFromGitHub(repoInfo.Organisation, repoInfo.Name)
				if err != nil {
					return err
				}
				extensionsRepositoryUrl = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/jenkins-x-extensions-repository.lock.yaml", repoInfo.Organisation, repoInfo.Name, resolvedTag)
			}
			log.Logger().Infof("Updating extensions from %s", extensionsRepositoryUrl)
			httpClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := httpClient.Get(fmt.Sprintf("%s?version=%d", extensionsRepositoryUrl, time.Now().UnixNano()/int64(time.Millisecond)))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			bs, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
		}
	}

	err = yaml.Unmarshal(bs, &extensionsRepository)
	if err != nil {
		return err
	}
	log.Logger().Infof("Upgrading to Extension Repository version %s", util.ColorInfo(extensionsRepository.Version))
	client, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	extensionsClient := client.JenkinsV1().Extensions(ns)

	availableExtensionsUUIDLookup := make(map[string]jenkinsv1.ExtensionSpec, 0)
	for _, e := range extensionsRepository.Extensions {
		availableExtensionsUUIDLookup[e.UUID] = e
	}

	installedExtensions, err := o.GetInstalledExtensions(extensionsClient)
	if err != nil {
		return err
	}
	// This will cause o.devNamespace to be populated
	_, _, err = o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	needsUpstalling := make([]jenkinsv1.ExtensionExecution, 0)
	for _, e := range extensionsRepository.Extensions {
		// TODO this is not very efficient probably
		for _, c := range extensionsList.Extensions {
			if c.Name == e.Name && c.Namespace == e.Namespace {
				n, err := o.UpsertExtension(&e, extensionsClient, installedExtensions, c, availableExtensionsUUIDLookup, 0, 0)
				if err != nil {
					return err
				}
				needsUpstalling = append(needsUpstalling, n...)
				break
			}
		}
	}
	// Before we start upstalling let's do a helm update, as extensions are likely to want it
	err = o.Helm().UpdateRepo()
	if err != nil {
		return err
	}
	for _, n := range needsUpstalling {
		envVars := ""
		if len(n.EnvironmentVariables) > 0 {
			envVarsFormatted := new(bytes.Buffer)
			for _, envVar := range n.EnvironmentVariables {
				fmt.Fprintf(envVarsFormatted, "%s=%s, ", envVar.Name, envVar.Value)
			}
			envVars = fmt.Sprintf("with environment variables [ %s ]", util.ColorInfo(strings.TrimSuffix(envVarsFormatted.String(), ", ")))
		}

		log.Logger().Infof("Preparing %s %s", util.ColorInfo(n.FullyQualifiedName()), envVars)
		n.Execute()
	}
	return nil
}

func (o *UpgradeExtensionsOptions) UpsertExtension(extension *jenkinsv1.ExtensionSpec, exts typev1.ExtensionInterface, installedExtensions map[string]jenkinsv1.Extension, extensionConfig jenkinsv1.ExtensionConfig, lookup map[string]jenkinsv1.ExtensionSpec, depth int, initialIndent int) (needsUpstalling []jenkinsv1.ExtensionExecution, err error) {
	result := make([]jenkinsv1.ExtensionExecution, 0)
	indent := ((depth - 1) * 2) + initialIndent

	_, devNamespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return result, err
	}
	// TODO Validate extension
	newVersion, err := semver.Parse(extension.Version)
	if err != nil {
		return result, err
	}
	existing, ok := installedExtensions[extension.UUID]
	if !ok {
		// Check for a name conflict
		res, err := exts.Get(extension.FullyQualifiedKebabName(), metav1.GetOptions{})
		if err == nil {
			return result, errors.New(fmt.Sprintf("Extension %s has changed UUID. It used to have UUID %s and now has UUID %s. If this is correct, then you should manually remove the extension using\n"+
				"\n"+
				"  kubectl delete ext %s\n"+
				"\n"+
				"If this is not correct, then contact the extension maintainer and inform them of this change.", util.ColorWarning(extension.FullyQualifiedName()), util.ColorWarning(res.Spec.UUID), util.ColorWarning(extension.UUID), extension.FullyQualifiedKebabName()))
		}
		// Doesn't exist
		res, err = exts.Create(&jenkinsv1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf(extension.FullyQualifiedKebabName()),
			},
			Spec: *extension,
		})
		if depth == 0 {
			initialIndent = 7
			log.Logger().Infof("Adding %s version %s", util.ColorInfo(extension.FullyQualifiedName()), util.ColorInfo(newVersion))
		} else {
			log.Logger().Infof("%s└ %s version %s", strings.Repeat(" ", indent), util.ColorInfo(extension.FullyQualifiedName()), util.ColorInfo(extension.Version))
		}
		if err != nil {
			return result, err
		}
		if o.Contains(extension.When, jenkinsv1.ExtensionWhenInstall) {
			e, _, err := extensions.ToExecutable(extension, extensionConfig.Parameters, devNamespace, exts)
			if err != nil {
				return result, err
			}
			result = append(result, e)
		}
	}
	// TODO Handle uninstalling existing extension if name has changed but UUID hasn't
	if existing.Spec.Version != "" {
		existingVersion, err := semver.Parse(existing.Spec.Version)
		if err != nil {
			return result, err
		}
		if existingVersion.LT(newVersion) {
			existing.Spec = *extension
			_, err := exts.PatchUpdate(&existing)
			if err != nil {
				return result, err
			}
			if o.Contains(extension.When, jenkinsv1.ExtensionWhenUpgrade) {
				e, _, err := extensions.ToExecutable(extension, extensionConfig.Parameters, devNamespace, exts)
				if err != nil {
					return result, err
				}
				result = append(result, e)
			}
			if depth == 0 {
				initialIndent = 10
				log.Logger().Infof("Upgrading %s from %s to %s", util.ColorInfo(extension.FullyQualifiedName()), util.ColorInfo(existingVersion), util.ColorInfo(newVersion))
			} else {
				log.Logger().Infof("%s└ %s version %s", strings.Repeat(" ", indent), util.ColorInfo(extension.FullyQualifiedName()), util.ColorInfo(extension.Version))
			}
		}
	}

	for _, childRef := range extension.Children {
		if child, ok := lookup[childRef]; ok {
			e, err := o.UpsertExtension(&child, exts, installedExtensions, extensionConfig, lookup, depth+1, initialIndent)
			if err != nil {
				return result, err
			}
			result = append(result, e...)
		} else {
			errors.New(fmt.Sprintf("Unable to locate extension %s", childRef))
		}
	}
	return result, nil
}

func (o *UpgradeExtensionsOptions) Contains(whens []jenkinsv1.ExtensionWhen, when jenkinsv1.ExtensionWhen) bool {
	for _, w := range whens {
		if when == w {
			return true
		}
	}
	return false
}

func (o *UpgradeExtensionsOptions) GetInstalledExtensions(extensions typev1.ExtensionInterface) (installedExtensions map[string]jenkinsv1.Extension, err error) {
	exts, err := extensions.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	installedExtensions = make(map[string]jenkinsv1.Extension)
	for _, ext := range exts.Items {
		if ext.Spec.UUID == "" {
			return nil, errors.New(fmt.Sprintf("Extension %s does not have a UUID", util.ColorInfo(fmt.Sprintf("%s:%s", ext.Namespace, ext.Name))))
		}
		installedExtensions[ext.Spec.UUID] = ext
	}
	return installedExtensions, nil
}
