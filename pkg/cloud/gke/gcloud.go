package gke

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"time"

	osUser "os/user"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// KmsLocation indicates the location used by the Google KMS service
const KmsLocation = "global"

var (
	// RequiredServiceAccountRoles the roles required to create a cluster with terraform
	RequiredServiceAccountRoles = []string{"roles/owner"}

	// KanikoServiceAccountRoles the roles required to run kaniko with GCS
	KanikoServiceAccountRoles = []string{"roles/storage.admin",
		"roles/storage.objectAdmin",
		"roles/storage.objectCreator"}

	// VeleroServiceAccountRoles the roles required to run velero with GCS
	VeleroServiceAccountRoles = []string{
		/* TODO
		"roles/compute.disks.get",
		"roles/compute.disks.create",
		"roles/compute.disks.createSnapshot",
		"roles/compute.snapshots.get",
		"roles/compute.snapshots.create",
		"roles/compute.snapshots.useReadOnly",
		"roles/compute.snapshots.delete",
		"roles/compute.zones.get",
		*/
		"roles/storage.admin",
		"roles/storage.objectAdmin",
		"roles/storage.objectCreator"}
)

// GCloud real implementation of the gcloud helper
type GCloud struct {
}

// Cluster struct to represent a cluster on gcloud
type Cluster struct {
	Name           string            `json:"name,omitempty"`
	ResourceLabels map[string]string `json:"resourceLabels,omitempty"`
	Status         string            `json:"status,omitempty"`
	Location       string            `json:"location,omitempty"`
}

type recordSet struct {
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Rrdatas []string `json:"rrdatas"`
	TTL     int      `json:"ttl"`
	Type    string   `json:"type"`
}

// generateManagedZoneName constructs and returns a managed zone name using the domain value
func generateManagedZoneName(domain string) string {

	var managedZoneName string

	if domain != "" {
		managedZoneName = strings.Replace(domain, ".", "-", -1)
		return fmt.Sprintf("%s-zone", managedZoneName)
	}

	return ""

}

// getManagedZoneName checks for a given domain zone within the specified project and returns its name
func getManagedZoneName(projectID string, domain string) (string, error) {
	args := []string{"dns",
		"managed-zones",
		fmt.Sprintf("--project=%s", projectID),
		"list",
		fmt.Sprintf("--filter=%s.", domain),
		"--format=json",
	}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}

	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return "", errors.Wrap(err, "executing gcloud dns managed-zones list command ")
	}

	type managedZone struct {
		Name string `json:"name"`
	}

	var managedZones []managedZone

	err = yaml.Unmarshal([]byte(output), &managedZones)
	if err != nil {
		return "", errors.Wrap(err, "unmarshalling gcloud response")
	}

	if len(managedZones) == 1 {
		return managedZones[0].Name, nil
	}

	return "", nil
}

// CreateManagedZone creates a managed zone for the given domain in the specified project
func (g *GCloud) CreateManagedZone(projectID string, domain string) error {
	managedZoneName, err := getManagedZoneName(projectID, domain)
	if err != nil {
		return errors.Wrap(err, "unable to determine whether managed zone exists")
	}
	if managedZoneName == "" {
		log.Logger().Infof("Managed Zone doesn't exist for %s domain, creating...", domain)
		managedZoneName := generateManagedZoneName(domain)
		args := []string{"dns",
			"managed-zones",
			fmt.Sprintf("--project=%s", projectID),
			"create",
			managedZoneName,
			"--dns-name",
			fmt.Sprintf("%s.", domain),
			"--description=managed-zone utilised by jx",
		}

		cmd := util.Command{
			Name: "gcloud",
			Args: args,
		}

		_, err := cmd.RunWithoutRetry()
		if err != nil {
			return errors.Wrap(err, "executing gcloud dns managed-zones list command ")
		}
	} else {
		log.Logger().Infof("Managed Zone exists for %s domain.", domain)
	}
	return nil
}

// CreateDNSZone creates the DNS zone if it doesn't exist
// and returns the list of name servers for the given domain and project
func (g *GCloud) CreateDNSZone(projectID string, domain string) (string, []string, error) {
	var managedZone, nameServers = "", []string{}
	err := g.CreateManagedZone(projectID, domain)
	if err != nil {
		return "", []string{}, errors.Wrap(err, "while trying to creating a CloudDNS managed zone")
	}
	managedZone, nameServers, err = g.GetManagedZoneNameServers(projectID, domain)
	if err != nil {
		return "", []string{}, errors.Wrap(err, "while trying to retrieve the managed zone name servers")
	}

	// Update TTL to 60 for managed zone NS record
	err = updateManagedZoneRecordTTL(projectID, domain, managedZone, "NS", 60)
	if err != nil {
		return "", []string{}, errors.Wrap(err, "when trying to update the managed zone NS record-set")
	}

	// Update TTL to 60 for managed zone SOA record
	err = updateManagedZoneRecordTTL(projectID, domain, managedZone, "SOA", 60)
	if err != nil {
		return "", []string{}, errors.Wrap(err, "when trying to update the managed zone SOA record-set")
	}

	return managedZone, nameServers, nil
}

