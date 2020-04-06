package verify

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/blang/semver"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"
	"github.com/jenkins-x/jx/pkg/prow"
	"sigs.k8s.io/yaml"

	"github.com/jenkins-x/jx/pkg/boot"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cloud/buckets"
	"github.com/jenkins-x/jx/pkg/cloud/factory"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StepVerifyPreInstallOptions contains the command line flags
type StepVerifyPreInstallOptions struct {
	StepVerifyOptions
	Debug                 bool
	Dir                   string
	LazyCreate            bool
	DisableVerifyHelm     bool
	DisableVerifyPackages bool
	LazyCreateFlag        string
	Namespace             string
	ProviderValuesDir     string
	TestKanikoSecretData  string
	TestVeleroSecretData  string
	WorkloadIdentity      bool
}

// NewCmdStepVerifyPreInstall creates the `jx step verify pod` command
func NewCmdStepVerifyPreInstall(commonOpts *opts.CommonOptions) *cobra.Command {

	options := &StepVerifyPreInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "preinstall",
		Aliases: []string{"pre-install", "pre"},
		Short:   "Verifies all of the cloud infrastructure is setup before we try to boot up a cluster via 'jx boot'",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.Debug, "debug", "", false, "Output logs of any failed pod")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the install requirements file")
	cmd.Flags().StringVarP(&options.LazyCreateFlag, "lazy-create", "", "", fmt.Sprintf("Specify true/false as to whether to lazily create missing resources. If not specified it is enabled if Terraform is not specified in the %s file", config.RequirementsConfigFileName))
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "the namespace that Jenkins X will be booted into. If not specified it defaults to $DEPLOY_NAMESPACE")
	cmd.Flags().StringVarP(&options.ProviderValuesDir, "provider-values-dir", "", "", "The optional directory of kubernetes provider specific files")
	cmd.Flags().BoolVarP(&options.WorkloadIdentity, "workload-identity", "", false, "Enable this if using GKE Workload Identity to avoid reconnecting to the Cluster.")
	cmd.Flags().BoolVarP(&options.DisableVerifyPackages, "disable-verify-packages", "", false, "Disable packages verification, helpful when testing different package versions.")
	cmd.Flags().BoolVarP(&options.DisableVerifyHelm, "disable-verify-helm", "", false, "Disable Helm verification, helpful when testing different Helm versions.")

	return cmd
}

