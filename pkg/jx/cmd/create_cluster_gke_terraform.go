package cmd

import (
	"io"

	"strings"

	"fmt"

	"errors"

	os_user "os/user"

	"github.com/Pallinder/go-randomdata"
	"github.com/jenkins-x/jx/pkg/jx/cmd/gke"
	"github.com/jenkins-x/jx/pkg/jx/cmd/log"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	cmdutil "github.com/jenkins-x/jx/pkg/jx/cmd/util"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
	"gopkg.in/src-d/go-git.v4"
	"os"
	"path/filepath"
	"time"
)

// CreateClusterOptions the flags for running create cluster
type CreateClusterGKETerraformOptions struct {
	CreateClusterOptions

	Flags CreateClusterGKETerraformFlags
}

type CreateClusterGKETerraformFlags struct {
	AutoUpgrade bool
	ClusterName string
	//ClusterIpv4Cidr string
	//ClusterVersion  string
	DiskSize      string
	MachineType   string
	MinNumOfNodes string
	MaxNumOfNodes string
	ProjectId     string
	SkipLogin     bool
	Zone          string
	Labels        string
}

var (
	createClusterGKETerraformLong = templates.LongDesc(`
		This command creates a new kubernetes cluster on GKE, installing required local dependencies and provisions the
		Jenkins X platform

		You can see a demo of this command here: [http://jenkins-x.io/demos/create_cluster_gke/](http://jenkins-x.io/demos/create_cluster_gke/)

		Google Kubernetes Engine is a managed environment for deploying containerized applications. It brings our latest
		innovations in developer productivity, resource efficiency, automated operations, and open source flexibility to
		accelerate your time to market.

		Google has been running production workloads in containers for over 15 years, and we build the best of what we
		learn into Kubernetes, the industry-leading open source container orchestrator which powers Kubernetes Engine.

`)

	createClusterGKETerraformExample = templates.Examples(`

		jx create cluster gke terraform

`)

	requiredServiceAccountRoles = []string{"roles/compute.instanceAdmin.v1",
		"roles/iam.serviceAccountActor",
		"roles/container.clusterAdmin",
		"roles/container.admin",
		"roles/container.developer"}
)

// NewCmdGet creates a command object for the generic "init" action, which
// installs the dependencies required to run the jenkins-x platform on a kubernetes cluster.
func NewCmdCreateClusterGKETerraform(f cmdutil.Factory, out io.Writer, errOut io.Writer) *cobra.Command {
	options := CreateClusterGKETerraformOptions{
		CreateClusterOptions: createCreateClusterOptions(f, out, errOut, GKE),
	}
	cmd := &cobra.Command{
		Use:     "terraform",
		Short:   "Create a new kubernetes cluster on GKE using Terraform: Runs on Google Cloud",
		Long:    createClusterGKETerraformLong,
		Example: createClusterGKETerraformExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			cmdutil.CheckErr(err)
		},
	}

	options.addCreateClusterFlags(cmd)
	options.addCommonFlags(cmd)

	cmd.Flags().StringVarP(&options.Flags.ClusterName, optionClusterName, "n", "", "The name of this cluster, default is a random generated name")
	//cmd.Flags().StringVarP(&options.Flags.ClusterIpv4Cidr, "cluster-ipv4-cidr", "", "", "The IP address range for the pods in this cluster in CIDR notation (e.g. 10.0.0.0/14)")
	//cmd.Flags().StringVarP(&options.Flags.ClusterVersion, optionKubernetesVersion, "v", "", "The Kubernetes version to use for the master and nodes. Defaults to server-specified")
	cmd.Flags().StringVarP(&options.Flags.DiskSize, "disk-size", "d", "100", "Size in GB for node VM boot disks. Defaults to 100GB")
	cmd.Flags().BoolVarP(&options.Flags.AutoUpgrade, "enable-autoupgrade", "", false, "Sets autoupgrade feature for a cluster's default node-pool(s)")
	cmd.Flags().StringVarP(&options.Flags.MachineType, "machine-type", "m", "", "The type of machine to use for nodes")
	cmd.Flags().StringVarP(&options.Flags.MinNumOfNodes, "min-num-nodes", "", "", "The minimum number of nodes to be created in each of the cluster's zones")
	cmd.Flags().StringVarP(&options.Flags.MaxNumOfNodes, "max-num-nodes", "", "", "The maximum number of nodes to be created in each of the cluster's zones")
	cmd.Flags().StringVarP(&options.Flags.ProjectId, "project-id", "p", "", "Google Project ID to create cluster in")
	cmd.Flags().StringVarP(&options.Flags.Zone, "zone", "z", "", "The compute zone (e.g. us-central1-a) for the cluster")
	cmd.Flags().BoolVarP(&options.Flags.SkipLogin, "skip-login", "", false, "Skip Google auth if already logged in via gloud auth")
	cmd.Flags().StringVarP(&options.Flags.Labels, "labels", "", "", "The labels to add to the cluster being created such as 'foo=bar,whatnot=123'. Label names must begin with a lowercase character ([a-z]), end with a lowercase alphanumeric ([a-z0-9]) with dashes (-), and lowercase alphanumeric ([a-z0-9]) between.")
	return cmd
}

