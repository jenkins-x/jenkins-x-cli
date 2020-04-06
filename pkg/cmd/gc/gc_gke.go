package gc

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/spf13/cobra"

	"encoding/json"

	"fmt"

	"io/ioutil"

	"os"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// GCGKEOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type GCGKEOptions struct {
	*opts.CommonOptions
	Flags                GCGKEFlags
	RevisionHistoryLimit int
}

// GCGKEFlags contains the flags for the command
type GCGKEFlags struct {
	ProjectID string
	RunNow    bool
}

var (
	GCGKELong = templates.LongDesc(`
		Garbage collect Google Container Engine resources that are not deleted when a delete cluster is performed

		This command will generate the gcloud command to run and delete external loadbalancers and persistent disks
		that are no longer in use.

`)

	GCGKEExample = templates.Examples(`
		jx garbage collect gke
		jx gc gke
`)

	ServiceAccountSuffixes = []string{"-vt", "-ko", "-tf", "-dn", "-ex", "-jb", "-st", "-tk", "-vo"}
)

type Rules struct {
	Rules []Rule
}

type Rule struct {
	Name       string   `json:"name"`
	TargetTags []string `json:"targetTags"`
}

type cluster struct {
	Name string `json:"name"`
}

type address struct {
	Name   string `json:"name"`
	Region string `json:"region"`
}

type zone struct {
	Name string `json:"name"`
}

type disk struct {
	Name string `json:"name"`
}

type serviceAccount struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type iamPolicy struct {
	Bindings []iamBinding `json:"bindings"`
}

type iamBinding struct {
	Members []string `json:"members"`
	Role    string   `json:"role"`
}

func (o *GCGKEOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Flags.ProjectID, "project", "p", "", "The google project id to create the GC script for")
	cmd.Flags().BoolVarP(&o.Flags.RunNow, "run-now", "", false, "Execute the script")
}

// NewCmdGCGKE is a command object for the "step" command
func NewCmdGCGKE(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GCGKEOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "gke",
		Short:   "garbage collection for gke",
		Long:    GCGKELong,
		Example: GCGKEExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.CommonOptions.AddBaseFlags(cmd)
	options.addFlags(cmd)

	return cmd
}