// Run implements this command
func (o *StepVerifyPreInstallOptions) Run() error {
	info := util.ColorInfo
	requirements, requirementsFileName, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}

	err = o.ConfigureCommonOptions(requirements)
	if err != nil {
		return err
	}

	requirements, err = o.gatherRequirements(requirements, requirementsFileName)
	if err != nil {
		return err
	}

	err = o.ValidateRequirements(requirements, requirementsFileName)
	if err != nil {
		return err
	}

	o.LazyCreate, err = requirements.IsLazyCreateSecrets(o.LazyCreateFlag)
	if err != nil {
		return err
	}

	// lets find the namespace to use
	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	err = o.verifyTLS(requirements)
	if err != nil {
		return errors.WithStack(err)
	}

	o.SetDevNamespace(ns)

	log.Logger().Infof("Verifying the kubernetes cluster before we try to boot Jenkins X in namespace: %s", info(ns))
	if o.LazyCreate {
		log.Logger().Infof("Trying to lazily create any missing resources to get the current cluster ready to boot Jenkins X")
	} else {
		log.Logger().Warn("Lazy create of cloud resources is disabled")
	}

	err = o.verifyDevNamespace(kubeClient, ns)
	if err != nil {
		if o.LazyCreate {
			log.Logger().Infof("Attempting to lazily create the deploy namespace %s", info(ns))

			err = kube.EnsureDevNamespaceCreatedWithoutEnvironment(kubeClient, ns)
			if err != nil {
				return errors.Wrapf(err, "failed to lazily create the namespace %s", ns)
			}
			// lets rerun the verify step to ensure its all sorted now
			err = o.verifyDevNamespace(kubeClient, ns)
			if err != nil {
				return errors.Wrapf(err, "failed to verify the namespace %s", ns)
			}
		}
	}

	err = o.verifyIngress(requirements, requirementsFileName)
	if err != nil {
		return err
	}
	no := &namespace.NamespaceOptions{}
	no.CommonOptions = o.CommonOptions
	no.Args = []string{ns}
	err = no.Run()
	if err != nil {
		return err
	}
	log.Logger().Info("\n")

	po := &StepVerifyPackagesOptions{}
	po.CommonOptions = o.CommonOptions
	po.Packages = []string{"kubectl", "git", "helm"}
	po.Dir = o.Dir
	if !o.DisableVerifyPackages {
		err = po.Run()
		if err != nil {
			return err
		}
		log.Logger().Info("\n")
	}

	err = o.VerifyInstallConfig(kubeClient, ns, requirements, requirementsFileName)
	if err != nil {
		return err
	}
	err = o.verifyStorage(requirements, requirementsFileName)
	if err != nil {
		return err
	}
	log.Logger().Info("\n")
	if !o.DisableVerifyHelm && !requirements.Helmfile {
		err = o.verifyHelm(ns)
		if err != nil {
			return err
		}
	}
	if requirements.Kaniko {
		if requirements.Cluster.Provider == cloud.GKE {
			log.Logger().Infof("Validating Kaniko secret in namespace %s", info(ns))

			err = o.validateKaniko(ns)
			if err != nil {
				if o.LazyCreate {
					log.Logger().Infof("attempting to lazily create the deploy namespace %s", info(ns))

					err = o.lazyCreateKanikoSecret(requirements, ns)
					if err != nil {
						return errors.Wrapf(err, "failed to lazily create the kaniko secret in: %s", ns)
					}
					// lets rerun the verify step to ensure its all sorted now
					err = o.validateKaniko(ns)
				}
			}
			if err != nil {
				return err
			}
			log.Logger().Info("\n")
		}
	}

	if vns := requirements.Velero.Namespace; vns != "" {
		if requirements.Cluster.Provider == cloud.GKE {
			log.Logger().Infof("Validating the velero secret in namespace %s", info(vns))

			err = o.validateVelero(vns)
			if err != nil {
				if o.LazyCreate {
					log.Logger().Infof("Attempting to lazily create the deploy namespace %s", info(vns))

					err = o.lazyCreateVeleroSecret(requirements, vns)
					if err != nil {
						return errors.Wrapf(err, "failed to lazily create the velero secret in: %s", vns)
					}
					// lets rerun the verify step to ensure its all sorted now
					err = o.validateVelero(vns)
				}
			}
			if err != nil {
				return err
			}
			log.Logger().Info("\n")
		}
	}

	if requirements.Webhook == config.WebhookTypeLighthouse {
		// we don't need the ConfigMaps for prow yet
		err = o.verifyProwConfigMaps(kubeClient, ns)
		if err != nil {
			return err
		}
	}

	if requirements.Cluster.Provider == cloud.EKS && o.LazyCreate {
		if !cluster.IsInCluster() || os.Getenv("OVERRIDE_IRSA_IN_CLUSTER") == "true" {
			log.Logger().Info("Attempting to lazily create the IAM Role for Service Accounts permissions")
			err = amazon.EnableIRSASupportInCluster(requirements)
			if err != nil {
				return errors.Wrap(err, "error enabling IRSA in cluster")
			}
			err = amazon.CreateIRSAManagedServiceAccounts(requirements, o.ProviderValuesDir)
			if err != nil {
				return errors.Wrap(err, "error creating the IRSA managed Service Accounts")
			}
		} else {
			log.Logger().Info("Running in cluster, not recreating permissions")
		}
	}

	// Lets update the TeamSettings with the VersionStream data from the jx-requirements.yml file so we make sure
	// we are upgrading with the latest versions
	log.Logger().Infof("Cluster looks good, you are ready to '%s' now!", info("jx boot"))
	fmt.Println()
	return nil
}

// EnsureHelm ensures helm is installed
func (o *StepVerifyPreInstallOptions) verifyHelm(ns string) error {
	log.Logger().Debug("Verifying Helm...")
	// lets make sure we don't try use tiller
	o.EnableRemoteKubeCluster()
	v, err := o.Helm().Version(false)
	if err != nil {
		err = o.InstallHelm()
		if err != nil {
			return errors.Wrap(err, "failed to install Helm")
		}
		v, err = o.Helm().Version(false)
		if err != nil {
			return errors.Wrap(err, "failed to get Helm version after install")
		}
	}
	currVersion, err := semver.Make(v)
	if err != nil {
		return errors.Wrapf(err, "unable to parse semantic version %s", v)
	}
	noInitRequiredVersion := semver.MustParse("3.0.0")
	if currVersion.LT(noInitRequiredVersion) {
		cfg := opts.InitHelmConfig{
			Namespace:       ns,
			OnlyHelmClient:  true,
			Helm3:           false,
			SkipTiller:      true,
			GlobalTiller:    false,
			TillerNamespace: "",
			TillerRole:      "",
		}
		err = o.InitHelm(cfg)
		if err != nil {
			return errors.Wrapf(err, "initializing helm with config: %v", cfg)
		}
	}

	o.EnableRemoteKubeCluster()

	_, err = o.AddHelmBinaryRepoIfMissing(kube.DefaultChartMuseumURL, kube.DefaultChartMuseumJxRepoName, "", "")
	if err != nil {
		return errors.Wrapf(err, "adding '%s' helm charts repository", kube.DefaultChartMuseumURL)
	}
	log.Logger().Infof("Ensuring Helm chart repository %s is configured\n", kube.DefaultChartMuseumURL)

	return nil
}

func (o *StepVerifyPreInstallOptions) verifyDevNamespace(kubeClient kubernetes.Interface, ns string) error {
	log.Logger().Debug("Verifying Dev Namespace...")
	ns, envName, err := kube.GetDevNamespace(kubeClient, ns)
	if err != nil {
		return err
	}
	if ns == "" {
		return fmt.Errorf("no dev namespace name found")
	}
	if envName == "" {
		return fmt.Errorf("namespace %s has no team label", ns)
	}
	return nil
}