func (o *CreateClusterGKETerraformOptions) Run() error {
	err := o.installRequirements(GKE, "terraform")
	if err != nil {
		return err
	}

	err = o.createClusterGKETerraform()
	if err != nil {
		log.Errorf("error creating cluster %v", err)
		return err
	}

	return nil
}

func (o *CreateClusterGKETerraformOptions) createClusterGKETerraform() error {
	if !o.BatchMode {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Creating a GKE cluster with terraform is an experimental feature in jx.  Would you like to continue?",
		}
		survey.AskOne(prompt, &confirm, nil)

		if !confirm {
			// exit at this point
			return nil
		}
	}

	var err error
	if !o.Flags.SkipLogin {
		err := o.runCommand("gcloud", "auth", "login", "--brief")
		if err != nil {
			return err
		}
	}

	projectId := o.Flags.ProjectId
	if projectId == "" {
		projectId, err = o.getGoogleProjectId()
		if err != nil {
			return err
		}
	}

	err = o.runCommand("gcloud", "config", "set", "project", projectId)
	if err != nil {
		return err
	}

	if o.Flags.ClusterName == "" {
		o.Flags.ClusterName = strings.ToLower(randomdata.SillyName())
		log.Infof("No cluster name provided so using a generated one: %s\n", o.Flags.ClusterName)
	}

	zone := o.Flags.Zone
	if zone == "" {
		availableZones, err := gke.GetGoogleZones()
		if err != nil {
			return err
		}
		prompts := &survey.Select{
			Message:  "Google Cloud Zone:",
			Options:  availableZones,
			PageSize: 10,
			Help:     "The compute zone (e.g. us-central1-a) for the cluster",
		}

		err = survey.AskOne(prompts, &zone, nil)
		if err != nil {
			return err
		}
	}

	machineType := o.Flags.MachineType
	if machineType == "" {
		prompts := &survey.Select{
			Message:  "Google Cloud Machine Type:",
			Options:  gke.GetGoogleMachineTypes(),
			Help:     "We recommend a minimum of n1-standard-2 for Jenkins X,  a table of machine descriptions can be found here https://cloud.google.com/kubernetes-engine/docs/concepts/cluster-architecture",
			PageSize: 10,
			Default:  "n1-standard-2",
		}

		err := survey.AskOne(prompts, &machineType, nil)
		if err != nil {
			return err
		}
	}

	minNumOfNodes := o.Flags.MinNumOfNodes
	if minNumOfNodes == "" {
		prompt := &survey.Input{
			Message: "Minimum number of Nodes",
			Default: "3",
			Help:    "We recommend a minimum of 3 for Jenkins X,  the minimum number of nodes to be created in each of the cluster's zones",
		}

		survey.AskOne(prompt, &minNumOfNodes, nil)
	}

	maxNumOfNodes := o.Flags.MaxNumOfNodes
	if maxNumOfNodes == "" {
		prompt := &survey.Input{
			Message: "Maximum number of Nodes",
			Default: "5",
			Help:    "We recommend at least 5 for Jenkins X,  the maximum number of nodes to be created in each of the cluster's zones",
		}

		survey.AskOne(prompt, &maxNumOfNodes, nil)
	}

	// check to see if a service account exists
	serviceAccount := fmt.Sprintf("jx-%s", o.Flags.ClusterName)
	log.Infof("Checking for service account %s\n", serviceAccount)

	args := []string{"iam",
		"service-accounts",
		"list",
		"--filter",
		serviceAccount}

	output, err := o.getCommandOutput("", "gcloud", args...)
	if err != nil {
		return err
	}

	if output == "Listed 0 items." {
		log.Infof("Unable to find service account %s, checking if we have enough permission to create\n", serviceAccount)

		// if it doesn't check to see if we have permissions to create (assign roles) to a service account
		args = []string{"iam",
			"list-testable-permissions",
			fmt.Sprintf("//cloudresourcemanager.googleapis.com/projects/%s", projectId),
			"--filter",
			"resourcemanager.projects.setIamPolicy"}

		output, err = o.getCommandOutput("", "gcloud", args...)
		if err != nil {
			return err
		}

		if strings.Contains(output, "resourcemanager.projects.setIamPolicy") {
			// create service
			log.Infof("Creating service account %s\n", serviceAccount)
			args = []string{"iam",
				"service-accounts",
				"create",
				serviceAccount}

			err = o.runCommand("gcloud", args...)
			if err != nil {
				return err
			}

			// assign roles to service account
			for _, role := range requiredServiceAccountRoles {
				log.Infof("Assigning role %s\n", role)
				args = []string{"projects",
					"add-iam-policy-binding",
					projectId,
					"--member",
					fmt.Sprintf("serviceAccount:%s@%s.iam.gserviceaccount.com", serviceAccount, projectId),
					"--role",
					role}

				err = o.runCommand("gcloud", args...)
				if err != nil {
					return err
				}
			}

		} else {
			return errors.New("User does not have the required role 'resourcemanager.projects.setIamPolicy' to configure a service account")
		}

	} else {
		log.Info("Service Account exists\n")
	}

	user, err := os_user.Current()
	if err != nil {
		return err
	}

	jxHome := filepath.Join(user.HomeDir, ".jx")
	clustersHome := filepath.Join(jxHome, "clusters")
	clusterHome := filepath.Join(clustersHome, o.Flags.ClusterName)
	os.MkdirAll(clusterHome, os.ModePerm)

	keyPath := filepath.Join(clusterHome, fmt.Sprintf("%s.key.json", serviceAccount))

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		log.Info("Downloading service account key\n")
		args = []string{"iam",
			"service-accounts",
			"keys",
			"create",
			keyPath,
			"--iam-account",
			fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccount, projectId)}

		err = o.runCommand("gcloud", args...)
		if err != nil {
			return err
		}
	}

	terraformDir := filepath.Join(clusterHome, "terraform")
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		os.MkdirAll(terraformDir, os.ModePerm)
		_, err = git.PlainClone(terraformDir, false, &git.CloneOptions{
			URL:           "https://github.com/jenkins-x/terraform-jx-templates-gke",
			ReferenceName: "refs/heads/master",
			SingleBranch:  true,
			Progress:      o.Out,
		})
	}

	username := sanitizeLabel(user.Username)

	// create .tfvars file in .jx folder
	terraformVars := filepath.Join(terraformDir, "terraform.tfvars")
	o.appendFile(terraformVars, fmt.Sprintf("created_by = \"%s\"\n", username))
	o.appendFile(terraformVars, fmt.Sprintf("created_timestamp = \"%s\"\n", time.Now().Format("20060102150405")))
	o.appendFile(terraformVars, fmt.Sprintf("credentials = \"%s\"\n", keyPath))
	o.appendFile(terraformVars, fmt.Sprintf("cluster_name = \"%s\"\n", o.Flags.ClusterName))
	o.appendFile(terraformVars, fmt.Sprintf("gcp_zone = \"%s\"\n", zone))
	o.appendFile(terraformVars, fmt.Sprintf("gcp_project = \"%s\"\n", projectId))
	o.appendFile(terraformVars, fmt.Sprintf("min_node_count = \"%s\"\n", minNumOfNodes))
	o.appendFile(terraformVars, fmt.Sprintf("max_node_count = \"%s\"\n", maxNumOfNodes))
	o.appendFile(terraformVars, fmt.Sprintf("node_machine_type = \"%s\"\n", machineType))
	o.appendFile(terraformVars, "node_preemptible = \"false\"\n")
	o.appendFile(terraformVars, fmt.Sprintf("node_disk_size = \"%s\"\n", o.Flags.DiskSize))
	o.appendFile(terraformVars, "auto_repair = \"false\"\n")
	o.appendFile(terraformVars, fmt.Sprintf("auto_upgrade = \"%t\"\n", o.Flags.AutoUpgrade))
	o.appendFile(terraformVars, "enable_kubernetes_alpha = \"false\"\n")
	o.appendFile(terraformVars, "enable_legacy_abac = \"true\"\n")

	args = []string{"init", terraformDir}
	err = o.runCommand("terraform", args...)
	if err != nil {
		return err
	}

	terraformState := filepath.Join(terraformDir, "terraform.tfstate")

	args = []string{"plan",
		fmt.Sprintf("-state=%s", terraformState),
		fmt.Sprintf("-var-file=%s", terraformVars),
		terraformDir}

	output, err = o.getCommandOutput("", "terraform", args...)
	if err != nil {
		return err
	}

	log.Info("Applying plan...\n")

	args = []string{"apply",
		"-auto-approve",
		fmt.Sprintf("-state=%s", terraformState),
		fmt.Sprintf("-var-file=%s", terraformVars),
		terraformDir}

	err = o.runCommandVerbose("terraform", args...)
	if err != nil {
		return err
	}

	// should we setup the labels at this point?
	//gcloud container clusters update ninjacandy --update-labels ''
	args = []string{"container",
		"clusters",
		"update",
		o.Flags.ClusterName}

	labels := o.Flags.Labels
	if err == nil && user != nil {
		username := sanitizeLabel(user.Username)
		if username != "" {
			sep := ""
			if labels != "" {
				sep = ","
			}
			labels += sep + "created-by=" + username
		}
	}

	sep := ""
	if labels != "" {
		sep = ","
	}
	labels += sep + fmt.Sprintf("created-with=terraform,created-on=%s", time.Now().Format("20060102150405"))
	args = append(args, "--update-labels="+strings.ToLower(labels))

	err = o.runCommand("gcloud", args...)
	if err != nil {
		return err
	}

	output, err = o.getCommandOutput("", "gcloud", "auth", "activate-service-account", "--key-file", keyPath)
	if err != nil {
		return err
	}
	log.Info(output)

	output, err = o.getCommandOutput("", "gcloud", "container", "clusters", "get-credentials", o.Flags.ClusterName, "--zone", zone, "--project", projectId)
	if err != nil {
		return err
	}
	log.Info(output)

	log.Info("Initialising cluster ...\n")
	o.InstallOptions.Flags.DefaultEnvironmentPrefix = o.Flags.ClusterName
	err = o.initAndInstall(GKE)
	if err != nil {
		return err
	}

	context, err := o.getCommandOutput("", "kubectl", "config", "current-context")
	if err != nil {
		return err
	}
	log.Info(context)

	ns := o.InstallOptions.Flags.Namespace
	if ns == "" {
		f := o.Factory
		_, ns, _ = f.CreateClient()
		if err != nil {
			return err
		}
	}

	err = o.runCommand("kubectl", "config", "set-context", context, "--namespace", ns)
	if err != nil {
		return err
	}

	err = o.runCommand("kubectl", "get", "ingress")
	if err != nil {
		return err
	}
	return nil
}

