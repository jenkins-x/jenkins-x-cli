package create

import (
	"fmt"
	"strings"
	"time"

	randomdata "github.com/Pallinder/go-randomdata"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/pkg/errors"
	survey "gopkg.in/AlecAivazis/survey.v1"

	osUser "os/user"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/features"
	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CreateClusterOptions the flags for running create cluster
type CreateClusterGKEOptions struct {
	CreateClusterOptions

	Flags CreateClusterGKEFlags `mapstructure:"cluster"`
}

type CreateClusterGKEFlags struct {
	AutoUpgrade              bool   `mapstructure:"enable-autoupgrade"`
	ClusterName              string `mapstructure:"cluster-name"`
	ClusterIpv4Cidr          string `mapstructure:"cluster-ipv4-cidr"`
	ClusterVersion           string `mapstructure:"kubernetes-version"`
	DiskSize                 string `mapstructure:"disk-size"`
	ImageType                string `mapstructure:"image-type"`
	MachineType              string `mapstructure:"machine-type"`
	MinNumOfNodes            string `mapstructure:"min-num-nodes"`
	MaxNumOfNodes            string `mapstructure:"max-num-nodes"`
	Network                  string
	ProjectID                string `mapstructure:"project-id"`
	SkipLogin                bool   `mapstructure:"skip-login"`
	SubNetwork               string
	Region                   string
	Zone                     string
	Namespace                string
	Labels                   string
	EnhancedScopes           bool `mapstructure:"enhanced-scopes"`
	Scopes                   []string
	Preemptible              bool
	EnhancedApis             bool `mapstructure:"enhanced-apis"`
	UseStackDriverMonitoring bool `mapstructure:"use-stackdriver-monitoring"`
	ServiceAccount           string
}

const (
	skipLoginFlagName         = "skip-login"
	preemptibleFlagName       = "preemptible"
	enhancedAPIFlagName       = "enhanced-apis"
	enhancedScopesFlagName    = "enhanced-scopes"
	maxGKEClusterNameLength   = 27
	machineTypeFlagName       = "machine-type"
	minNodesFlagName          = "min-num-nodes"
	maxNodesFlagName          = "max-num-nodes"
	projectIDFlagName         = "project-id"
	zoneFlagName              = "zone"
	regionFlagName            = "region"
	diskSizeFlagName          = "disk-size"
	imageTypeFlagName         = "image-type"
	clusterIpv4CidrFlagName   = "cluster-ipv4-cidr"
	enableAutoupgradeFlagName = "enable-autoupgrade"
	networkFlagName           = "network"
	subNetworkFlagName        = "subnetwork"
	labelsFlagName            = "labels"
	scopeFlagName             = "scope"
	stackDriverFlagName       = "use-stackdriver-monitoring"
	serviceAccountFlagName    = "service-account"
)

var (
	createClusterGKELong = templates.LongDesc(`
		This command creates a new Kubernetes cluster on GKE, installing required local dependencies and provisions the
		Jenkins X platform

		You can see a demo of this command here: [https://jenkins-x.io/demos/create_cluster_gke/](https://jenkins-x.io/demos/create_cluster_gke/)

		Google Kubernetes Engine is a managed environment for deploying containerized applications. It brings our latest
		innovations in developer productivity, resource efficiency, automated operations, and open source flexibility to
		accelerate your time to market.

		Google has been running production workloads in containers for over 15 years, and we build the best of what we
		learn into Kubernetes, the industry-leading open source container orchestrator which powers Kubernetes Engine.

`)

	createClusterGKEExample = templates.Examples(`

		jx create cluster gke

`)
)

// NewCmdCreateClusterGKE creates a command object for the generic "init" action, which
// installs the dependencies required to run the jenkins-x platform on a Kubernetes cluster.
func NewCmdCreateClusterGKE(commonOpts *opts.CommonOptions) *cobra.Command {
	options := CreateClusterGKEOptions{
		CreateClusterOptions: createCreateClusterOptions(commonOpts, cloud.GKE),
	}
	cmd := &cobra.Command{
		Use:     "gke",
		Short:   "Create a new Kubernetes cluster on GKE: Runs on Google Cloud",
		Long:    createClusterGKELong,
		Example: createClusterGKEExample,
		PreRun: func(cmd *cobra.Command, args []string) {
			err := features.IsEnabled(cmd)
			helper.CheckErr(err)
			err = options.InstallOptions.CheckFeatures()
			helper.CheckErr(err)
		},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addCreateClusterFlags(cmd)

	cmd.Flags().StringVarP(&options.Flags.ClusterName, optionClusterName, "n", "", "The name of this cluster, default is a random generated name")
	cmd.Flags().StringVarP(&options.Flags.ClusterIpv4Cidr, clusterIpv4CidrFlagName, "", "", "The IP address range for the pods in this cluster in CIDR notation (e.g. 10.0.0.0/14)")
	cmd.Flags().StringVarP(&options.Flags.ClusterVersion, optionKubernetesVersion, "v", "", "The Kubernetes version to use for the master and nodes. Defaults to server-specified")
	cmd.Flags().StringVarP(&options.Flags.DiskSize, diskSizeFlagName, "d", "", "Size in GB for node VM boot disks. Defaults to 100GB")
	cmd.Flags().BoolVarP(&options.Flags.AutoUpgrade, enableAutoupgradeFlagName, "", false, "Sets autoupgrade feature for a cluster's default node-pool(s)")
	cmd.Flags().StringVarP(&options.Flags.MachineType, machineTypeFlagName, "m", "", "The type of machine to use for nodes")
	cmd.Flags().StringVarP(&options.Flags.MinNumOfNodes, minNodesFlagName, "", "", "The minimum number of nodes to be created in each of the cluster's zones")
	cmd.Flags().StringVarP(&options.Flags.MaxNumOfNodes, maxNodesFlagName, "", "", "The maximum number of nodes to be created in each of the cluster's zones")
	cmd.Flags().StringVarP(&options.Flags.ProjectID, projectIDFlagName, "p", "", "Google Project ID to create cluster in")
	cmd.Flags().StringVarP(&options.Flags.Network, networkFlagName, "", "", "The Compute Engine Network that the cluster will connect to")
	cmd.Flags().StringVarP(&options.Flags.ImageType, imageTypeFlagName, "", "", "The image type for the nodes in the cluster")
	cmd.Flags().StringVarP(&options.Flags.SubNetwork, subNetworkFlagName, "", "", "The Google Compute Engine subnetwork to which the cluster is connected")
	cmd.Flags().StringVarP(&options.Flags.Zone, zoneFlagName, "z", "", "The compute zone (e.g. us-central1-a) for the cluster")
	cmd.Flags().StringVarP(&options.Flags.Region, regionFlagName, "r", "", "Compute region (e.g. us-central1) for the cluster")
	cmd.Flags().BoolVarP(&options.Flags.SkipLogin, skipLoginFlagName, "", false, "Skip Google auth if already logged in via gcloud auth")
	cmd.Flags().StringVarP(&options.Flags.Labels, labelsFlagName, "", "", "The labels to add to the cluster being created such as 'foo=bar,whatnot=123'. Label names must begin with a lowercase character ([a-z]), end with a lowercase alphanumeric ([a-z0-9]) with dashes (-), and lowercase alphanumeric ([a-z0-9]) between.")
	cmd.Flags().StringArrayVarP(&options.Flags.Scopes, scopeFlagName, "", []string{}, "The OAuth scopes to be added to the cluster")
	cmd.Flags().BoolVarP(&options.Flags.Preemptible, preemptibleFlagName, "", false, "Use preemptible VMs in the node-pool")
	cmd.Flags().BoolVarP(&options.Flags.EnhancedScopes, enhancedScopesFlagName, "", false, "Use enhanced Oauth scopes for access to GCS/GCR")
	cmd.Flags().BoolVarP(&options.Flags.EnhancedApis, enhancedAPIFlagName, "", false, "Enable enhanced APIs to utilise Container Registry & Cloud Build")
	cmd.Flags().BoolVarP(&options.Flags.UseStackDriverMonitoring, stackDriverFlagName, "", true, "Enable Stackdriver Kubernetes Engine Monitoring")
	cmd.Flags().StringVarP(&options.Flags.ServiceAccount, serviceAccountFlagName, "", "", "The service account used to run the cluster")
	bindGKEConfigToFlags(cmd)

	return cmd
}

func bindGKEConfigToFlags(cmd *cobra.Command) {
	_ = viper.BindPFlag(clusterConfigKey(optionClusterName), cmd.Flags().Lookup(optionClusterName))
	_ = viper.BindPFlag(clusterConfigKey(clusterIpv4CidrFlagName), cmd.Flags().Lookup(clusterIpv4CidrFlagName))
	_ = viper.BindPFlag(clusterConfigKey(optionKubernetesVersion), cmd.Flags().Lookup(optionKubernetesVersion))
	_ = viper.BindPFlag(clusterConfigKey(diskSizeFlagName), cmd.Flags().Lookup(diskSizeFlagName))
	_ = viper.BindPFlag(clusterConfigKey(enableAutoupgradeFlagName), cmd.Flags().Lookup(enableAutoupgradeFlagName))
	_ = viper.BindPFlag(clusterConfigKey(machineTypeFlagName), cmd.Flags().Lookup(machineTypeFlagName))
	_ = viper.BindPFlag(clusterConfigKey(minNodesFlagName), cmd.Flags().Lookup(minNodesFlagName))
	_ = viper.BindPFlag(clusterConfigKey(maxNodesFlagName), cmd.Flags().Lookup(maxNodesFlagName))
	_ = viper.BindPFlag(clusterConfigKey(projectIDFlagName), cmd.Flags().Lookup(projectIDFlagName))
	_ = viper.BindPFlag(clusterConfigKey(networkFlagName), cmd.Flags().Lookup(networkFlagName))
	_ = viper.BindPFlag(clusterConfigKey(imageTypeFlagName), cmd.Flags().Lookup(imageTypeFlagName))
	_ = viper.BindPFlag(clusterConfigKey(subNetworkFlagName), cmd.Flags().Lookup(subNetworkFlagName))
	_ = viper.BindPFlag(clusterConfigKey(zoneFlagName), cmd.Flags().Lookup(zoneFlagName))
	_ = viper.BindPFlag(clusterConfigKey(regionFlagName), cmd.Flags().Lookup(regionFlagName))
	_ = viper.BindPFlag(clusterConfigKey(skipLoginFlagName), cmd.Flags().Lookup(skipLoginFlagName))
	_ = viper.BindPFlag(clusterConfigKey(labelsFlagName), cmd.Flags().Lookup(labelsFlagName))
	_ = viper.BindPFlag(clusterConfigKey(scopeFlagName), cmd.Flags().Lookup(labelsFlagName))
	_ = viper.BindPFlag(clusterConfigKey(preemptibleFlagName), cmd.Flags().Lookup(preemptibleFlagName))
	_ = viper.BindPFlag(clusterConfigKey(enhancedScopesFlagName), cmd.Flags().Lookup(enhancedScopesFlagName))
	_ = viper.BindPFlag(clusterConfigKey(enhancedAPIFlagName), cmd.Flags().Lookup(enhancedAPIFlagName))
	_ = viper.BindPFlag(clusterConfigKey(stackDriverFlagName), cmd.Flags().Lookup(stackDriverFlagName))
	_ = viper.BindPFlag(clusterConfigKey(serviceAccountFlagName), cmd.Flags().Lookup(serviceAccountFlagName))
}

func (o *CreateClusterGKEOptions) Run() error {
	err := validateClusterName(o.Flags.ClusterName)
	if err != nil {
		return err
	}

	err = o.InstallRequirements(cloud.GKE)
	if err != nil {
		return err
	}

	err = o.GetConfiguration(&o)
	if err != nil {
		return errors.Wrap(err, "getting gke cluster configuration")
	}

	err = o.createClusterGKE()
	if err != nil {
		log.Logger().Errorf("error creating cluster %v", err)
		return err
	}

	return nil
}

func (o *CreateClusterGKEOptions) createClusterGKE() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	var err error
	if !o.Flags.SkipLogin {
		err := o.RunCommandVerbose("gcloud", "auth", "login", "--brief")
		if err != nil {
			return err
		}
	}

	projectID := o.Flags.ProjectID
	if projectID == "" {
		projectID, err = o.GetGoogleProjectID("")
		if err != nil {
			return err
		}
	} else {
		log.Logger().Infof(util.QuestionAnswer("Configured project id", projectID))
	}

	err = o.RunCommandVerbose("gcloud", "config", "set", "project", projectID)
	if err != nil {
		return err
	}

	log.Logger().Debugf("Let's ensure we have %s and %s enabled on your project", util.ColorInfo("container"), util.ColorInfo("compute"))
	err = o.GCloud().EnableAPIs(projectID, "container", "compute")
	if err != nil {
		return err
	}

	advancedMode := o.AdvancedMode

	if o.Flags.ClusterName == "" {
		defaultClusterName := strings.ToLower(randomdata.SillyName())
		if len(defaultClusterName) > maxGKEClusterNameLength {
			defaultClusterName = strings.ToLower(randomdata.SillyName())
		}
		if advancedMode {
			clusterNameHelp := fmt.Sprintf("Cluster name should be less than %d chars and limited to lowercase alphanumerics and dashes", maxGKEClusterNameLength)
			var invalidClusterName = true
			for invalidClusterName {
				prompt := &survey.Input{
					Message: "What cluster name would you like to use",
					Help:    clusterNameHelp,
					Default: defaultClusterName,
				}

				err = survey.AskOne(prompt, &o.Flags.ClusterName, nil, surveyOpts)
				if err != nil {
					return err
				}
				err = validateClusterName(o.Flags.ClusterName)
				if err != nil {
					log.Logger().Infof(util.ColorAnswer(clusterNameHelp))
				} else {
					invalidClusterName = false
				}
			}
		} else {
			o.Flags.ClusterName = defaultClusterName
			log.Logger().Infof(util.QuestionAnswer("No cluster name provided so using a generated one", o.Flags.ClusterName))
		}
	} else {
		log.Logger().Infof(util.QuestionAnswer("Configured cluster name", o.Flags.ClusterName))
	}

	region := o.Flags.Region
	zone := o.Flags.Zone

	if o.InstallOptions.Flags.NextGeneration && region != "" {
		return errors.New("cannot create a regional cluster with --ng")
	}

	if !o.BatchMode {
		if zone == "" && region == "" {
			clusterType := "Zonal"

			if o.InstallOptions.Flags.NextGeneration {
				log.Logger().Infof(util.ColorWarning("Defaulting to zonal cluster type as --ng is selected."))
			} else if advancedMode {
				prompts := &survey.Select{
					Message: "What type of cluster would you like to create",
					Options: []string{"Regional", "Zonal"},
					Help:    "A Regional cluster will create a node-pool in each zone causing it to use more resources.  Please ensure you have enough quota.",
					Default: clusterType,
				}

				err = survey.AskOne(prompts, &clusterType, nil, surveyOpts)
				if err != nil {
					return err
				}
			} else {
				log.Logger().Infof(util.QuestionAnswer("Defaulting to cluster type", clusterType))
			}

			if "Regional" == clusterType {
				region, err = o.GetGoogleRegion(projectID)
				if err != nil {
					return err
				}
			} else {
				zone, err = o.GetGoogleZone(projectID, "")
				if err != nil {
					return err
				}
			}
		} else if region != "" {
			log.Logger().Infof(util.QuestionAnswer("Configured to cluster type", "Regional"))
			log.Logger().Infof(util.QuestionAnswer("Configured Google Cloud Region", region))
		} else {
			log.Logger().Infof(util.QuestionAnswer("Configured to cluster type", "Zonal"))
			log.Logger().Infof(util.QuestionAnswer("Configured Google Cloud Zone", zone))
		}
	} else {
		if zone == "" && region == "" {
			return errors.New("in batchmode, either a region or a zone must be set")
		}
	}

	machineType := o.Flags.MachineType
	if machineType == "" {
		defaultMachineType := "n1-standard-2"
		if advancedMode {
			prompts := &survey.Select{
				Message:  "Google Cloud Machine Type:",
				Options:  gke.GetGoogleMachineTypes(),
				Help:     "We recommend a minimum of n1-standard-2 for Jenkins X,  a table of machine descriptions can be found here https://cloud.google.com/kubernetes-engine/docs/concepts/cluster-architecture",
				PageSize: 10,
				Default:  defaultMachineType,
			}
			err := survey.AskOne(prompts, &machineType, nil, surveyOpts)

			if err != nil {
				return err
			}
		} else {
			machineType = defaultMachineType
			log.Logger().Infof(util.QuestionAnswer("Defaulting to machine type", machineType))
		}
	} else {
		log.Logger().Infof(util.QuestionAnswer("Configured to machine type", machineType))
	}

	minNumOfNodes := o.Flags.MinNumOfNodes
	if minNumOfNodes == "" {
		defaultNodes := "3"
		if region != "" {
			defaultNodes = "1"
		}
		if advancedMode {
			prompt := &survey.Input{
				Message: "Minimum number of Nodes (per zone)",
				Default: defaultNodes,
				Help:    "We recommend a minimum of " + defaultNodes + " for Jenkins X, the minimum number of nodes to be created in each of the cluster's zones",
			}

			err = survey.AskOne(prompt, &minNumOfNodes, nil, surveyOpts)
			if err != nil {
				return err
			}
		} else {
			minNumOfNodes = defaultNodes
			log.Logger().Infof(util.QuestionAnswer("Defaulting to minimum number of nodes", minNumOfNodes))
		}
	} else {
		log.Logger().Infof(util.QuestionAnswer("Configured to minimum number of nodes", minNumOfNodes))
	}

	maxNumOfNodes := o.Flags.MaxNumOfNodes
	if maxNumOfNodes == "" {
		defaultNodes := "5"
		if region != "" {
			defaultNodes = "2"
		}
		if advancedMode {
			prompt := &survey.Input{
				Message: "Maximum number of Nodes",
				Default: defaultNodes,
				Help:    "We recommend at least " + defaultNodes + " for Jenkins X, the maximum number of nodes to be created in each of the cluster's zones",
			}

			err = survey.AskOne(prompt, &maxNumOfNodes, nil, surveyOpts)
			if err != nil {
				return err
			}
		} else {
			maxNumOfNodes = defaultNodes
			log.Logger().Infof(util.QuestionAnswer("Defaulting to maximum number of nodes", maxNumOfNodes))
		}
	} else {
		log.Logger().Infof(util.QuestionAnswer("Configured to maximum number of nodes", maxNumOfNodes))
	}

	if !o.BatchMode {
		if !o.IsConfigExplicitlySet("cluster", preemptibleFlagName) {
			if advancedMode {
				prompt := &survey.Confirm{
					Message: "Would you like to use preemptible VMs?",
					Default: false,
					Help:    "Preemptible VMs can significantly lower the cost of a cluster",
				}
				err = survey.AskOne(prompt, &o.Flags.Preemptible, nil, surveyOpts)
				if err != nil {
					return err
				}
			} else {
				o.Flags.Preemptible = false
				log.Logger().Infof(util.QuestionAnswer("Defaulting use of preemptible VMs", util.YesNo(o.Flags.Preemptible)))
			}
		} else {
			log.Logger().Infof(util.QuestionAnswer("Configured use of preemptible VMs", util.YesNo(o.Flags.Preemptible)))
		}
	}

	// this really shouldn't be here
	if o.InstallOptions.Flags.NextGeneration || o.InstallOptions.Flags.Tekton {
		o.Flags.EnhancedApis = true
		o.Flags.EnhancedScopes = true
		o.InstallOptions.Flags.Kaniko = true
		o.InstallOptions.Flags.StaticJenkins = false
	}

	if !o.BatchMode {
		// if scopes is empty &
		if len(o.Flags.Scopes) == 0 && !o.IsConfigExplicitlySet("cluster", enhancedScopesFlagName) {
			if advancedMode {
				prompt := &survey.Confirm{
					Message: "Would you like to access Google Cloud Storage / Google Container Registry?",
					Default: o.InstallOptions.Flags.DockerRegistry == "",
					Help:    "Enables enhanced oauth scopes to allow access to storage based services",
				}
				err = survey.AskOne(prompt, &o.Flags.EnhancedScopes, nil, surveyOpts)
				if err != nil {
					return err
				}
			} else {
				o.Flags.EnhancedScopes = true
				log.Logger().Infof(util.QuestionAnswer("Defaulting access to Google Cloud Storage / Google Container Registry", util.YesNo(o.Flags.EnhancedScopes)))
			}
		} else {
			log.Logger().Infof(util.QuestionAnswer("Configured access to Google Cloud Storage / Google Container Registry", util.YesNo(o.Flags.EnhancedScopes)))
		}
	}

	if o.Flags.EnhancedScopes {
		o.Flags.Scopes = []string{"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/compute",
			"https://www.googleapis.com/auth/devstorage.full_control",
			"https://www.googleapis.com/auth/service.management",
			"https://www.googleapis.com/auth/servicecontrol",
			"https://www.googleapis.com/auth/logging.write",
			"https://www.googleapis.com/auth/monitoring"}
	}

	if !o.BatchMode {
		// only provide the option if enhanced scopes are enabled
		if o.Flags.EnhancedScopes {
			if !o.IsConfigExplicitlySet("cluster", enhancedAPIFlagName) {
				if advancedMode {
					prompt := &survey.Confirm{
						Message: "Would you like to enable Cloud Build, Container Registry & Container Analysis APIs?",
						Default: o.Flags.EnhancedScopes,
						Help:    "Enables extra APIs on the GCP project",
					}
					err = survey.AskOne(prompt, &o.Flags.EnhancedApis, nil, surveyOpts)
					if err != nil {
						return err
					}
				} else {
					o.Flags.EnhancedApis = true
					log.Logger().Infof(util.QuestionAnswer("Defaulting enabling Cloud Build, Container Registry & Container Analysis API's", util.YesNo(o.Flags.EnhancedApis)))
				}
			} else {
				log.Logger().Infof(util.QuestionAnswer("Configured access to Cloud Build, Container Registry & Container Analysis API's", util.YesNo(o.Flags.EnhancedApis)))
			}
		}
	}

	if o.Flags.EnhancedApis {
		log.Logger().Debugf("checking if we need to enable APIs for GCB and GCR")
		err = o.GCloud().EnableAPIs(projectID, "cloudbuild", "containerregistry", "containeranalysis")
		if err != nil {
			return err
		}
	}

	if !o.BatchMode {
		// only provide the option if enhanced scopes are enabled
		if o.Flags.EnhancedScopes {
			if !o.IsConfigExplicitlySet("install", "kaniko") {
				if advancedMode {
					prompt := &survey.Confirm{
						Message: "Would you like to enable Kaniko for building container images",
						Default: o.Flags.EnhancedScopes,
						Help:    "Use Kaniko for docker images",
					}
					err = survey.AskOne(prompt, &o.InstallOptions.Flags.Kaniko, nil, surveyOpts)
					if err != nil {
						return err
					}
				} else {
					o.InstallOptions.Flags.Kaniko = false
					log.Logger().Infof(util.QuestionAnswer("Defaulting enabling Kaniko for building container images", util.YesNo(o.InstallOptions.Flags.Kaniko)))
				}
			} else {
				log.Logger().Infof(util.QuestionAnswer("Configured enabling Kaniko for building container images", util.YesNo(o.InstallOptions.Flags.Kaniko)))
			}
		}
	}

	// mandatory flags are machine type, num-nodes, zone or region
	args := []string{"container", "clusters", "create",
		o.Flags.ClusterName,
		"--num-nodes", minNumOfNodes,
		"--machine-type", machineType,
		"--enable-autoscaling",
		"--min-nodes", minNumOfNodes,
		"--max-nodes", maxNumOfNodes}

	if region != "" {
		args = append(args, "--region", region)
	} else {
		args = append(args, "--zone", zone)
	}

	if o.Flags.DiskSize != "" {
		args = append(args, "--disk-size", o.Flags.DiskSize)
	}

	if o.Flags.ClusterIpv4Cidr != "" {
		args = append(args, "--cluster-ipv4-cidr", o.Flags.ClusterIpv4Cidr)
	}

	if o.Flags.ClusterVersion != "" {
		args = append(args, "--cluster-version", o.Flags.ClusterVersion)
	}

	if o.Flags.AutoUpgrade {
		args = append(args, "--enable-autoupgrade")
	}

	if o.Flags.ImageType != "" {
		args = append(args, "--image-type", o.Flags.ImageType)
	}

	if o.Flags.Network != "" {
		args = append(args, "--network", o.Flags.Network)
	}

	if o.Flags.SubNetwork != "" {
		args = append(args, "--subnetwork", o.Flags.SubNetwork)
	}

	if len(o.Flags.Scopes) > 0 {
		log.Logger().Debugf("using cluster scopes: %s", util.ColorInfo(strings.Join(o.Flags.Scopes, " ")))

		args = append(args, fmt.Sprintf("--scopes=%s", strings.Join(o.Flags.Scopes, ",")))
	}

	if o.Flags.Preemptible {
		args = append(args, "--preemptible")
	}

	if o.Flags.UseStackDriverMonitoring {
		args = append(args, "--enable-stackdriver-kubernetes")
	}

	labels := o.Flags.Labels
	user, err := osUser.Current()
	if err == nil && user != nil {
		labels = AddLabel(labels, "created-by", user.Username)
	}
	timeText := time.Now().Format("Mon-Jan-2-2006-15:04:05")
	labels = AddLabel(labels, "create-time", timeText)
	if labels != "" {
		args = append(args, "--labels="+strings.ToLower(labels))
	}

	if o.Flags.ServiceAccount != "" {
		args = append(args, "--service-account", o.Flags.ServiceAccount)
	}

	log.Logger().Info("Creating cluster...")
	log.Logger().Debugf("gcloud %s", strings.Join(args, " "))

	err = o.RunCommand("gcloud", args...)
	if err != nil {
		return err
	}

	log.Logger().Info("Initialising cluster ...")

	o.InstallOptions.SetInstallValues(map[string]string{
		kube.Zone:        zone,
		kube.Region:      region,
		kube.ProjectID:   projectID,
		kube.ClusterName: o.Flags.ClusterName,
	})

	err = o.initAndInstall(cloud.GKE)
	if err != nil {
		return err
	}

	getCredsCommand := []string{"container", "clusters", "get-credentials", o.Flags.ClusterName}
	if "" != zone {
		getCredsCommand = append(getCredsCommand, "--zone", zone)
	} else if "" != region {
		getCredsCommand = append(getCredsCommand, "--region", region)
	}

	getCredsCommand = append(getCredsCommand, "--project", projectID)

	err = o.RunCommand("gcloud", getCredsCommand...)
	if err != nil {
		return err
	}

	context, err := o.GetCommandOutput("", "kubectl", "config", "current-context")
	if err != nil {
		return err
	}

	_, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	if o.InstallOptions.Flags.Namespace != "" {
		ns = o.InstallOptions.Flags.Namespace
	}

	err = o.RunCommand("kubectl", "config", "set-context", context, "--namespace", ns)
	if err != nil {
		return err
	}

	err = o.RunCommand("kubectl", "get", "ingress")
	if err != nil {
		return err
	}

	return nil
}

// AddLabel adds the given label key and value to the label string
func AddLabel(labels string, name string, value string) string {
	username := util.SanitizeLabel(value)
	if username != "" {
		sep := ""
		if labels != "" {
			sep = ","
		}
		labels += sep + util.SanitizeLabel(name) + "=" + username
	}
	return labels
}

func clusterConfigKey(key string) string {
	return fmt.Sprintf("cluster.%s", key)
}