// GetManagedZoneNameServers retrieves a list of name servers associated with a zone
func (g *GCloud) GetManagedZoneNameServers(projectID string, domain string) (string, []string, error) {
	var nameServers = []string{}
	managedZoneName, err := getManagedZoneName(projectID, domain)
	if err != nil {
		return "", []string{}, errors.Wrap(err, "unable to determine whether managed zone exists")
	}
	if managedZoneName != "" {
		log.Logger().Infof("Getting nameservers for %s domain", domain)
		args := []string{"dns",
			"managed-zones",
			fmt.Sprintf("--project=%s", projectID),
			"describe",
			managedZoneName,
			"--format=json",
		}

		cmd := util.Command{
			Name: "gcloud",
			Args: args,
		}

		type mz struct {
			Name        string   `json:"name"`
			NameServers []string `json:"nameServers"`
		}

		var managedZone mz

		output, err := cmd.RunWithoutRetry()
		if err != nil {
			return "", []string{}, errors.Wrap(err, "executing gcloud dns managed-zones list command ")
		}

		json.Unmarshal([]byte(output), &managedZone)
		if err != nil {
			return "", []string{}, errors.Wrap(err, "unmarshalling gcloud response when returning managed-zone nameservers")
		}
		nameServers = managedZone.NameServers
	} else {
		log.Logger().Infof("Managed Zone doesn't exist for %s domain.", domain)
	}
	return managedZoneName, nameServers, nil
}

func getManagedZoneRecordSet(parentProject string, parentZone string, mzRecordSet recordSet) (recordSet, error) {
	var googleRecordSet recordSet

	args := []string{"dns",
		"record-sets",
		fmt.Sprintf("--project=%s", parentProject),
		"list",
		fmt.Sprintf("--name=%s", mzRecordSet.Name),
		fmt.Sprintf("--zone=%s", parentZone),
		fmt.Sprintf("--filter=type:%s", mzRecordSet.Type),
		"--format=json",
	}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}

	output, err := cmd.Run()
	if err != nil {
		return googleRecordSet, errors.Wrap(err, "executing gcloud dns managed-zones list command")
	}
	var recordSets []recordSet
	err = yaml.Unmarshal([]byte(output), &recordSets)
	if err != nil {
		return googleRecordSet, errors.Wrap(err, "unmarshalling gcloud response")
	}

	if len(recordSets) == 1 {
		log.Logger().Infof("google dns record-set contains - domain: %s, with nameServers: %s\n", recordSets[0].Name, strings.Join(recordSets[0].Rrdatas, " "))
		googleRecordSet = recordSets[0]
	} else {
		log.Logger().Debugf("No record-set or more than one record-set found for %s in %s", mzRecordSet.Name, parentZone)
	}

	return googleRecordSet, nil
}

