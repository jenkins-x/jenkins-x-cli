package create

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/packages"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	ibmcloud "github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/api/container/containerv1"
	"github.com/IBM-Cloud/bluemix-go/session"
	randomdata "github.com/Pallinder/go-randomdata"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/iks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/features"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	survey "gopkg.in/AlecAivazis/survey.v1"
)

// CreateClusterOptions the flags for running create cluster
type CreateClusterIKSOptions struct {
	CreateClusterOptions

	Flags CreateClusterIKSFlags
}

type CreateClusterIKSFlags struct {
	Username          string
	Password          string
	Account           string
	SSOCode           bool
	APIKey            string
	Region            string
	ClusterName       string
	KubeVersion       string
	Zone              string
	MachineType       string
	PrivateVLAN       string
	CreatePrivateVLAN bool
	PublicVLAN        string
	CreatePublicVLAN  bool
	PrivateOnly       bool
	Workers           string
	Isolation         string
	NoSubnet          bool
	DiskEncrypt       bool
	Trusted           bool
	SkipLogin         bool
}

const (
	iKSSubDomain        = ".containers.appdomain.cloud"
	dockerRegistryhost  = "docker-registry.jx."
	DEFAULT_IBMREPO_URL = "https://registry.bluemix.net/helm/ibm"
)

var (
	createClusterIKSLong = templates.LongDesc(`
		This command creates a new kubernetes cluster on IKS, installing required local dependencies and provisions the
		Jenkins X platform

		IBM® Cloud Kubernetes Service delivers powerful tools by combining Docker containers, the Kubernetes technology, 
		an intuitive user experience, and built-in security and isolation to automate the deployment, operation, scaling, 
		and monitoring of containerized apps in a cluster of compute hosts.

		Important: In order to create a "standard cluster" required for jenkins-x, you must have a Trial, Pay-As-You-Go, 
				or Subscription IBM Cloud account (https://console.bluemix.net/registration/). "Free cluster"s are currently not
		supported.  
`)

	createClusterIKSExample = templates.Examples(`

		jx create cluster iks

`)
	re = regexp.MustCompile("^([0-9]+)")
)

type byNumberIndex []string

func (s byNumberIndex) Len() int {
	return len(s)
}
func (s byNumberIndex) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byNumberIndex) Less(i, j int) bool {
	var left, right int = 0, 0
	left, _ = strconv.Atoi(re.FindAllString(s[i], -1)[0])
	right, _ = strconv.Atoi(re.FindAllString(s[j], -1)[0])
	return left < right
}