func (o *StepVerifyPreInstallOptions) lazyCreateKanikoSecret(requirements *config.RequirementsConfig, ns string) error {
	log.Logger().Debugf("Lazily creating the kaniko secret")
	io := &create.InstallOptions{}
	io.CommonOptions = o.CommonOptions
	io.Flags.Kaniko = true
	io.Flags.Namespace = ns
	io.Flags.Provider = requirements.Cluster.Provider
	io.SetInstallValues(map[string]string{
		kube.ClusterName: requirements.Cluster.ClusterName,
		kube.ProjectID:   requirements.Cluster.ProjectID,
	})
	if o.TestKanikoSecretData != "" {
		io.AdminSecretsService.Flags.KanikoSecret = o.TestKanikoSecretData
	} else {
		err := io.ConfigureKaniko()
		if err != nil {
			return err
		}
	}
	data := io.AdminSecretsService.Flags.KanikoSecret
	if data == "" {
		return fmt.Errorf("failed to create the kaniko secret data")
	}
	return o.createSecret(ns, kube.SecretKaniko, kube.SecretKaniko, data)
}

func (o *StepVerifyPreInstallOptions) lazyCreateVeleroSecret(requirements *config.RequirementsConfig, ns string) error {
	log.Logger().Debugf("Lazily creating the velero secret")
	var data string
	var err error
	if o.TestVeleroSecretData != "" {
		data = o.TestVeleroSecretData
	} else {
		data, err = o.configureVelero(requirements)
		if err != nil {
			return errors.Wrap(err, "failed to create the velero secret data")
		}
	}
	if data == "" {
		return nil
	}
	return o.createSecret(ns, kube.SecretVelero, "cloud", data)
}

// ConfigureVelero configures the velero SA and secret
func (o *StepVerifyPreInstallOptions) configureVelero(requirements *config.RequirementsConfig) (string, error) {
	if requirements.Cluster.Provider != cloud.GKE {
		log.Logger().Infof("we are assuming your IAM roles are setup so that Velero has cluster-admin\n")
		return "", nil
	}

	serviceAccountDir, err := ioutil.TempDir("", "gke")
	if err != nil {
		return "", errors.Wrap(err, "creating a temporary folder where the service account will be stored")
	}
	defer os.RemoveAll(serviceAccountDir)

	clusterName := requirements.Cluster.ClusterName
	projectID := requirements.Cluster.ProjectID
	if projectID == "" || clusterName == "" {
		if kubeClient, ns, err := o.KubeClientAndDevNamespace(); err == nil {
			if data, err := kube.ReadInstallValues(kubeClient, ns); err == nil && data != nil {
				if projectID == "" {
					projectID = data[kube.ProjectID]
				}
				if clusterName == "" {
					clusterName = data[kube.ClusterName]
				}
			}
		}
	}
	if projectID == "" {
		projectID, err = o.GetGoogleProjectID("")
		if err != nil {
			return "", errors.Wrap(err, "getting the GCP project ID")
		}
		requirements.Cluster.ProjectID = projectID
	}
	if clusterName == "" {
		clusterName, err = o.GetGKEClusterNameFromContext()
		if err != nil {
			return "", errors.Wrap(err, "gettting the GKE cluster name from current context")
		}
		requirements.Cluster.ClusterName = clusterName
	}

	serviceAccountName := requirements.Velero.ServiceAccount
	if serviceAccountName == "" {
		serviceAccountName = naming.ToValidNameTruncated(fmt.Sprintf("%s-vo", clusterName), 30)
		requirements.Velero.ServiceAccount = serviceAccountName
	}
	log.Logger().Infof("Configuring Velero service account %s for project %s", util.ColorInfo(serviceAccountName), util.ColorInfo(projectID))
	serviceAccountPath, err := o.GCloud().GetOrCreateServiceAccount(serviceAccountName, projectID, serviceAccountDir, gke.VeleroServiceAccountRoles)
	if err != nil {
		return "", errors.Wrap(err, "creating the service account")
	}

	bucket := requirements.Storage.Backup.URL
	if bucket == "" {
		return "", fmt.Errorf("missing requirements.storage.backup.url")
	}
	err = o.GCloud().ConfigureBucketRoles(projectID, serviceAccountName, bucket, gke.VeleroServiceAccountRoles)
	if err != nil {
		return "", errors.Wrap(err, "associate the IAM roles to the bucket")
	}

	serviceAccount, err := ioutil.ReadFile(serviceAccountPath)
	if err != nil {
		return "", errors.Wrapf(err, "reading the service account from file '%s'", serviceAccountPath)
	}
	return string(serviceAccount), nil
}

