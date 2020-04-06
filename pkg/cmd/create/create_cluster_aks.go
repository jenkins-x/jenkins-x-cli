package create

import (
	"os"
	"strings"

	randomdata "github.com/Pallinder/go-randomdata"
	"github.com/jenkins-x/jx/pkg/packages"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/aks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/features"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// CreateClusterOptions the flags for running create cluster
type CreateClusterAKSOptions struct {
	CreateClusterOptions

	Flags CreateClusterAKSFlags
}

type CreateClusterAKSFlags struct {
	UserName                  string
	Password                  string
	ClusterName               string
	ResourceName              string
	Location                  string
	NodeVMSize                string
	NodeOSDiskSize            string
	NodeCount                 string
	KubeVersion               string
	PathToPublicKey           string
	ClientSecret              string
	ServicePrincipal          string
	Subscription              string
	AADClientAppID            string
	AADServerAppID            string
	AADServerAppSecret        string
	AADTenantID               string
	AdminUsername             string
	DNSNamePrefix             string
	DNSServiceIP              string
	DockerBridgeAddress       string
	PodCIDR                   string
	ServiceCIDR               string
	VnetSubnetID              string
	WorkspaceResourceID       string
	SkipLogin                 bool
	SkipProviderRegistration  bool
	SkipResourceGroupCreation bool
	Tags                      string
}

var (
	createClusterAKSLong = templates.LongDesc(`
		This command creates a new Kubernetes cluster on AKS, installing required local dependencies and provisions the
		Jenkins X platform

		Azure Container Service (AKS) manages your hosted Kubernetes environment, making it quick and easy to deploy and
		manage containerized applications without container orchestration expertise. It also eliminates the burden of
		ongoing operations and maintenance by provisioning, upgrading, and scaling resources on demand, without taking
		your applications offline.

		Please use a location local to you: you can retrieve this from the Azure portal or by 
		running "az provider list" in your terminal.

		Important: You will need an account on Azure, with a storage account (https://portal.azure.com/#create/Microsoft.StorageAccount-ARM)
        and network (https://portal.azure.com/#create/Microsoft.VirtualNetwork-ARM) - both linked to the resource group you use
		to create the cluster in.
`)

	createClusterAKSExample = templates.Examples(`

		jx create cluster aks

`)
)

// NewCmdGet creates a command object for the generic "init" action, which
// installs the dependencies required to run the jenkins-x platform on a Kubernetes cluster.
func NewCmdCreateClusterAKS(commonOpts *opts.CommonOptions) *cobra.Command {
	options := CreateClusterAKSOptions{
		CreateClusterOptions: createCreateClusterOptions(commonOpts, cloud.AKS),
	}
	cmd := &cobra.Command{
		Use:     "aks",
		Short:   "Create a new Kubernetes cluster on AKS: Runs on Azure",
		Long:    createClusterAKSLong,
		Example: createClusterAKSExample,
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

	cmd.Flags().StringVarP(&options.Flags.UserName, "user-name", "u", "", "Azure user name")
	cmd.Flags().StringVarP(&options.Flags.Password, "password", "p", "", "Azure password")
	cmd.Flags().StringVarP(&options.Flags.ResourceName, "resource-group-name", "n", "", "Name of the resource group")
	cmd.Flags().StringVarP(&options.Flags.ClusterName, "cluster-name", "c", "", "Name of the cluster")
	cmd.Flags().StringVarP(&options.Flags.Location, "location", "l", "", "Location to run cluster in")
	cmd.Flags().StringVarP(&options.Flags.NodeVMSize, "node-vm-size", "s", "", "Size of Virtual Machines to create as Kubernetes nodes")
	cmd.Flags().StringVarP(&options.Flags.NodeOSDiskSize, "disk-size", "", "", "Size in GB of the OS disk for each node in the node pool.")
	cmd.Flags().StringVarP(&options.Flags.NodeCount, "nodes", "o", "", "Number of node")
	cmd.Flags().StringVarP(&options.Flags.KubeVersion, optionKubernetesVersion, "v", "", "Version of Kubernetes to use for creating the cluster, such as '1.8.11' or '1.9.6'.  Values from: `az aks get-versions`.")
	cmd.Flags().StringVarP(&options.Flags.PathToPublicKey, "path-To-public-rsa-key", "k", "", "Path to public RSA key")
	cmd.Flags().StringVarP(&options.Flags.ClientSecret, "client-secret", "", "", "Azure AD client secret to use an existing SP")
	cmd.Flags().StringVarP(&options.Flags.ServicePrincipal, "service-principal", "", "", "Azure AD service principal to use an existing SP")
	cmd.Flags().StringVarP(&options.Flags.Subscription, "subscription", "", "", "Azure subscription to be used if not default one")
	cmd.Flags().StringVarP(&options.Flags.AADClientAppID, "aad-client-app-id", "", "", "The ID of an Azure Active Directory client application")
	cmd.Flags().StringVarP(&options.Flags.AADServerAppID, "aad-server-app-id", "", "", "The ID of an Azure Active Directory server application")
	cmd.Flags().StringVarP(&options.Flags.AADServerAppSecret, "aad-server-app-secret", "", "", "The secret of an Azure Active Directory server application")
	cmd.Flags().StringVarP(&options.Flags.AADTenantID, "aad-tenant-id", "", "", "The ID of an Azure Active Directory tenant")
	cmd.Flags().StringVarP(&options.Flags.AdminUsername, "admin-username", "", "", "User account to create on node VMs for SSH access")
	cmd.Flags().StringVarP(&options.Flags.DNSNamePrefix, "dns-name-prefix", "", "", "Prefix for hostnames that are created")
	cmd.Flags().StringVarP(&options.Flags.DNSServiceIP, "dns-service-ip", "", "", "IP address assigned to the Kubernetes DNS service")
	cmd.Flags().StringVarP(&options.Flags.DockerBridgeAddress, "docker-bridge-address", "", "", "An IP address and netmask assigned to the Docker bridge")
	cmd.Flags().StringVarP(&options.Flags.PodCIDR, "pod-cidr", "", "", "A CIDR notation IP range from which to assign pod IPs")
	cmd.Flags().StringVarP(&options.Flags.ServiceCIDR, "service-cidr", "", "", "A CIDR notation IP range from which to assign service cluster IPs")
	cmd.Flags().StringVarP(&options.Flags.VnetSubnetID, "vnet-subnet-id", "", "", "The ID of a subnet in an existing VNet into which to deploy the cluster")
	cmd.Flags().StringVarP(&options.Flags.WorkspaceResourceID, "workspace-resource-id", "", "", "The resource ID of an existing Log Analytics Workspace")
	cmd.Flags().BoolVarP(&options.Flags.SkipLogin, "skip-login", "", false, "Skip login if already logged in using `az login`")
	cmd.Flags().BoolVarP(&options.Flags.SkipProviderRegistration, "skip-provider-registration", "", false, "Skip provider registration")
	cmd.Flags().BoolVarP(&options.Flags.SkipResourceGroupCreation, "skip-resource-group-creation", "", false, "Skip resource group creation")
	cmd.Flags().StringVarP(&options.Flags.Tags, "tags", "", "", "Space-separated tags in 'key[=value]' format. Use '' to clear existing tags.")
	return cmd
}

func (o *CreateClusterAKSOptions) Run() error {

	var deps []string
	d := packages.BinaryShouldBeInstalled("az")
	if d != "" {
		deps = append(deps, d)
	}
	err := o.InstallMissingDependencies(deps)
	if err != nil {
		log.Logger().Errorf("%v\nPlease fix the error or install manually then try again", err)
		os.Exit(-1)
	}

	err = o.createClusterAKS()
	if err != nil {
		log.Logger().Errorf("error creating cluster %v", err)
		os.Exit(-1)
	}

	return nil
}

func (o *CreateClusterAKSOptions) createClusterAKS() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)

	resourceName := o.Flags.ResourceName
	if resourceName == "" {
		resourceName = strings.ToLower(randomdata.SillyName())
		log.Logger().Infof("No resource name provided so using a generated one: %s", resourceName)
	}

	clusterName := o.Flags.ClusterName
	if clusterName == "" {
		clusterName = strings.ToLower(randomdata.SillyName())
		log.Logger().Infof("No cluster name provided so using a generated one: %s", clusterName)
	}

	location := o.Flags.Location
	if location == "" {
		prompt := &survey.Select{
			Message:  "Location",
			Options:  aks.GetResourceGroupLocation(),
			Default:  "eastus",
			PageSize: 10,
			Help:     "location to run cluster",
		}
		err := survey.AskOne(prompt, &location, nil, surveyOpts)
		if err != nil {
			return err
		}
	}

	nodeVMSize := o.Flags.NodeVMSize
	if nodeVMSize == "" {
		prompts := &survey.Select{
			Message:  "Virtual Machine Size:",
			Options:  aks.GetSizes(),
			Help:     "We recommend a minimum of Standard_D2s_v3 for Jenkins X.\nA table of machine descriptions can be found here https://azure.microsoft.com/en-us/pricing/details/virtual-machines/linux/",
			PageSize: 10,
			Default:  "Standard_D2s_v3",
		}

		err := survey.AskOne(prompts, &nodeVMSize, nil, surveyOpts)
		if err != nil {
			return err
		}
	}

	nodeCount := o.Flags.NodeCount
	if nodeCount == "" {
		prompt := &survey.Input{
			Message: "Number of Nodes",
			Default: "3",
			Help:    "We recommend a minimum of 3 nodes for Jenkins X",
		}
		survey.AskOne(prompt, &nodeCount, nil, surveyOpts)
	}

	pathToPublicKey := o.Flags.PathToPublicKey

	userName := o.Flags.UserName
	password := o.Flags.Password

	var err error
	if !o.Flags.SkipLogin {
		//First login

		if userName != "" && password != "" {
			log.Logger().Info("Logging in to Azure using provider username and password...")
			err = o.RunCommand("az", "login", "-u", userName, "-p", password)
			if err != nil {
				return err
			}
		} else {
			log.Logger().Info("Logging in to Azure interactively...")
			err = o.RunCommandVerbose("az", "login")
			if err != nil {
				return err
			}
		}
	}

	if !o.Flags.SkipProviderRegistration {
		//register for Microsoft Compute and Containers

		err = o.RunCommand("az", "provider", "register", "-n", "Microsoft.Compute")
		if err != nil {
			return err
		}

		err = o.RunCommand("az", "provider", "register", "-n", "Microsoft.ContainerService")
		if err != nil {
			return err
		}
	}

	if !o.Flags.SkipResourceGroupCreation {
		//create a resource group

		createGroup := []string{"group", "create", "-l", location, "-n", resourceName}

		err = o.RunCommand("az", createGroup...)

		if err != nil {
			return err
		}
	}

	subscription := o.Flags.Subscription

	if subscription != "" {
		log.Logger().Info("Changing subscription...")
		err = o.RunCommandVerbose("az", "account", "set", "--subscription", subscription)

		if err != nil {
			return err
		}
	}
	createCluster := []string{"aks", "create", "-g", resourceName, "-n", clusterName}

	if o.Flags.KubeVersion != "" {
		createCluster = append(createCluster, "--kubernetes-version", o.Flags.KubeVersion)
	}

	if nodeVMSize != "" {
		createCluster = append(createCluster, "--node-vm-size", nodeVMSize)
	}

	if o.Flags.NodeOSDiskSize != "" {
		createCluster = append(createCluster, "--node-osdisk-size", o.Flags.NodeOSDiskSize)
	}

	if nodeCount != "" {
		createCluster = append(createCluster, "--node-count", nodeCount)
	}

	if pathToPublicKey != "" {
		createCluster = append(createCluster, "--ssh-key-value", pathToPublicKey)
	} else {
		createCluster = append(createCluster, "--generate-ssh-keys")
	}

	if o.Flags.ClientSecret != "" {
		createCluster = append(createCluster, "--client-secret", o.Flags.ClientSecret)
	}

	if o.Flags.ServicePrincipal != "" {
		createCluster = append(createCluster, "--service-principal", o.Flags.ServicePrincipal)
	}

	if o.Flags.AADClientAppID != "" {
		createCluster = append(createCluster, "--aad-client-app-id", o.Flags.AADClientAppID)
	}

	if o.Flags.AADServerAppID != "" {
		createCluster = append(createCluster, "--aad-server-app-id", o.Flags.AADServerAppID)
	}

	if o.Flags.AADServerAppSecret != "" {
		createCluster = append(createCluster, "--aad-server-app-secret", o.Flags.AADServerAppSecret)
	}

	if o.Flags.AADTenantID != "" {
		createCluster = append(createCluster, "--aad-tenant-id", o.Flags.AADTenantID)
	}

	if o.Flags.AdminUsername != "" {
		createCluster = append(createCluster, "--admin-username", o.Flags.AdminUsername)
	}

	if o.Flags.DNSNamePrefix != "" {
		createCluster = append(createCluster, "--dns-name-prefix", o.Flags.DNSNamePrefix)
	}

	if o.Flags.DNSServiceIP != "" {
		createCluster = append(createCluster, "--dns-service-ip", o.Flags.DNSServiceIP)
	}

	if o.Flags.DockerBridgeAddress != "" {
		createCluster = append(createCluster, "--docker-bridge-address", o.Flags.DockerBridgeAddress)
	}

	if o.Flags.PodCIDR != "" {
		createCluster = append(createCluster, "--pod-cidr", o.Flags.PodCIDR)
	}

	if o.Flags.ServiceCIDR != "" {
		createCluster = append(createCluster, "--service-cidr", o.Flags.ServiceCIDR)
	}

	if o.Flags.VnetSubnetID != "" {
		createCluster = append(createCluster, "--vnet-subnet-id", o.Flags.VnetSubnetID)
	}

	if o.Flags.WorkspaceResourceID != "" {
		createCluster = append(createCluster, "--workspace-resource-id", o.Flags.WorkspaceResourceID)
	}

	if o.Flags.Tags != "" {
		createCluster = append(createCluster, "--tags", o.Flags.Tags)
	}

	log.Logger().Infof("Creating cluster named %s in resource group %s...", clusterName, resourceName)
	err = o.RunCommand("az", createCluster...)
	if err != nil {
		return err
	}

	//setup the kube context

	getCredentials := []string{"aks", "get-credentials", "--resource-group", resourceName, "--name", clusterName}

	err = o.RunCommand("az", getCredentials...)
	if err != nil {
		return err
	}

	log.Logger().Info("Initialising cluster ...")
	return o.initAndInstall(cloud.AKS)
}