// NewCmdGet creates a command object for the generic "init" action, which
// installs the dependencies required to run the jenkins-x platform on a kubernetes cluster.
func NewCmdCreateClusterIKS(commonOpts *opts.CommonOptions) *cobra.Command {
	options := CreateClusterIKSOptions{
		CreateClusterOptions: createCreateClusterOptions(commonOpts, cloud.OKE),
	}
	cmd := &cobra.Command{
		Use:     "iks",
		Short:   "Create a new kubernetes cluster on IBM Cloud Kubernetes Services",
		Long:    createClusterIKSLong,
		Example: createClusterIKSExample,
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

	cmd.Flags().StringVarP(&options.Flags.Username, "login", "u", "", "Username")
	cmd.Flags().StringVarP(&options.Flags.Password, "password", "p", "", "Password")
	cmd.Flags().StringVarP(&options.Flags.Account, "account", "c", "", "Account")
	cmd.Flags().BoolVarP(&options.Flags.SSOCode, "sso", "", false, "SSO Passcode. See run 'ibmcloud login --sso'")
	cmd.Flags().StringVarP(&options.Flags.APIKey, "apikey", "", "", "The IBM Cloud API Key.")
	cmd.Flags().StringVarP(&options.Flags.Region, "region", "r", "", "The IBM Cloud Region. Default is 'us-east'")
	cmd.Flags().StringVarP(&options.Flags.ClusterName, "name", "n", "", "Set the name of the cluster that will be created.")
	cmd.Flags().StringVarP(&options.Flags.KubeVersion, "kube-version", "k", "", "Specify the Kubernetes version, including at least the major.minor version. If you do not include this flag, the default version is used. To see available versions, run ‘ibmcloud ks kube-versions’.")
	cmd.Flags().StringVarP(&options.Flags.Zone, "zone", "z", "", "Specify the zone where you want to create the cluster, the options depend on what region that you are logged in to. To see available zones, run 'ibmcloud ks zones'. Default is 'wdc07'")
	cmd.Flags().StringVarP(&options.Flags.MachineType, "machine-type", "m", "", "The machine type of the worker node. To see available machine types, run 'ibmcloud ks machine-types --zone <zone name>'. Default is 'b2c.4x16', 4 cores CPU, 16GB Memory")
	cmd.Flags().StringVarP(&options.Flags.PrivateVLAN, "private-vlan", "", "", "Conditional: Specify the ID of the private VLAN. To see available VLANs, run 'ibmcloud ks vlans --zone <zone name>'. If you do not have a private VLAN yet, do not specify this option because one will be automatically created for you. When you specify a private VLAN, you must also specify either the ‘--public-vlan’ flag or the ‘--private-only’ flag.")
	cmd.Flags().BoolVarP(&options.Flags.CreatePrivateVLAN, "create-private-vlan", "", false, "Automatically create private vlan (default 'true')")
	cmd.Flags().StringVarP(&options.Flags.PublicVLAN, "public-vlan", "", "", "Conditional: Specify the ID of the public VLAN. To see available VLANs, run 'ibmcloud ks vlans --zone <zone name>'. If you do not have a public VLAN yet, do not specify this option because one will be automatically created for you.")
	cmd.Flags().BoolVarP(&options.Flags.CreatePublicVLAN, "create-public-vlan", "", false, "Automatically create public vlan (default 'true')")
	cmd.Flags().BoolVarP(&options.Flags.PrivateOnly, "private-only", "", false, "Use this flag to prevent a public VLAN from being created. Required only when you specify the ‘--private-vlan’ flag without specifying the ‘--public-vlan’ flag.")
	cmd.Flags().StringVarP(&options.Flags.Workers, "workers", "", "", "The number of cluster worker nodes. Defaults to 3.")
	cmd.Flags().StringVarP(&options.Flags.Isolation, "isolation", "", "public", "The level of hardware isolation for your worker node. Use 'private' to have available physical resources dedicated to you only, or 'public' to allow physical resources to be shared with other IBM customers. For IBM Cloud Public accounts, the default value is public.")
	cmd.Flags().BoolVarP(&options.Flags.NoSubnet, "no-subnet", "", false, "Optional: Prevent the creation of a portable subnet when creating the cluster. By default, both a public and a private portable subnet are created on the associated VLAN, and this flag prevents that behavior. To add a subnet to the cluster later, run 'ibmcloud ks cluster-subnet-add'.")
	cmd.Flags().BoolVarP(&options.Flags.DiskEncrypt, "disk-encrypt", "", true, "Optional: Disable encryption on a worker node.")
	cmd.Flags().BoolVarP(&options.Flags.Trusted, "trusted", "", false, "Optional: Enable trusted cluster feature.")
	cmd.Flags().BoolVarP(&options.Flags.SkipLogin, "skip-login", "", false, "Skip login if already logged in using `ibmcloud login`")
	return cmd
}

func (o *CreateClusterIKSOptions) Run() error {

	var deps []string
	d := packages.BinaryShouldBeInstalled("ibmcloud")
	if d != "" {
		deps = append(deps, d)
	}
	err := o.InstallMissingDependencies(deps)
	if err != nil {
		log.Logger().Errorf("%v\nPlease fix the error or install manually then try again", err)
		os.Exit(-1)
	}

	err = o.createClusterIKS()
	if err != nil {
		log.Logger().Errorf("error creating cluster %v", err)
		os.Exit(-1)
	}

	return nil
}

func (o *CreateClusterIKSOptions) createClusterIKS() error {

	var err error

	var c *ibmcloud.Config
	var s *session.Session

	var regions iks.Regions
	var region *iks.Region

	var zones iks.Zones
	var zone *iks.Zone
	var machineTypes iks.MachineTypes
	var machineType *iks.MachineType
	var vLANs iks.VLANs
	var clusters containerv1.Clusters
	var config iks.Clusters
	var kubeVersion, privateVLAN, publicVLAN, accountGUID string

	userName := o.Flags.Username
	password := o.Flags.Password
	aPIKey := o.Flags.APIKey
	sSO := o.Flags.SSOCode
	accountGUID = o.Flags.Account

	c = new(ibmcloud.Config)
	s, err = session.New(c)
	if err != nil {
		return err
	}

	if o.Flags.Region == "" {
		regionsarr, err := iks.GetAuthRegions(c)
		if err != nil {
			return err
		}
		prompt := &survey.Select{
			Message:  "Region",
			Options:  regionsarr,
			Default:  "us-east",
			PageSize: 10,
			Help:     "IBM Cloud Region to authenticate with and create the cluster in:",
		}
		var regionstr string
		err = survey.AskOne(prompt, &regionstr, nil)
		c.Region = regionstr
		if err != nil {
			return err
		}
	} else {
		c.Region = o.Flags.Region
	}

	if !o.Flags.SkipLogin {
		// [-a API_ENDPOINT] [--sso] [-u USERNAME] [-p PASSWORD] [--apikey KEY | @KEY_FILE] [--no-iam] [-c ACCOUNT_ID | --no-account] [-g RESOURCE_GROUP] [-o ORG] [-s SPACE]
		var ibmLogin []string
		if c.Region == "us-south" {
			ibmLogin = []string{"login", "-a", "api.ng.bluemix.net"}
		} else {
			ibmLogin = []string{"login", "-a", "api." + c.Region + ".bluemix.net"}
		}
		if aPIKey != "" {
			ibmLogin = append(ibmLogin, "--apikey", aPIKey)
		} else if sSO {
			ibmLogin = append(ibmLogin, "--sso")
		} else {
			ibmLogin = append(ibmLogin, "-u", userName, "-p", password)
		}
		if o.Flags.Account != "" {
			ibmLogin = append(ibmLogin, "-c", o.Flags.Account)
		}
		err = o.RunCommandInteractive(true, "ibmcloud", ibmLogin...)
	}
	accountGUID, err = iks.ConfigFromJSON(c)
	if err != nil {
		return err
	}
	c.Endpoint = nil
	sessionlessAPI, err := iks.NewSessionless(s)
	regions = sessionlessAPI.Regions()
	region, err = regions.GetRegion(c.Region)
	if err != nil {
		return err
	}
	// Authenticated at this point, grab a session

	sessionedAPI, err := iks.New(s)
	if err != nil {
		return err
	}
	clusterAPI, err := containerv1.New(s)
	if err != nil {
		return err
	}
	zones = sessionlessAPI.Zones()
	machineTypes = sessionedAPI.MachineTypes()
	vLANs = sessionedAPI.VLANs()
	clusters = clusterAPI.Clusters()
	config = sessionedAPI.Clusters()

	target := containerv1.ClusterTargetHeader{
		Region:    c.Region,
		AccountID: accountGUID,
	}

	var validClusterName = false
	clusterName := o.Flags.ClusterName
	// +1 for the dot
	clusterMaxLength := 64 - (len(iKSSubDomain) + len(dockerRegistryhost) + len(c.Region) + 1)
	if clusterName != "" {
		if len(clusterName) > clusterMaxLength {
			log.Logger().Infof("Cluster name too can only be %d bytes for %s", clusterMaxLength, c.Region)
		} else {
			validClusterName = true
		}
	} else {
		clusterName = strings.ToLower(randomdata.SillyName())
		if len(clusterName) > clusterMaxLength {
			clusterName = clusterName[:clusterMaxLength]
		}
	}
	for !validClusterName {
		for !validClusterName {
			cluster, err := clusters.Find(clusterName, target)
			if err != nil || cluster.ID == "" {
				validClusterName = true
			} else {
				clusterName = strings.ToLower(randomdata.SillyName())
				if len(clusterName) > clusterMaxLength {
					clusterName = clusterName[:clusterMaxLength]
				}
			}
		}

		prompt := &survey.Input{
			Message: "Cluster Name:",
			Default: clusterName,
		}
		validator := survey.ComposeValidators(survey.Required, survey.MaxLength(clusterMaxLength))
		err := survey.AskOne(prompt, &clusterName, validator)
		if err != nil {
			return err
		}
	}

	if o.Flags.Zone == "" {
		zonesarr, err := iks.GetZones(*region, zones)
		if err != nil {
			return err
		}
		prompts := &survey.Select{
			Message:  "Zone (location):",
			Options:  zonesarr,
			Help:     "A single zone cluster will be create by default in the washington (us-east) data center 7",
			PageSize: 10,
			Default:  "wdc07",
		}
		var zonestr string
		err = survey.AskOne(prompts, &zonestr, nil)
		if err != nil {
			return err
		}
		zone, err = zones.GetZone(zonestr, *region)
		if err != nil {
			return err
		}
	} else {
		zone, err = zones.GetZone(o.Flags.Zone, *region)
		if err != nil {
			return err
		}
	}

	clusterClient, err := containerv1.New(s)
	if err != nil {
		return err
	}

	if o.Flags.KubeVersion == "" {
		versionarr, defversion, err := iks.GetKubeVersions(clusterClient.KubeVersions())
		prompts := &survey.Select{
			Message:  "Kubernetes Version:",
			Options:  versionarr,
			Help:     "Version of Kubernetes cluster to create",
			PageSize: 10,
			Default:  defversion,
		}
		err = survey.AskOne(prompts, &kubeVersion, nil)

		if err != nil {
			return err
		}
	} else {
		kubeVersion = o.Flags.KubeVersion
	}

	if o.Flags.MachineType == "" {
		machinetypearr, err := iks.GetMachineTypes(*zone, *region, machineTypes)
		prompts := &survey.Select{
			Message:  "Kubernetes Node Machine Type:",
			Options:  machinetypearr,
			Help:     "Machine type to use for kubernetes nodes;",
			PageSize: 10,
			Default:  "b2c.4x16",
		}
		var machineTypeStr string
		err = survey.AskOne(prompts, &machineTypeStr, nil)
		if err != nil {
			return err
		}
		machineType, err = machineTypes.GetMachineType(machineTypeStr, *zone, *region)
		if err != nil {
			return err
		}
	} else {
		machineType, err = machineTypes.GetMachineType(o.Flags.MachineType, *zone, *region)
		if err != nil {
			return err
		}
	}

	var workers *int
	if o.Flags.Workers != "" {
		i, err := strconv.Atoi(o.Flags.Workers)
		if err != nil {
			o.Flags.Workers = ""
		} else {
			workers = &i
		}
	}
	if workers == nil {
		prompt := &survey.Input{
			Message: "Number of kubernetes workers:",
			Default: "3",
		}
		workers = new(int)
		err := survey.AskOne(prompt, workers, survey.Required)
		if err != nil {
			return err
		}
	}
	privatearr, err := iks.GetPrivateVLANs(*zone, *region, vLANs)
	if privatearr != nil && len(privatearr) > 0 && err == nil && !o.Flags.CreatePrivateVLAN && o.Flags.PrivateVLAN == "" {
		if len(privatearr) > 1 {
			prompts := &survey.Select{
				Message:  "Private VLAN for workers:",
				Options:  privatearr,
				Help:     "Private VLAN to use for kubernetes nodes.",
				PageSize: 10,
				Default:  "",
			}
			err = survey.AskOne(prompts, &privateVLAN, nil)
			if err != nil {
				return err
			}
		} else {
			privateVLAN = privatearr[0]
		}
	} else {
		privateVLAN = o.Flags.PrivateVLAN
	}
	if privateVLAN == "" {
		log.Logger().Info("Creating Private VLAN.")
	} else {
		log.Logger().Infof("Chosen Private VLAN is %s", util.ColorInfo(privateVLAN))
	}

	publicarr, err := iks.GetPublicVLANs(*zone, *region, vLANs)
	if publicarr != nil && len(publicarr) > 0 && err == nil && !o.Flags.CreatePublicVLAN && o.Flags.PublicVLAN == "" && !o.Flags.PrivateOnly {
		if len(publicarr) > 1 {
			prompts := &survey.Select{
				Message:  "Public VLAN for workers:",
				Options:  publicarr,
				Help:     "Public VLAN to use for kubernetes nodes.",
				PageSize: 10,
				Default:  "",
			}
			err = survey.AskOne(prompts, &publicVLAN, nil)
			if err != nil {
				return err
			}
		} else {
			publicVLAN = publicarr[0]
		}
	} else {
		publicVLAN = o.Flags.PublicVLAN
	}
	if publicVLAN == "" {
		if o.Flags.PrivateOnly {
			log.Logger().Info("Private only -> No Public VLAN")
		} else {
			log.Logger().Info("Creating Public VLAN.")
		}
	} else {
		log.Logger().Infof("Chosen Public VLAN is %s", util.ColorInfo(publicVLAN))
	}

	var clusterInfo = containerv1.ClusterCreateRequest{
		Name:           clusterName,
		Datacenter:     zone.ID,
		MachineType:    machineType.Name,
		WorkerNum:      *workers,
		PrivateVlan:    privateVLAN,
		PublicVlan:     publicVLAN,
		Isolation:      o.Flags.Isolation,
		NoSubnet:       o.Flags.NoSubnet,
		MasterVersion:  kubeVersion,
		DiskEncryption: o.Flags.DiskEncrypt,
		EnableTrusted:  o.Flags.Trusted,
	}

	log.Logger().Infof("Creating cluster named %s", clusterName)
	//	log.Logger().Infof(clusterInfo)
	createResponse, err := clusters.Create(clusterInfo, target)
	if err != nil {
		return err
	}
	cluster, err := clusters.Find(createResponse.ID, target)
	if err != nil {
		return err
	}

	/*	cluster, err := clusters.Find("sagenickel", target)
		if err != nil {
			return err
	}*/

	log.Logger().Infof("Waiting for cluster named %s, can take around 15 minutes (time out in 1h) ...", cluster.Name)

	timeout := time.After(time.Hour)
	tick := time.Tick(time.Second)
	start := time.Now()
L:
	for {
		select {
		case <-timeout:
			return errors.New("timed out")
		case <-tick:
			cluster, err = clusters.Find(createResponse.ID, target)
			//cluster, err = clusters.Find("sagenickel", target)
			if err != nil {
				return err
			}
			if cluster.State != "normal" {
				duration := time.Since(start)
				seconds := int64(math.Mod(duration.Seconds(), 60))
				secondsString := strconv.FormatInt(seconds, 10)
				if len(secondsString) < 2 {
					secondsString = "0" + secondsString
				}
				log.Logger().Infof("\rTime elapsed: %dm%ss", int(duration.Minutes()), secondsString)
			} else {
				break L
			}
		}
	}

	//setup the kube context

	log.Logger().Infof("\nCreated cluster named %s", cluster.Name)

	kubeconfig, err := config.GetClusterConfig(cluster.Name, target)
	if err != nil {
		return err
	}

	log.Logger().Info("Setting kube config file")
	log.Logger().Infof("export KUBECONFIG=\"%s\"", kubeconfig)
	os.Setenv("KUBECONFIG", kubeconfig)
	log.Logger().Info("Initialising cluster ...")

	return o.initAndInstall(cloud.IKS)
}