// VerifyInstallConfig lets ensure we modify the install ConfigMap with the requirements
func (o *StepVerifyPreInstallOptions) VerifyInstallConfig(kubeClient kubernetes.Interface, ns string, requirements *config.RequirementsConfig, requirementsFileName string) error {
	log.Logger().Debug("Verifying Install Config...")
	_, err := kube.DefaultModifyConfigMap(kubeClient, ns, kube.ConfigMapNameJXInstallConfig,
		func(configMap *corev1.ConfigMap) error {
			secretsLocation := string(secrets.FileSystemLocationKind)
			if requirements.SecretStorage == config.SecretStorageTypeVault {
				secretsLocation = string(secrets.VaultLocationKind)
			}
			modifyMapIfNotBlank(configMap.Data, kube.KubeProvider, requirements.Cluster.Provider)
			modifyMapIfNotBlank(configMap.Data, kube.ProjectID, requirements.Cluster.ProjectID)
			modifyMapIfNotBlank(configMap.Data, kube.ClusterName, requirements.Cluster.ClusterName)
			modifyMapIfNotBlank(configMap.Data, secrets.SecretsLocationKey, secretsLocation)
			modifyMapIfNotBlank(configMap.Data, kube.Region, requirements.Cluster.Region)
			modifyMapIfNotBlank(configMap.Data, kube.Zone, requirements.Cluster.Zone)
			return nil
		}, nil)
	if err != nil {
		return errors.Wrapf(err, "saving secrets location in ConfigMap %s in namespace %s", kube.ConfigMapNameJXInstallConfig, ns)
	}
	return nil
}