// Run implements this command
func (o *GCGKEOptions) Run() error {

	if o.Flags.ProjectID == "" {
		if o.BatchMode {
			projectID, err := o.getCurrentGoogleProjectId()
			if err != nil {
				return err
			}
			o.Flags.ProjectID = projectID
		} else {
			projectID, err := o.GetGoogleProjectID("")
			if err != nil {
				return err
			}
			o.Flags.ProjectID = projectID
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	gkeSa := os.Getenv("GKE_SA_KEY_FILE")
	if gkeSa != "" {
		err = o.GCloud().Login(gkeSa, true)
		if err != nil {
			return err
		}
	}

	path := util.UrlJoin(dir, "gc_gke.sh")
	util.FileExists(path)

	os.Remove(path)

	message := `#!/bin/bash

set -euo pipefail

###################################################################################################
#
#  WARNING: this command is experimental and the generated script should be executed at the users own risk.  We use this
#  generated command on the Jenkins X project itself but it has not been tested on other clusters.
#
###################################################################################################

# Project %s

%s

%s

%s

%s

`
	log.Logger().Warn("This command is experimental and the generated script should be executed at the users own risk\n")
	log.Logger().Warn("We will generate a script for you to review and execute, this command will not delete any resources by itself\n")
	log.Logger().Info("It may take a few minutes to create the script\n")

	fw, err := o.cleanUpFirewalls()
	if err != nil {
		return err
	}

	disks, err := o.cleanUpPersistentDisks()
	if err != nil {
		return err
	}

	addr, err := o.cleanUpAddresses()
	if err != nil {
		return err
	}

	serviceAccounts, err := o.cleanUpServiceAccounts()
	if err != nil {
		return err
	}

	data := fmt.Sprintf(message, o.Flags.ProjectID, fw, strings.Join(disks, "\n"), strings.Join(addr, "\n"), strings.Join(serviceAccounts, "\n"))
	data = strings.Replace(data, "[", "", -1)
	data = strings.Replace(data, "]", "", -1)

	err = ioutil.WriteFile("gc_gke.sh", []byte(data), util.DefaultWritePermissions)
	if err != nil {
		return err
	}
	log.Logger().Info("Script 'gc_gke.sh' created!")
	if o.Flags.RunNow {
		log.Logger().Info("Executing 'gc_gke.sh'")
		err = o.RunCommand("gc_gke.sh")
		log.Logger().Info("Done")
	}
	return err
}

func (o *GCGKEOptions) cleanUpFirewalls() (string, error) {
	co := &opts.CommonOptions{}
	data, err := co.GetCommandOutput("", "gcloud", "compute", "firewall-rules", "list", "--format", "json", "--project", o.Flags.ProjectID)
	if err != nil {
		return "", err
	}

	var rules []Rule
	err = json.Unmarshal([]byte(data), &rules)
	if err != nil {
		return "", err
	}

	out, err := co.GetCommandOutput("", "gcloud", "container", "clusters", "list", "--project", o.Flags.ProjectID)
	if err != nil {
		return "", err
	}

	lines := strings.Split(out, "\n")
	var existingClusters []string
	for _, l := range lines {
		if strings.Contains(l, "NAME") {
			continue
		}
		if strings.TrimSpace(l) == "" {
			break
		}
		fields := strings.Fields(l)
		existingClusters = append(existingClusters, fields[0])
	}

	var nameToDelete []string
	for _, rule := range rules {
		name := strings.TrimPrefix(rule.Name, "gke-")

		if !contains(existingClusters, name) {
			for _, tagFull := range rule.TargetTags {
				tag := strings.TrimPrefix(tagFull, "gke-")

				if !contains(existingClusters, tag) {
					nameToDelete = append(nameToDelete, rule.Name)
				}
			}
		}
	}

	if nameToDelete != nil {
		args := "gcloud compute firewall-rules delete --quiet --project " + o.Flags.ProjectID
		for _, name := range nameToDelete {
			args = args + " " + name
		}
		args = args + " || true"
		return args, nil
	}

	return "# No firewalls found for deletion", nil
}

func (o *GCGKEOptions) cleanUpPersistentDisks() ([]string, error) {
	zones, err := o.getZones()
	if err != nil {
		return nil, err
	}
	var line []string

	for _, z := range zones {
		disks, err := o.getUnusedDisksForZone(z)
		if err != nil {
			return nil, err
		}

		for _, d := range disks {
			if strings.HasPrefix(d.Name, "gke-") {
				line = append(line, fmt.Sprintf("gcloud compute disks delete --zone=%s --quiet %s --project %s || true", z.Name, d.Name, o.Flags.ProjectID))
			}
		}
	}

	if len(line) == 0 {
		line = append(line, "# No disks found for deletion\n")
	}

	return line, nil
}

func (o *GCGKEOptions) cleanUpAddresses() ([]string, error) {

	cmd := "gcloud compute addresses list --filter=\"status:RESERVED\" --format=json --project " + o.Flags.ProjectID
	data, err := o.GetCommandOutput("", "bash", "-c", cmd)
	if err != nil {
		return nil, err
	}

	var addresses []address
	err = json.Unmarshal([]byte(data), &addresses)
	if err != nil {
		return nil, err
	}

	var line []string
	if len(addresses) > 0 {
		for _, address := range addresses {
			var scope string
			if address.Region != "" {
				region := getLastString(strings.Split(address.Region, "/"))
				scope = fmt.Sprintf("--region %s", region)
			} else {
				scope = "--global"
			}
			line = append(line, fmt.Sprintf("gcloud compute addresses delete %s %s --project %s || true", address.Name, scope, o.Flags.ProjectID))
		}
		return line, nil
	}

	if len(line) == 0 {
		line = append(line, "# No addresses found for deletion\n")
	}

	return line, nil
}

func (o *GCGKEOptions) cleanUpServiceAccounts() ([]string, error) {
	serviceAccounts, err := o.getServiceAccounts()
	if err != nil {
		return nil, err
	}

	clusters, err := o.getClusters()
	if err != nil {
		return nil, err
	}

	serviceAccounts, err = o.getFilteredServiceAccounts(serviceAccounts, clusters)
	if err != nil {
		return nil, err
	}
	var line []string

	if len(serviceAccounts) > 0 {
		for _, sa := range serviceAccounts {
			log.Logger().Debugf("About to delete service account %s", sa)
			line = append(line, fmt.Sprintf("gcloud iam service-accounts delete %s --quiet --project %s || true", sa.Email, o.Flags.ProjectID))
		}
	}

	if len(line) == 0 {
		line = append(line, "# No service accounts found for deletion\n")
	}

	policy, err := o.getIamPolicy()
	if err != nil {
		return nil, err
	}

	iam, err := o.determineUnusedIamBindings(policy)
	if err != nil {
		return nil, err
	}

	if len(iam) == 0 {
		line = append(line, "# No iam policy bindings found for deletion\n")
	}

	line = append(line, iam...)

	if len(line) == 0 {
		line = append(line, "# No service accounts found for deletion\n")
	}

	return line, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.HasPrefix(e, a) {
			return true
		}
	}
	return false
}

func getLastString(s []string) string {
	return s[len(s)-1]
}

func (o *GCGKEOptions) getZones() ([]zone, error) {
	cmd := "gcloud compute zones list --format=json --project " + o.Flags.ProjectID
	data, err := o.GetCommandOutput("", "bash", "-c", cmd)
	if err != nil {
		return nil, err
	}

	var zones []zone
	err = json.Unmarshal([]byte(data), &zones)
	if err != nil {
		return nil, err
	}

	return zones, nil
}

func (o *GCGKEOptions) getClusters() ([]cluster, error) {
	cmd := "gcloud container clusters list --format=json --project " + o.Flags.ProjectID
	data, err := o.GetCommandOutput("", "bash", "-c", cmd)
	if err != nil {
		return nil, err
	}

	var clusters []cluster
	err = json.Unmarshal([]byte(data), &clusters)
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func (o *GCGKEOptions) getUnusedDisksForZone(z zone) ([]disk, error) {
	diskCmd := fmt.Sprintf("gcloud compute disks list --filter=\"NOT users:* AND zone:(%s)\" --format=json  --project %s", z.Name, o.Flags.ProjectID)
	data, err := o.GetCommandOutput("", "bash", "-c", diskCmd)
	if err != nil {
		return nil, err
	}

	var disks []disk
	err = json.Unmarshal([]byte(data), &disks)
	if err != nil {
		return nil, err
	}

	return disks, nil
}

func (o *GCGKEOptions) getFilteredServiceAccounts(serviceAccounts []serviceAccount, clusters []cluster) ([]serviceAccount, error) {

	filteredServiceAccounts := []serviceAccount{}
	for _, sa := range serviceAccounts {
		if isServiceAccount(sa.DisplayName) {
			if o.shouldRemoveServiceAccount(sa.DisplayName, clusters) {
				log.Logger().Debugf("Adding service account to filtered service account list %s", sa.DisplayName)
				filteredServiceAccounts = append(filteredServiceAccounts, sa)
			}
		}
	}
	return filteredServiceAccounts, nil
}

func (o *GCGKEOptions) shouldRemoveServiceAccount(saDisplayName string, clusters []cluster) bool {
	sz := len(saDisplayName)
	clusterName := saDisplayName[:sz-3]
	if strings.HasPrefix(clusterName, "pr") {
		// clusters with '-' in names such as BDD test clusters
		// e.g. pr-331-170-gitop-vt
		clusterNameParts := strings.Split(clusterName, "-")
		if len(clusterNameParts) > 2 {
			clusterNamePrefix := clusterNameParts[0] + "-" + clusterNameParts[1] + "-" + clusterNameParts[2]
			if !o.clusterExistsWithPrefix(clusters, clusterNamePrefix) {
				log.Logger().Debugf("cluster with prefix %s does not exist", clusterNamePrefix)
				return true
			}
		}
	} else {
		if !o.clusterExists(clusters, clusterName) {
			// clusters that don't start with pr
			log.Logger().Debugf("cluster %s does not exist", clusterName)
			return true
		}
	}
	log.Logger().Debugf("cluster %s exists, excluding service account %s", clusterName, saDisplayName)
	return false
}

func (o *GCGKEOptions) getServiceAccounts() ([]serviceAccount, error) {
	cmd := "gcloud iam service-accounts list --format=json --project " + o.Flags.ProjectID
	data, err := o.GetCommandOutput("", "bash", "-c", cmd)
	if err != nil {
		return nil, err
	}

	var serviceAccounts []serviceAccount
	err = json.Unmarshal([]byte(data), &serviceAccounts)
	if err != nil {
		return nil, err
	}

	for _, sa := range serviceAccounts {
		if sa.DisplayName == "" {
			sa.DisplayName = sa.Email[:strings.IndexByte(sa.Email, '@')]
		}
	}
	return serviceAccounts, nil
}

func (o *GCGKEOptions) clusterExistsWithPrefix(clusters []cluster, clusterNamePrefix string) bool {
	for _, cluster := range clusters {
		if strings.HasPrefix(cluster.Name, clusterNamePrefix) {
			return true
		}
	}
	return false
}

func (o *GCGKEOptions) clusterExists(clusters []cluster, clusterName string) bool {
	for _, cluster := range clusters {
		if cluster.Name == clusterName {
			return true
		}
	}
	return false
}

func (o *GCGKEOptions) getCurrentGoogleProjectId() (string, error) {
	cmd := "gcloud config get-value core/project"
	data, err := o.GetCommandOutput("", "bash", "-c", cmd)
	if err != nil {
		return "", err
	}
	return data, nil
}

func (o *GCGKEOptions) getIamPolicy() (iamPolicy, error) {
	cmd := fmt.Sprintf("gcloud projects get-iam-policy %s --format=json", o.Flags.ProjectID)
	data, err := o.GetCommandOutput("", "bash", "-c", cmd)
	if err != nil {
		return iamPolicy{}, err
	}

	var policy iamPolicy
	err = json.Unmarshal([]byte(data), &policy)
	if err != nil {
		return iamPolicy{}, err
	}

	return policy, nil
}

func (o *GCGKEOptions) determineUnusedIamBindings(policy iamPolicy) ([]string, error) {
	var line []string

	clusters, err := o.getClusters()
	if err != nil {
		return line, err
	}
	for _, b := range policy.Bindings {
		for _, m := range b.Members {
			if strings.HasPrefix(m, "deleted:serviceAccount:") || strings.HasPrefix(m, "serviceAccount:") {
				saName := strings.TrimPrefix(m, "deleted:")
				saName = strings.TrimPrefix(saName, "serviceAccount:")
				displayName := saName[:strings.IndexByte(saName, '@')]

				if isServiceAccount(displayName) {
					if o.shouldRemoveServiceAccount(displayName, clusters) {
						cmd := fmt.Sprintf("gcloud projects remove-iam-policy-binding %s --member=%s --role=%s --quiet", o.Flags.ProjectID, m, b.Role)
						line = append(line, cmd)
					}
				}
			}
		}
	}
	return line, nil
}

func isServiceAccount(sa string) bool {
	for _, suffix := range ServiceAccountSuffixes {
		if strings.HasSuffix(sa, suffix) {
			return true
		}
	}
	return false
}
