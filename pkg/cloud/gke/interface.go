package gke

import "k8s.io/client-go/kubernetes"

// GClouder interface to define interactions with the gcloud command
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/cloud/gke GClouder -o mocks/gclouder.go
type GClouder interface {
	CreateManagedZone(projectID string, domain string) error
	CreateDNSZone(projectID string, domain string) (string, []string, error)
	GetManagedZoneNameServers(projectID string, domain string) (string, []string, error)
	ClusterZone(cluster string) (string, error)
	BucketExists(projectID string, bucketName string) (bool, error)
	CreateBucket(projectID string, bucketName string, location string) error
	AddBucketLabel(bucketName string, label string)
	FindBucket(bucketName string) bool
	DeleteAllObjectsInBucket(bucketName string) error
	DeleteBucket(bucketName string) error
	FindServiceAccount(serviceAccount string, projectID string) bool
	GetOrCreateServiceAccount(serviceAccount string, projectID string, clusterConfigDir string, roles []string) (string, error)
	CreateServiceAccountKey(serviceAccount string, projectID string, keyPath string) error
	GetServiceAccountKeys(serviceAccount string, projectID string) ([]string, error)
	ListClusters(region string, projectID string) ([]Cluster, error)
	LoadGkeCluster(region string, projectID string, clusterName string) (*Cluster, error)
	UpdateGkeClusterLabels(region string, projectID string, clusterName string, labels []string) error
	DeleteServiceAccountKey(serviceAccount string, projectID string, key string) error
	CleanupServiceAccountKeys(serviceAccount string, projectID string) error
	DeleteServiceAccount(serviceAccount string, projectID string, roles []string) error
	GetEnabledApis(projectID string) ([]string, error)
	EnableAPIs(projectID string, apis ...string) error
	Login(serviceAccountKeyPath string, skipLogin bool) error
	CheckPermission(perm string, projectID string) (bool, error)
	CreateKmsKeyring(keyringName string, projectID string) error
	IsKmsKeyringAvailable(keyringName string, projectID string) bool
	CreateKmsKey(keyName string, keyringName string, projectID string) error
	IsKmsKeyAvailable(keyName string, keyringName string, projectID string) bool
	IsGCSWriteRoleEnabled(cluster string, zone string) (bool, error)
	UserLabel() string
	CreateGCPServiceAccount(kubeClient kubernetes.Interface, serviceName, serviceAbbreviation, namespace, clusterName, projectID string, serviceAccountRoles []string, serviceAccountSecretKey string) (string, error)
	ConnectToCluster(projectID, zone, clusterName string) error
	ConnectToRegionCluster(projectID, region, clusterName string) error
	ConfigureBucketRoles(projectID string, serviceAccount string, bucketURL string, roles []string) error
	GetProjectNumber(projectID string) (string, error)
}