// gatherRequirements gathers cluster requirements and connects to the cluster if required
func (o *StepVerifyPreInstallOptions) gatherRequirements(requirements *config.RequirementsConfig, requirementsFileName string) (*config.RequirementsConfig, error) {
	log.Logger().Debug("Gathering Requirements...")
	if o.BatchMode {
		msg := "please specify '%s' in jx-requirements when running  in  batch mode"
		if requirements.Cluster.Provider == "" {
			return nil, errors.Errorf(msg, "provider")
		}
		if requirements.Cluster.Provider == cloud.EKS || requirements.Cluster.Provider == cloud.AWS {
			if requirements.Cluster.Region == "" {
				return nil, errors.Errorf(msg, "region")
			}
		}
		if requirements.Cluster.Provider == cloud.GKE {
			if requirements.Cluster.ProjectID == "" {
				return nil, errors.Errorf(msg, "project")
			}
			if requirements.Cluster.Zone == "" {
				return nil, errors.Errorf(msg, "zone")
			}
		}
		if requirements.Cluster.EnvironmentGitOwner == "" {
			return nil, errors.Errorf(msg, "environmentGitOwner")
		}
		if requirements.Cluster.ClusterName == "" {
			return nil, errors.Errorf(msg, "clusterName")
		}
	}
	var err error
	if requirements.Cluster.Provider == "" {
		requirements.Cluster.Provider, err = util.PickName(cloud.KubernetesProviders, "Select Kubernetes provider", "the type of Kubernetes installation", o.GetIOFileHandles())
		if err != nil {
			return nil, errors.Wrap(err, "selecting Kubernetes provider")
		}
	}

	if requirements.Cluster.Provider != cloud.GKE && requirements.Cluster.Provider != cloud.EKS {
		// lets check we want to try installation as we've only tested on GKE at the moment
		if answer, err := o.showProvideFeedbackMessage(); err != nil {
			return requirements, err
		} else if !answer {
			return requirements, errors.New("finishing execution")
		}
	}

	if requirements.Cluster.Provider == cloud.GKE {
		var currentProject, currentZone, currentClusterName string
		autoAcceptDefaults := false
		if requirements.Cluster.ProjectID == "" || requirements.Cluster.Zone == "" || requirements.Cluster.ClusterName == "" {
			kubeConfig, _, err := o.Kube().LoadConfig()
			if err != nil {
				return nil, errors.Wrapf(err, "loading kubeconfig")
			}
			context := kube.Cluster(kubeConfig)
			currentProject, currentZone, currentClusterName, err = gke.ParseContext(context)
			if err != nil {
				return nil, errors.Wrapf(err, "")
			}
			if currentClusterName != "" && currentProject != "" && currentZone != "" {
				log.Logger().Infof("Currently connected cluster is %s in %s in project %s", util.ColorInfo(currentClusterName), util.ColorInfo(currentZone), util.ColorInfo(currentProject))
				autoAcceptDefaults, err = util.Confirm(fmt.Sprintf("Do you want to jx boot the %s cluster?", util.ColorInfo(currentClusterName)), true, "Enter Y to use the currently connected cluster or enter N to specify a different cluster", o.GetIOFileHandles())
				if err != nil {
					return nil, err
				}
			} else {
				log.Logger().Infof("Enter the cluster you want to jx boot")
			}
		}

		if requirements.Cluster.ProjectID == "" {
			if autoAcceptDefaults && currentProject != "" {
				requirements.Cluster.ProjectID = currentProject
			} else {
				requirements.Cluster.ProjectID, err = o.GetGoogleProjectID(currentProject)
				if err != nil {
					return nil, errors.Wrap(err, "getting project ID")
				}
			}
		}
		if requirements.Cluster.Zone == "" {
			if autoAcceptDefaults && currentZone != "" {
				requirements.Cluster.Zone = currentZone
			} else {
				requirements.Cluster.Zone, err = o.GetGoogleZone(requirements.Cluster.ProjectID, currentZone)
				if err != nil {
					return nil, errors.Wrap(err, "getting GKE Zone")
				}
			}
		}
		if requirements.Cluster.ClusterName == "" {
			if autoAcceptDefaults && currentClusterName != "" {
				requirements.Cluster.ClusterName = currentClusterName
			} else {
				requirements.Cluster.ClusterName, err = util.PickValue("Cluster name", currentClusterName, true,
					"The name for your cluster", o.GetIOFileHandles())
				if err != nil {
					return nil, errors.Wrap(err, "getting cluster name")
				}
				if requirements.Cluster.ClusterName == "" {
					return nil, errors.Errorf("no cluster name provided")
				}
			}
		}
		if !autoAcceptDefaults {
			if !o.WorkloadIdentity && !o.InCluster() {
				// connect to the specified cluster if different from the currently connected one
				log.Logger().Infof("Connecting to cluster %s", util.ColorInfo(requirements.Cluster.ClusterName))
				err = o.GCloud().ConnectToCluster(requirements.Cluster.ProjectID, requirements.Cluster.Zone, requirements.Cluster.ClusterName)
				if err != nil {
					return nil, err
				}
			} else {
				log.Logger().Info("no need to reconnect to cluster")
			}
		}
	} else if requirements.Cluster.Provider == cloud.EKS || requirements.Cluster.Provider == cloud.AWS {
		var currentRegion, currentClusterName string
		var autoAcceptDefaults bool
		if requirements.Cluster.Region == "" || requirements.Cluster.ClusterName == "" {
			currentClusterName, currentRegion, err = session.GetCurrentlyConnectedRegionAndClusterName()
			if err != nil {
				return requirements, errors.Wrap(err, "there was a problem obtaining the current cluster name and region")
			}
			if currentClusterName != "" && currentRegion != "" {
				log.Logger().Infof("")
				log.Logger().Infof("Currently connected cluster is %s in region %s", util.ColorInfo(currentClusterName), util.ColorInfo(currentRegion))
				autoAcceptDefaults, err = util.Confirm(fmt.Sprintf("Do you want to jx boot the %s cluster?", util.ColorInfo(currentClusterName)), true, "Enter Y to use the currently connected cluster or enter N to specify a different cluster", o.GetIOFileHandles())
				if err != nil {
					return nil, err
				}
			} else {
				log.Logger().Infof("Enter the cluster you want to jx boot")
			}
		}

		if requirements.Cluster.Region == "" {
			if autoAcceptDefaults && currentRegion != "" {
				requirements.Cluster.Region = currentRegion
			}
		}
		if requirements.Cluster.ClusterName == "" {
			if autoAcceptDefaults && currentClusterName != "" {
				requirements.Cluster.ClusterName = currentClusterName
			} else {
				requirements.Cluster.ClusterName, err = util.PickValue("Cluster name", currentClusterName, true,
					"The name for your cluster", o.GetIOFileHandles())
				if err != nil {
					return nil, errors.Wrap(err, "getting cluster name")
				}
			}
		}
	}

	if requirements.Cluster.ClusterName == "" && !o.BatchMode {
		requirements.Cluster.ClusterName, err = util.PickValue("Cluster name", "", true,
			"The name for your cluster", o.GetIOFileHandles())
		if err != nil {
			return nil, errors.Wrap(err, "getting cluster name")
		}
		if requirements.Cluster.ClusterName == "" {
			return nil, errors.Errorf("no cluster name provided")
		}
	}

	requirements.Cluster.Provider = strings.TrimSpace(strings.ToLower(requirements.Cluster.Provider))
	requirements.Cluster.ProjectID = strings.TrimSpace(requirements.Cluster.ProjectID)
	requirements.Cluster.Zone = strings.TrimSpace(strings.ToLower(requirements.Cluster.Zone))
	requirements.Cluster.Region = strings.TrimSpace(strings.ToLower(requirements.Cluster.Region))
	requirements.Cluster.ClusterName = strings.TrimSpace(strings.ToLower(requirements.Cluster.ClusterName))

	err = o.gatherGitRequirements(requirements)
	if err != nil {
		return nil, errors.Wrap(err, "error gathering git requirements")
	}

	// Lock the version stream to a tag
	if requirements.VersionStream.Ref == "" {
		requirements.VersionStream.Ref = os.Getenv(boot.VersionsRepoBaseRefEnvVarName)
	}
	if requirements.VersionStream.URL == "" {
		requirements.VersionStream.URL = os.Getenv(boot.VersionsRepoURLEnvVarName)
	}

	// attempt to resolve the version stream ref to a tag
	_, ref, err := o.CloneJXVersionsRepo(requirements.VersionStream.URL, requirements.VersionStream.Ref)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving version stream ref")
	}
	if ref != "" && ref != requirements.VersionStream.Ref {
		log.Logger().Infof("Locking version stream %s to release %s. Jenkins X will use this release rather than %s to resolve all versions from now on.", util.ColorInfo(requirements.VersionStream.URL), util.ColorInfo(ref), requirements.VersionStream.Ref)
		requirements.VersionStream.Ref = ref
	}

	err = o.SaveConfig(requirements, requirementsFileName)
	if err != nil {
		return nil, errors.Wrap(err, "error saving requirements file")
	}

	err = o.writeOwnersFile(requirements)
	if err != nil {
		return nil, errors.Wrapf(err, "writing approvers to OWNERS file in %s", o.Dir)
	}

	return requirements, nil
}