func applyManagedZoneRecordTTL(parentProject string, parentZone string, googleRecordSet recordSet, ttl int) error {

	if googleRecordSet.Name != "" {
		// transaction start
		startArgs := []string{"dns",
			"record-sets",
			fmt.Sprintf("--project=%s", parentProject),
			"transaction",
			"start",
			fmt.Sprintf("--zone=%s", parentZone),
			"--format=json",
		}
		startCmd := util.Command{
			Name: "gcloud",
			Args: startArgs,
		}

		_, err := startCmd.RunWithoutRetry()
		if err != nil {
			return errors.Wrap(err, "executing gcloud dns record-sets transaction start command")
		}

		// remove the previous record as it needs to be updated
		removeArgs1 := []string{"dns",
			"record-sets",
			fmt.Sprintf("--project=%s", parentProject),
			"transaction",
			"remove",
		}
		removeArgs2 := []string{fmt.Sprintf("--name=%s", googleRecordSet.Name),
			fmt.Sprintf("--ttl=%d", googleRecordSet.TTL),
			fmt.Sprintf("--type=%s", googleRecordSet.Type),
			fmt.Sprintf("--zone=%s", parentZone),
			"--format=json",
		}
		removeArgs := append(removeArgs1, googleRecordSet.Rrdatas...)
		removeArgs = append(removeArgs, removeArgs2...)

		removeCmd := util.Command{
			Name: "gcloud",
			Args: removeArgs,
		}

		_, err = removeCmd.RunWithoutRetry()
		if err != nil {
			return errors.Wrap(err, "executing gcloud dns record-sets transaction remove command")
		}

		// transaction add
		addArgs1 := []string{"dns",
			"record-sets",
			fmt.Sprintf("--project=%s", parentProject),
			"transaction",
			"add",
		}
		addArgs2 := []string{fmt.Sprintf("--name=%s", googleRecordSet.Name),
			fmt.Sprintf("--ttl=%d", ttl),
			fmt.Sprintf("--type=%s", googleRecordSet.Type),
			fmt.Sprintf("--zone=%s", parentZone),
			"--format=json",
		}
		addArgs := append(addArgs1, googleRecordSet.Rrdatas...)
		addArgs = append(addArgs, addArgs2...)

		addCmd := util.Command{
			Name: "gcloud",
			Args: addArgs,
		}

		_, err = addCmd.RunWithoutRetry()
		if err != nil {
			return errors.Wrap(err, "executing gcloud dns record-sets transaction add command")
		}

		// transaction execute
		executeArgs := []string{"dns",
			"record-sets",
			fmt.Sprintf("--project=%s", parentProject),
			"transaction",
			"execute",
			fmt.Sprintf("--zone=%s", parentZone),
			"--format=json",
		}
		executeCmd := util.Command{
			Name: "gcloud",
			Args: executeArgs,
		}

		_, err = executeCmd.RunWithoutRetry()
		if err != nil {
			return errors.Wrap(err, "executing gcloud dns record-sets transaction start command")
		}
	}
	return nil
}

func updateManagedZoneRecordTTL(projectID string, domain string, managedZone string, dnsType string, ttl int) error {
	var managedZoneRecord recordSet
	managedZoneRecord.Name = addDomainSuffix(domain)
	managedZoneRecord.Type = dnsType
	managedZoneRecord.TTL = ttl
	managedZoneRecordSet, err := getManagedZoneRecordSet(projectID, managedZone, managedZoneRecord)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("when retrieving the '%s' record-set of type '%s'", domain, dnsType))
	}

	err = applyManagedZoneRecordTTL(projectID, managedZone, managedZoneRecordSet, 60)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("when updating the '%s' record-set of type '%s'", domain, dnsType))
	}
	return nil
}

// ClusterZone retrives the zone of GKE cluster description
func (g *GCloud) ClusterZone(cluster string) (string, error) {
	args := []string{"container",
		"clusters",
		"describe",
		cluster}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return "", err
	}

	zone, err := parseClusterZone(output)
	if err != nil {
		return "", err
	}
	return zone, nil
}

func parseClusterZone(clusterInfo string) (string, error) {
	ci := struct {
		Zone string `json:"zone"`
	}{}

	err := yaml.Unmarshal([]byte(clusterInfo), &ci)
	if err != nil {
		return "", errors.Wrap(err, "extracting cluster zone from cluster info")
	}
	return ci.Zone, nil
}

type nodeConfig struct {
	OauthScopes []string `json:"oauthScopes"`
}

func parseScopes(clusterInfo string) ([]string, error) {

	ci := struct {
		NodeConfig nodeConfig `json:"nodeConfig"`
	}{}

	err := yaml.Unmarshal([]byte(clusterInfo), &ci)
	if err != nil {
		return nil, errors.Wrap(err, "extracting cluster oauthScopes from cluster info")
	}
	return ci.NodeConfig.OauthScopes, nil
}

// BucketExists checks if a Google Storage bucket exists
func (g *GCloud) BucketExists(projectID string, bucketName string) (bool, error) {
	fullBucketName := fmt.Sprintf("gs://%s", bucketName)
	args := []string{"ls"}

	if projectID != "" {
		args = append(args, "-p")
		args = append(args, projectID)
	}

	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	output, err := cmd.Run()
	if err != nil {
		log.Logger().Infof("Error checking bucket exists: %s, %s", output, err)
		return false, err
	}
	return strings.Contains(output, fullBucketName), nil
}

// ListObjects checks if a Google Storage bucket exists
func (g *GCloud) ListObjects(bucketName string, path string) ([]string, error) {
	fullBucketName := fmt.Sprintf("gs://%s/%s", bucketName, path)
	args := []string{"ls", fullBucketName}

	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		log.Logger().Infof("Error checking bucket exists: %s, %s", output, err)
		return []string{}, err
	}
	return strings.Split(output, "\n"), nil
}