// asks to chose from existing projects or optionally creates one if none exist
func (o *CreateClusterGKETerraformOptions) getGoogleProjectId() (string, error) {
	out, err := o.getCommandOutput("", "gcloud", "projects", "list")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(out), "\n")
	var existingProjects []string
	for _, l := range lines {
		if strings.Contains(l, CLUSTER_LIST_HEADER) {
			continue
		}
		fields := strings.Fields(l)
		existingProjects = append(existingProjects, fields[0])
	}

	var projectId string
	if len(existingProjects) == 0 {
		confirm := &survey.Confirm{
			Message: fmt.Sprintf("No existing Google Projects exist, create one now?"),
			Default: true,
		}
		flag := true
		err = survey.AskOne(confirm, &flag, nil)
		if err != nil {
			return "", err
		}
		if !flag {
			return "", errors.New("no google project to create cluster in, please manual create one and rerun this wizard")
		}

		if flag {
			return "", errors.New("auto creating projects not yet implemented, please manually create one and rerun the wizard")
		}
	} else if len(existingProjects) == 1 {
		projectId = existingProjects[0]
		o.Printf("Using the only Google Cloud Project %s to create the cluster\n", util.ColorInfo(projectId))
	} else {
		prompts := &survey.Select{
			Message: "Google Cloud Project:",
			Options: existingProjects,
			Help:    "Select a Google Project to create the cluster in",
		}

		err := survey.AskOne(prompts, &projectId, nil)
		if err != nil {
			return "", err
		}
	}

	if projectId == "" {
		return "", errors.New("no Google Cloud Project to create cluster in, please manual create one and rerun this wizard")
	}

	return projectId, nil
}

func (o *CreateClusterGKETerraformOptions) appendFile(path string, line string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(line)
	if err != nil {
		return err
	}

	return nil
}