func (o *StepVerifyPreInstallOptions) writeOwnersFile(requirements *config.RequirementsConfig) error {
	if len(requirements.Cluster.DevEnvApprovers) > 0 {
		path := filepath.Join(o.Dir, "OWNERS")
		filename, err := filepath.Abs(path)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve path %s", path)
		}
		data := prow.Owners{}
		for _, approver := range requirements.Cluster.DevEnvApprovers {
			data.Approvers = append(data.Approvers, approver)
			data.Reviewers = append(data.Reviewers, approver)
		}
		ownersYaml, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filename, ownersYaml, 0644)
		if err != nil {
			return err
		}
		log.Logger().Infof("writing the following to the OWNERS file for the development environment repository:\n%s", string(ownersYaml))
	}
	return nil
}

func (o *StepVerifyPreInstallOptions) gatherGitRequirements(requirements *config.RequirementsConfig) error {
	requirements.Cluster.EnvironmentGitOwner = strings.TrimSpace(requirements.Cluster.EnvironmentGitOwner)

	// lets fix up any missing or incorrect git kinds for public git servers
	if gits.IsGitHubServerURL(requirements.Cluster.GitServer) {
		requirements.Cluster.GitKind = "github"
	} else if gits.IsGitLabServerURL(requirements.Cluster.GitServer) {
		requirements.Cluster.GitKind = "gitlab"
	}

	var err error
	if requirements.Cluster.EnvironmentGitOwner == "" {
		requirements.Cluster.EnvironmentGitOwner, err = util.PickValue(
			"Git Owner name for environment repositories",
			"",
			true,
			"Jenkins X leverages GitOps to track and control what gets deployed into environments.  "+
				"This requires a Git repository per environment. "+
				"This question is asking for the Git Owner where these repositories will live.",
			o.GetIOFileHandles())
		if err != nil {
			return errors.Wrap(err, "error configuring git owner for env repositories")
		}

		if requirements.Cluster.EnvironmentGitPublic {
			log.Logger().Infof("Environment repos will be %s, if you want to create %s environment repos, please set %s to %s jx-requirements.yml", util.ColorInfo("public"), util.ColorInfo("private"), util.ColorInfo("environmentGitPublic"), util.ColorInfo("false"))
		} else {
			err = o.verifyPrivateRepos(requirements)
			if err != nil {
				return err
			}
		}
	}
	if len(requirements.Cluster.DevEnvApprovers) == 0 && !o.BatchMode {
		approversString, err := util.PickValue(
			"Comma-separated git provider usernames of approvers for development environment repository",
			"",
			true,
			"Pull requests to the development environment repository require approval by one or more "+
				"users, specified in the 'OWNERS' file in the repository. Please specify a comma-separated "+
				"list of usernames for your Git provider to be used as approvers.",
			o.GetIOFileHandles())
		if err != nil {
			return errors.Wrap(err, "configuring approvers for development environment repository")
		}
		for _, a := range strings.Split(approversString, ",") {
			requirements.Cluster.DevEnvApprovers = append(requirements.Cluster.DevEnvApprovers, strings.TrimSpace(a))
		}
	}
	return nil
}

func (o *StepVerifyPreInstallOptions) verifyPrivateRepos(requirements *config.RequirementsConfig) error {
	log.Logger().Infof("Environment repos will be %s, if you want to create %s environment repos, please set %s to %s in jx-requirements.yml", util.ColorInfo("private"), util.ColorInfo("public"), util.ColorInfo("environmentGitPublic"), util.ColorInfo("true"))

	if o.BatchMode {
		return nil
	}

	if requirements.Cluster.GitKind == "github" {
		message := fmt.Sprintf("If '%s' is an GitHub organisation it needs to have a paid subscription to create private repos. Do you wish to continue?", requirements.Cluster.EnvironmentGitOwner)
		help := fmt.Sprint("GitHub organisation on a free plan cannot create private repositories. You either need to upgrade, use a GitHub user instead or use public repositories.")
		if answer, err := util.Confirm(message, false, help, o.GetIOFileHandles()); err != nil {
			return err
		} else if !answer {
			return errors.New("cannot continue without completed git requirements")
		}
	}
	return nil
}

// verifyStorage verifies the associated buckets exist or if enabled lazily create them
func (o *StepVerifyPreInstallOptions) verifyStorage(requirements *config.RequirementsConfig, requirementsFileName string) error {
	log.Logger().Info("Verifying Storage...")
	storage := &requirements.Storage
	err := o.verifyStorageEntry(requirements, requirementsFileName, &storage.Logs, "logs", "Long term log storage")
	if err != nil {
		return err
	}
	err = o.verifyStorageEntry(requirements, requirementsFileName, &storage.Reports, "reports", "Long term report storage")
	if err != nil {
		return err
	}
	err = o.verifyStorageEntry(requirements, requirementsFileName, &storage.Repository, "repository", "Chart repository")
	if err != nil {
		return err
	}
	err = o.verifyStorageEntry(requirements, requirementsFileName, &storage.Backup, "backup", "backup storage")
	if err != nil {
		return err
	}
	log.Logger().Infof("Storage configuration looks good\n")
	return nil
}