// CreateBucket creates a new Google Storage bucket
func (g *GCloud) CreateBucket(projectID string, bucketName string, location string) error {
	fullBucketName := fmt.Sprintf("gs://%s", bucketName)
	args := []string{"mb", "-l", location}

	if projectID != "" {
		args = append(args, "-p")
		args = append(args, projectID)
	}

	args = append(args, fullBucketName)

	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		log.Logger().Infof("Error creating bucket: %s, %s", output, err)
		return err
	}
	return nil
}

//AddBucketLabel adds a label to a Google Storage bucket
func (g *GCloud) AddBucketLabel(bucketName string, label string) {
	found := g.FindBucket(bucketName)
	if found && label != "" {
		fullBucketName := fmt.Sprintf("gs://%s", bucketName)
		args := []string{"label", "ch", "-l", label}

		args = append(args, fullBucketName)

		cmd := util.Command{
			Name: "gsutil",
			Args: args,
		}
		output, err := cmd.RunWithoutRetry()
		if err != nil {
			log.Logger().Infof("Error adding bucket label: %s, %s", output, err)
		}
	}
}

// FindBucket finds a Google Storage bucket
func (g *GCloud) FindBucket(bucketName string) bool {
	fullBucketName := fmt.Sprintf("gs://%s", bucketName)
	args := []string{"list", "-b", fullBucketName}

	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return false
	}
	return true
}

// DeleteAllObjectsInBucket deletes all objects in a Google Storage bucket
func (g *GCloud) DeleteAllObjectsInBucket(bucketName string) error {
	found := g.FindBucket(bucketName)
	if !found {
		return nil // nothing to delete
	}
	fullBucketName := fmt.Sprintf("gs://%s", bucketName)
	args := []string{"-m", "rm", "-r", fullBucketName}

	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}

// StreamTransferFileFromBucket will perform a stream transfer from the GCS bucket to stdout and return a scanner
// with the piped result
func StreamTransferFileFromBucket(fullBucketURL string) (*bufio.Scanner, error) {
	args := []string{"cp", fullBucketURL, "-"}
	cmd := exec.Command("gsutil", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	return scanner, err
}

// DeleteBucket deletes a Google storage bucket
func (g *GCloud) DeleteBucket(bucketName string) error {
	found := g.FindBucket(bucketName)
	if !found {
		return nil // nothing to delete
	}
	fullBucketName := fmt.Sprintf("gs://%s", bucketName)
	args := []string{"rb", fullBucketName}

	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}

// GetRegionFromZone parses the region from a GCP zone name. TODO: Return an error if the format of the zone is not correct
func GetRegionFromZone(zone string) string {
	firstDash := strings.Index(zone, "-")
	lastDash := strings.LastIndex(zone, "-")
	if firstDash == lastDash { // It's a region, not a zone
		return zone
	}
	return zone[0:lastDash]
}

// FindServiceAccount checks if a service account exists
func (g *GCloud) FindServiceAccount(serviceAccount string, projectID string) bool {
	args := []string{"iam",
		"service-accounts",
		"list",
		"--filter",
		serviceAccount,
		"--project",
		projectID}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.Run()
	if err != nil {
		return false
	}

	if output == "Listed 0 items." {
		return false
	}
	return true
}

// GetOrCreateServiceAccount retrieves or creates a GCP service account. It will return the path to the file where the service
// account token is stored
func (g *GCloud) GetOrCreateServiceAccount(serviceAccount string, projectID string, clusterConfigDir string, roles []string) (string, error) {
	if projectID == "" {
		return "", errors.New("cannot get/create a service account without a projectId")
	}

	found := g.FindServiceAccount(serviceAccount, projectID)
	if !found {
		log.Logger().Infof("Unable to find service account %s, checking if we have enough permission to create", util.ColorInfo(serviceAccount))

		// if it doesn't check to see if we have permissions to create (assign roles) to a service account
		hasPerm, err := g.CheckPermission("resourcemanager.projects.setIamPolicy", projectID)
		if err != nil {
			return "", err
		}

		if !hasPerm {
			return "", errors.New("User does not have the required role 'resourcemanager.projects.setIamPolicy' to configure a service account")
		}

		// create service
		log.Logger().Infof("Creating service account %s", util.ColorInfo(serviceAccount))
		args := []string{"iam",
			"service-accounts",
			"create",
			serviceAccount,
			"--project",
			projectID,
			"--display-name",
			serviceAccount}

		cmd := util.Command{
			Name: "gcloud",
			Args: args,
		}
		_, err = cmd.RunWithoutRetry()
		if err != nil {
			return "", err
		}

		// assign roles to service account
		for _, role := range roles {
			log.Logger().Infof("Assigning role %s", role)
			args = []string{"projects",
				"add-iam-policy-binding",
				projectID,
				"--member",
				fmt.Sprintf("serviceAccount:%s@%s.iam.gserviceaccount.com", serviceAccount, projectID),
				"--role",
				role,
				"--project",
				projectID}

			cmd := util.Command{
				Name: "gcloud",
				Args: args,
			}
			_, err := cmd.Run()
			if err != nil {
				return "", err
			}
		}

	} else {
		log.Logger().Info("Service Account exists")
	}

	os.MkdirAll(clusterConfigDir, os.ModePerm)
	keyPath := filepath.Join(clusterConfigDir, fmt.Sprintf("%s.key.json", serviceAccount))

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		log.Logger().Info("Downloading service account key")
		err := g.CreateServiceAccountKey(serviceAccount, projectID, keyPath)
		if err != nil {
			log.Logger().Infof("Exceeds the maximum number of keys on service account %s",
				util.ColorInfo(serviceAccount))
			err := g.CleanupServiceAccountKeys(serviceAccount, projectID)
			if err != nil {
				return "", errors.Wrap(err, "cleaning up the service account keys")
			}
			err = g.CreateServiceAccountKey(serviceAccount, projectID, keyPath)
			if err != nil {
				return "", errors.Wrap(err, "creating service account key")
			}
		}
	} else {
		log.Logger().Info("Key already exists")
	}

	return keyPath, nil
}

// ConfigureBucketRoles gives the given roles to the given service account
func (g *GCloud) ConfigureBucketRoles(projectID string, serviceAccount string, bucketURL string, roles []string) error {
	member := fmt.Sprintf("serviceAccount:%s@%s.iam.gserviceaccount.com", serviceAccount, projectID)

	bindings := bucketMemberRoles{}
	for _, role := range roles {
		bindings.Bindings = append(bindings.Bindings, memberRole{
			Members: []string{member},
			Role:    role,
		})
	}
	file, err := ioutil.TempFile("", "gcp-iam-roles-")
	if err != nil {
		return errors.Wrapf(err, "failed to create temp file")
	}
	fileName := file.Name()

	data, err := json.Marshal(&bindings)
	if err != nil {
		return errors.Wrapf(err, "failed to convert bindings %#v to JSON", bindings)
	}
	log.Logger().Infof("created json %s", string(data))
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save bindings %#v to JSON file %s", bindings, fileName)
	}
	log.Logger().Infof("generated IAM bindings file %s", fileName)
	args := []string{
		"-m",
		"iam",
		"set",
		"-a",
		fileName,
		bucketURL,
	}
	cmd := util.Command{
		Name: "gsutil",
		Args: args,
	}
	log.Logger().Infof("running: gsutil %s", strings.Join(args, " "))
	_, err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

type bucketMemberRoles struct {
	Bindings []memberRole `json:"bindings"`
}

type memberRole struct {
	Members []string `json:"members"`
	Role    string   `json:"role"`
}

// CreateServiceAccountKey creates a new service account key and downloads into the given file
func (g *GCloud) CreateServiceAccountKey(serviceAccount string, projectID string, keyPath string) error {
	args := []string{"iam",
		"service-accounts",
		"keys",
		"create",
		keyPath,
		"--iam-account",
		fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccount, projectID),
		"--project",
		projectID}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrap(err, "creating a new service account key")
	}
	return nil
}

// GetServiceAccountKeys returns all keys of a service account
func (g *GCloud) GetServiceAccountKeys(serviceAccount string, projectID string) ([]string, error) {
	keys := []string{}
	account := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccount, projectID)
	args := []string{"iam",
		"service-accounts",
		"keys",
		"list",
		"--iam-account",
		account,
		"--project",
		projectID}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return keys, errors.Wrapf(err, "listing the keys of the service account '%s'", account)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	// Skip the first line with the header information
	scanner.Scan()
	for scanner.Scan() {
		keyFields := strings.Fields(scanner.Text())
		if len(keyFields) > 0 {
			keys = append(keys, keyFields[0])
		}
	}
	return keys, nil
}

// ListClusters returns the clusters in a GKE project
func (g *GCloud) ListClusters(region string, projectID string) ([]Cluster, error) {
	args := []string{"container", "clusters", "list", "--region=" + region, "--format=json", "--quiet"}
	if projectID != "" {
		args = append(args, "--project="+projectID)
	}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return nil, err
	}

	clusters := make([]Cluster, 0)
	err = json.Unmarshal([]byte(output), &clusters)
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