func (o *StepVerifyPreInstallOptions) verifyTLS(requirements *config.RequirementsConfig) error {
	if !requirements.Ingress.TLS.Enabled {
		confirm := false
		if requirements.SecretStorage == config.SecretStorageTypeVault {
			log.Logger().Warnf("Vault is enabled and TLS is not enabled. This means your secrets will be sent to and from your cluster in the clear. See %s for more information", config.TLSDocURL)
			confirm = true
		}
		if requirements.Webhook != config.WebhookTypeNone {
			log.Logger().Warnf("TLS is not enabled so your webhooks will be called using HTTP. This means your webhook secret will be sent to your cluster in the clear. See %s for more information", config.TLSDocURL)
			confirm = true
		}
		if os.Getenv(boot.OverrideTLSWarningEnvVarName) == "true" {
			confirm = false
		}
		if confirm && !o.BatchMode {

			message := fmt.Sprintf("Do you wish to continue?")
			help := fmt.Sprintf("Jenkins X needs TLS enabled to send secrets securely. We strongly recommend enabling TLS.")
			if answer, err := util.Confirm(message, false, help, o.GetIOFileHandles()); err != nil {
				return err
			} else if !answer {
				return errors.Errorf("cannot continue because TLS is not enabled.")
			}
		}

	}
	return nil
}

func (o *StepVerifyPreInstallOptions) verifyStorageEntry(requirements *config.RequirementsConfig, requirementsFileName string, storageEntryConfig *config.StorageEntryConfig, name string, text string) error {
	kubeProvider := requirements.Cluster.Provider
	if !storageEntryConfig.Enabled {
		if requirements.IsCloudProvider() {
			log.Logger().Warnf("Your requirements have not enabled cloud storage for %s - we recommend enabling this for kubernetes provider %s", name, kubeProvider)
		}
		return nil
	}

	provider := factory.NewBucketProvider(requirements)

	if storageEntryConfig.URL == "" {
		// lets allow the storage bucket to be entered or created
		if o.BatchMode {
			log.Logger().Warnf("No URL provided for storage: %s", name)
			return nil
		}
		scheme := buckets.KubeProviderToBucketScheme(kubeProvider)
		if scheme == "" {
			scheme = "s3"
		}
		message := fmt.Sprintf("%s bucket URL. Press enter to create and use a new bucket", text)
		help := fmt.Sprintf("please enter the URL of the bucket to use for storage using the format %s://<bucket-name>", scheme)
		value, err := util.PickValue(message, "", false, help, o.GetIOFileHandles())
		if err != nil {
			return errors.Wrapf(err, "failed to pick storage bucket for %s", name)
		}

		if value == "" {
			if provider == nil {
				log.Logger().Warnf("the kubernetes provider %s has no BucketProvider in jx yet so we cannot lazily create buckets", kubeProvider)
				log.Logger().Warnf("long term storage for %s will be disabled until you provide an existing bucket URL", name)
				return nil
			}
			safeClusterName := naming.ToValidName(requirements.Cluster.ClusterName)
			safeName := naming.ToValidName(name)
			value, err = provider.CreateNewBucketForCluster(safeClusterName, safeName)
			if err != nil {
				return errors.Wrapf(err, "failed to create a dynamic bucket for cluster %s and name %s", safeClusterName, safeName)
			}
		}
		if value != "" {
			storageEntryConfig.URL = value

			err = o.SaveConfig(requirements, requirementsFileName)
			if err != nil {
				return errors.Wrapf(err, "failed to save changes to file: %s", requirementsFileName)
			}
		}
	}

	if storageEntryConfig.URL != "" {
		if provider == nil {
			log.Logger().Warnf("the kubernetes provider %s has no BucketProvider in jx yet - so you have to manually setup and verify your bucket URLs exist", kubeProvider)
			log.Logger().Infof("please verify this bucket exists: %s", util.ColorInfo(storageEntryConfig.URL))
			return nil
		}

		err := provider.EnsureBucketIsCreated(storageEntryConfig.URL)
		if err != nil {
			return errors.Wrapf(err, "failed to ensure the bucket URL %s is created", storageEntryConfig.URL)
		}
	}
	return nil
}

func (o *StepVerifyPreInstallOptions) verifyProwConfigMaps(kubeClient kubernetes.Interface, ns string) error {
	err := o.verifyConfigMapExists(kubeClient, ns, "config", "config.yaml", "pod_namespace: jx")
	if err != nil {
		return err
	}
	return o.verifyConfigMapExists(kubeClient, ns, "plugins", "plugins.yaml", "cat: {}")
}