// LoadGkeCluster load a gke cluster from a GKE project
func (g *GCloud) LoadGkeCluster(region string, projectID string, clusterName string) (*Cluster, error) {
	args := []string{"container", "clusters", "describe", clusterName, "--region=" + region, "--format=json", "--quiet"}
	if projectID != "" {
		args = append(args, "--project="+projectID)
	}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return nil, err
	}

	cluster := &Cluster{}
	err = json.Unmarshal([]byte(output), cluster)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

// UpdateGkeClusterLabels updates labesl for a gke cluster
func (g *GCloud) UpdateGkeClusterLabels(region string, projectID string, clusterName string, labels []string) error {
	args := []string{"container", "clusters", "update", clusterName, "--quiet", "--update-labels=" + strings.Join(labels, ",") + ""}
	if region != "" {
		args = append(args, "--region="+region)
	}
	if projectID != "" {
		args = append(args, "--project="+projectID)
	}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	return err
}

// DeleteServiceAccountKey deletes a service account key
func (g *GCloud) DeleteServiceAccountKey(serviceAccount string, projectID string, key string) error {
	account := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccount, projectID)
	args := []string{"iam",
		"service-accounts",
		"keys",
		"delete",
		key,
		"--iam-account",
		account,
		"--project",
		projectID,
		"--quiet"}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "deleting the key '%s'from service account '%s'", key, account)
	}
	return nil
}

// CleanupServiceAccountKeys remove all keys from given service account
func (g *GCloud) CleanupServiceAccountKeys(serviceAccount string, projectID string) error {
	keys, err := g.GetServiceAccountKeys(serviceAccount, projectID)
	if err != nil {
		return errors.Wrap(err, "retrieving the service account keys")
	}

	log.Logger().Infof("Cleaning up the keys of the service account %s", util.ColorInfo(serviceAccount))

	for _, key := range keys {
		err := g.DeleteServiceAccountKey(serviceAccount, projectID, key)
		if err != nil {
			log.Logger().Infof("Cannot delete the key %s from service account %s: %v",
				util.ColorWarning(key), util.ColorInfo(serviceAccount), err)
		} else {
			log.Logger().Infof("Key %s was removed form service account %s",
				util.ColorInfo(key), util.ColorInfo(serviceAccount))
		}
	}
	return nil
}

// DeleteServiceAccount deletes a service account and its role bindings
func (g *GCloud) DeleteServiceAccount(serviceAccount string, projectID string, roles []string) error {
	found := g.FindServiceAccount(serviceAccount, projectID)
	if !found {
		return nil // nothing to delete
	}
	// remove roles to service account
	for _, role := range roles {
		log.Logger().Infof("Removing role %s", role)
		args := []string{"projects",
			"remove-iam-policy-binding",
			projectID,
			"--member",
			fmt.Sprintf("serviceAccount:%s@%s.iam.gserviceaccount.com", serviceAccount, projectID),
			"--role",
			role,
			"--project",
			projectID}

		cmd := util.Command{
			Name: "gcloud",
			Args: args,
		}
		_, err := cmd.RunWithoutRetry()
		if err != nil {
			return err
		}
	}
	args := []string{"iam",
		"service-accounts",
		"delete",
		fmt.Sprintf("%s@%s.iam.gserviceaccount.com", serviceAccount, projectID),
		"--project",
		projectID}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}

// GetEnabledApis returns which services have the API enabled
func (g *GCloud) GetEnabledApis(projectID string) ([]string, error) {
	args := []string{"services", "list", "--enabled"}

	if projectID != "" {
		args = append(args, "--project")
		args = append(args, projectID)
	}

	apis := []string{}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}

	out, err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if strings.Contains(l, "NAME") {
			continue
		}
		fields := strings.Fields(l)
		apis = append(apis, fields[0])
	}

	return apis, nil
}

// EnableAPIs enables APIs for the given services
func (g *GCloud) EnableAPIs(projectID string, apis ...string) error {
	enabledApis, err := g.GetEnabledApis(projectID)
	if err != nil {
		return err
	}

	toEnableArray := []string{}

	for _, toEnable := range apis {
		fullName := fmt.Sprintf("%s.googleapis.com", toEnable)
		if !util.Contains(enabledApis, fullName) {
			toEnableArray = append(toEnableArray, fullName)
		}
	}

	if len(toEnableArray) == 0 {
		log.Logger().Debugf("No apis need to be enable as they are already enabled: %s", util.ColorInfo(strings.Join(apis, " ")))
		return nil
	}

	args := []string{"services", "enable"}
	args = append(args, toEnableArray...)

	if projectID != "" {
		args = append(args, "--project")
		args = append(args, projectID)
	}

	log.Logger().Debugf("Lets ensure we have %s enabled on your project via: %s", toEnableArray, util.ColorInfo("gcloud "+strings.Join(args, " ")))

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err = cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}

// Login login an user into Google account. It skips the interactive login using the
// browser when the skipLogin flag is active
func (g *GCloud) Login(serviceAccountKeyPath string, skipLogin bool) error {
	if serviceAccountKeyPath != "" {
		log.Logger().Infof("Activating service account %s", util.ColorInfo(serviceAccountKeyPath))

		if _, err := os.Stat(serviceAccountKeyPath); os.IsNotExist(err) {
			return errors.New("Unable to locate service account " + serviceAccountKeyPath)
		}

		cmd := util.Command{
			Name: "gcloud",
			Args: []string{"auth", "activate-service-account", "--key-file", serviceAccountKeyPath},
		}
		_, err := cmd.RunWithoutRetry()
		if err != nil {
			return err
		}

		// GCP IAM changes can take up to 80 seconds to propagate
		retry(10, 10*time.Second, func() error {
			log.Logger().Infof("Checking for readiness...")

			projects, err := GetGoogleProjects()
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				return errors.New("service account not ready yet")
			}

			return nil
		})

	} else if !skipLogin {
		cmd := util.Command{
			Name: "gcloud",
			Args: []string{"auth", "login", "--brief"},
		}
		_, err := cmd.RunWithoutRetry()
		if err != nil {
			return err
		}
	}
	return nil
}

func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if s, ok := err.(stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return retry(attempts, 2*sleep, fn)
		}
		return err
	}
	return nil
}

type stop struct {
	error
}

// CheckPermission checks permission on the given project
func (g *GCloud) CheckPermission(perm string, projectID string) (bool, error) {
	if projectID == "" {
		return false, errors.New("cannot check permission without a projectId")
	}
	// if it doesn't check to see if we have permissions to create (assign roles) to a service account
	args := []string{"iam",
		"list-testable-permissions",
		fmt.Sprintf("//cloudresourcemanager.googleapis.com/projects/%s", projectID),
		"--filter",
		perm}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return false, err
	}

	return strings.Contains(output, perm), nil
}

// CreateKmsKeyring creates a new KMS keyring
func (g *GCloud) CreateKmsKeyring(keyringName string, projectID string) error {
	if keyringName == "" {
		return errors.New("provided keyring name is empty")
	}

	if g.IsKmsKeyringAvailable(keyringName, projectID) {
		log.Logger().Debugf("keyring '%s' already exists", keyringName)
		return nil
	}

	args := []string{"kms",
		"keyrings",
		"create",
		keyringName,
		"--location",
		KmsLocation,
		"--project",
		projectID,
	}

	log.Logger().Debugf("creating keyring '%s' project=%s, location=%s", keyringName, projectID, KmsLocation)

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrap(err, "creating kms keyring")
	}
	return nil
}

// IsKmsKeyringAvailable checks if the KMS keyring is already available
func (g *GCloud) IsKmsKeyringAvailable(keyringName string, projectID string) bool {
	log.Logger().Debugf("IsKmsKeyringAvailable keyring=%s, projectId=%s, location=%s", keyringName, projectID, KmsLocation)
	args := []string{"kms",
		"keyrings",
		"describe",
		keyringName,
		"--location",
		KmsLocation,
		"--project",
		projectID,
	}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return false
	}
	return true
}

// CreateKmsKey creates a new KMS key in the given keyring
func (g *GCloud) CreateKmsKey(keyName string, keyringName string, projectID string) error {
	if g.IsKmsKeyAvailable(keyName, keyringName, projectID) {
		log.Logger().Debugf("key '%s' already exists", keyName)
		return nil
	}

	log.Logger().Debugf("creating key '%s' keyring=%s, project=%s, location=%s", keyName, keyringName, projectID, KmsLocation)

	args := []string{"kms",
		"keys",
		"create",
		keyName,
		"--location",
		KmsLocation,
		"--keyring",
		keyringName,
		"--purpose",
		"encryption",
		"--project",
		projectID,
	}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "creating kms key '%s' into keyring '%s'", keyName, keyringName)
	}
	return nil
}

// IsKmsKeyAvailable checks if the KMS key is already available
func (g *GCloud) IsKmsKeyAvailable(keyName string, keyringName string, projectID string) bool {
	log.Logger().Debugf("IsKmsKeyAvailable keyName=%s, keyring=%s, projectId=%s, location=%s", keyName, keyringName, projectID, KmsLocation)

	args := []string{"kms",
		"keys",
		"describe",
		keyName,
		"--location",
		KmsLocation,
		"--keyring",
		keyringName,
		"--project",
		projectID,
	}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return false
	}
	return true
}