func (o *StepVerifyPreInstallOptions) verifyConfigMapExists(kubeClient kubernetes.Interface, ns string, name string, key string, defaultValue string) error {
	info := util.ColorInfo
	configMapInterface := kubeClient.CoreV1().ConfigMaps(ns)
	cm, err := configMapInterface.Get(name, metav1.GetOptions{})
	if err != nil {
		// lets try create it
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Data: map[string]string{
				key: defaultValue,
			},
		}
		cm, err = configMapInterface.Create(cm)
		if err != nil {
			// maybe someone else just created it - lets try one more time
			cm2, err2 := configMapInterface.Get(name, metav1.GetOptions{})
			if err == nil {
				log.Logger().Infof("created ConfigMap %s in namespace %s", info(name), info(ns))
			}
			if err2 != nil {
				return fmt.Errorf("failed to create the ConfigMap %s in namespace %s due to: %s - we cannot get it either: %s", name, ns, err.Error(), err2.Error())
			}
			cm = cm2
			err = nil
		}
	}
	if err != nil {
		return err
	}

	// lets verify that there is an entry
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	_, ok := cm.Data[key]
	if !ok {
		cm.Data[key] = defaultValue
		cm.Name = name

		_, err = configMapInterface.Update(cm)
		if err != nil {
			return fmt.Errorf("failed to update the ConfigMap %s in namespace %s to add key %s due to: %s", name, ns, key, err.Error())
		}
	}
	log.Logger().Infof("verified there is a ConfigMap %s in namespace %s", info(name), info(ns))
	return nil
}

func (o *StepVerifyPreInstallOptions) verifyIngress(requirements *config.RequirementsConfig, requirementsFileName string) error {
	log.Logger().Info("Verifying Ingress...")
	domain := requirements.Ingress.Domain
	if requirements.Ingress.IsAutoDNSDomain() && !requirements.Ingress.IgnoreLoadBalancer {
		log.Logger().Infof("Clearing the domain %s as when using auto-DNS domains we need to regenerate to ensure its always accurate in case the cluster or ingress service is recreated", util.ColorInfo(domain))
		requirements.Ingress.Domain = ""
		err := o.SaveConfig(requirements, requirementsFileName)
		if err != nil {
			return errors.Wrapf(err, "failed to save changes to file: %s", requirementsFileName)
		}
	}
	log.Logger().Info("\n")
	return nil
}

// ValidateRequirements validate the requirements; e.g. the webhook and git provider
func (o *StepVerifyPreInstallOptions) ValidateRequirements(requirements *config.RequirementsConfig, fileName string) error {
	if requirements.Webhook == config.WebhookTypeProw {
		kind := requirements.Cluster.GitKind
		server := requirements.Cluster.GitServer
		if (kind != "" && kind != "github") || (server != "" && !gits.IsGitHubServerURL(server)) {
			return fmt.Errorf("invalid requirements in file %s cannot use prow as a webhook for git kind: %s server: %s. Please try using lighthouse instead", fileName, kind, server)
		}
	}
	if requirements.Repository == config.RepositoryTypeBucketRepo && requirements.Cluster.ChartRepository == "" {
		requirements.Cluster.ChartRepository = "http://bucketrepo/bucketrepo/charts/"
		err := o.SaveConfig(requirements, fileName)
		if err != nil {
			return errors.Wrapf(err, "failed to save changes to file: %s", fileName)
		}
	}

	// lets verify that we have a repository name defined for every environment
	modified := false
	for i, env := range requirements.Environments {
		if env.Repository == "" {
			clusterName := requirements.Cluster.ClusterName
			if clusterName != "" {
				clusterName = clusterName + "-"
			}
			repoName := "environment-" + clusterName + env.Key
			requirements.Environments[i].Repository = naming.ToValidName(repoName)
			modified = true
		}
	}
	if modified {
		err := o.SaveConfig(requirements, fileName)
		if err != nil {
			return errors.Wrapf(err, "failed to save changes to file: %s", fileName)
		}
	}
	return nil
}

// SaveConfig saves the configuration file to the given project directory
func (o *StepVerifyPreInstallOptions) SaveConfig(c *config.RequirementsConfig, fileName string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", fileName)
	}

	if c.Helmfile {
		y := config.RequirementsValues{
			RequirementsConfig: c,
		}
		data, err = yaml.Marshal(y)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(path.Join(path.Dir(fileName), config.RequirementsValuesFileName), data, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to save file %s", config.RequirementsValuesFileName)
		}
	}
	return nil
}

func modifyMapIfNotBlank(m map[string]string, key string, value string) {
	if m != nil {
		if value != "" {
			m[key] = value
		} else {
			log.Logger().Debugf("Cannot update key %s, value is nil", key)
		}
	} else {
		log.Logger().Debugf("Cannot update key %s, map is nil", key)
	}
}

func (o *StepVerifyPreInstallOptions) showProvideFeedbackMessage() (bool, error) {
	log.Logger().Info("jx boot has only been validated on GKE and EKS, we'd love feedback and contributions for other Kubernetes providers")
	if !o.BatchMode {
		return util.Confirm("Continue execution anyway?",
			true, "", o.GetIOFileHandles())
	}
	log.Logger().Info("Running in Batch Mode, execution will continue")
	return true, nil
}