// IsGCSWriteRoleEnabled will check if the devstorage.full_control scope is enabled in the cluster in order to use GCS
func (g *GCloud) IsGCSWriteRoleEnabled(cluster string, zone string) (bool, error) {
	args := []string{"container",
		"clusters",
		"describe",
		cluster,
		"--zone",
		zone}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return false, err
	}

	oauthScopes, err := parseScopes(output)
	if err != nil {
		return false, err
	}

	for _, s := range oauthScopes {
		if strings.Contains(s, "devstorage.full_control") {
			return true, nil
		}
	}
	return false, nil
}

// ConnectToCluster connects to the specified cluster
func (g *GCloud) ConnectToCluster(projectID, zone, clusterName string) error {
	args := []string{"container",
		"clusters",
		"get-credentials",
		clusterName,
		"--zone",
		zone,
		"--project", projectID}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to connect to cluster %s", clusterName)
	}
	return nil
}

// ConnectToRegionCluster connects to the specified regional cluster
func (g *GCloud) ConnectToRegionCluster(projectID, region, clusterName string) error {
	args := []string{"container",
		"clusters",
		"get-credentials",
		clusterName,
		"--region",
		region,
		"--project", projectID}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to connect to region cluster %s", clusterName)
	}
	return nil
}

// UserLabel returns a string identifying current user that can be used as a label
func (g *GCloud) UserLabel() string {
	user, err := osUser.Current()
	if err == nil && user != nil && user.Username != "" {
		userLabel := util.SanitizeLabel(user.Username)
		return fmt.Sprintf("created-by:%s", userLabel)
	}
	return ""
}

// CreateGCPServiceAccount creates a service account in GCP for a service using the account roles specified
func (g *GCloud) CreateGCPServiceAccount(kubeClient kubernetes.Interface, serviceName, serviceAbbreviation, namespace, clusterName, projectID string, serviceAccountRoles []string, serviceAccountSecretKey string) (string, error) {
	serviceAccountDir, err := ioutil.TempDir("", "gke")
	if err != nil {
		return "", errors.Wrap(err, "creating a temporary folder where the service account will be stored")
	}
	defer os.RemoveAll(serviceAccountDir)

	serviceAccountName := naming.ToValidGCPServiceAccount(ServiceAccountName(clusterName, serviceAbbreviation))

	serviceAccountPath, err := g.GetOrCreateServiceAccount(serviceAccountName, projectID, serviceAccountDir, serviceAccountRoles)
	if err != nil {
		return "", errors.Wrap(err, "creating the service account")
	}

	secretName, err := g.storeGCPServiceAccountIntoSecret(kubeClient, serviceAccountPath, serviceName, namespace, serviceAccountSecretKey)
	if err != nil {
		return "", errors.Wrap(err, "storing the service account into a secret")
	}
	return secretName, nil
}

func (g *GCloud) storeGCPServiceAccountIntoSecret(client kubernetes.Interface, serviceAccountPath, serviceName, namespace string, serviceAccountSecretKey string) (string, error) {
	serviceAccount, err := ioutil.ReadFile(serviceAccountPath)
	if err != nil {
		return "", errors.Wrapf(err, "reading the service account from file '%s'", serviceAccountPath)
	}

	secretName := GcpServiceAccountSecretName(serviceName)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Data: map[string][]byte{
			serviceAccountSecretKey: serviceAccount,
		},
	}

	secrets := client.CoreV1().Secrets(namespace)
	_, err = secrets.Get(secretName, metav1.GetOptions{})
	if err != nil {
		_, err = secrets.Create(secret)
	} else {
		_, err = secrets.Update(secret)
	}
	return secretName, nil
}

// CurrentProject returns the current GKE project name if it can be detected
func (g *GCloud) CurrentProject() (string, error) {
	args := []string{"config",
		"list",
		"--format",
		"value(core.project)",
	}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	text, err := cmd.RunWithoutRetry()
	if err != nil {
		return text, errors.Wrap(err, "failed to detect the current GCP project")
	}
	return strings.TrimSpace(text), nil
}

func (g *GCloud) GetProjectNumber(projectID string) (string, error) {
	args := []string{
		"projects",
		"describe",
		projectID,
		"--format=json",
	}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}

	log.Logger().Infof("running: gcloud %s", strings.Join(args, " "))
	output, err := cmd.Run()
	if err != nil {
		return "", err
	}

	var project project
	err = json.Unmarshal([]byte(output), &project)
	if err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal %s", output)
	}
	return project.ProjectNumber, nil
}

type project struct {
	ProjectNumber string `json:"projectNumber"`
}

func addDomainSuffix(domain string) string {
	if domain[len(domain)-1:] != "." {
		return fmt.Sprintf("%s.", domain)
	}
	return domain
}
