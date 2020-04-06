package kube

const (
	// DefaultChartMuseumURL default URL for Jenkins X Charts
	DefaultChartMuseumURL = "https://storage.googleapis.com/chartmuseum.jenkins-x.io"
	// DefaultChartMuseumJxRepoName default repo name for Jenkins X Charts
	DefaultChartMuseumJxRepoName = "jenkins-x"

	// ChartAmbassador the default chart for ambassador
	ChartAmbassador = "datawire/ambassador"

	// ChartAnchore the default chart for the Anchore plugin
	ChartAnchore = "stable/anchore-engine"

	// ChartExposecontrollerService the default name of the Exposecontroller Service chart for Edit environments
	ChartExposecontrollerService = "jenkins-x/exposecontroller-service"

	// ChartAnchore the default chart for the Anchore plugin
	ChartPipelineEvent = "jenkins-x/pipeline-events-addon"

	// ChartGitea the default name of the gitea chart
	ChartGitea = "jenkins-x/gitea"

	// ChartFlagger the default chart for the Flagger chart
	ChartFlagger              = "flagger/flagger"
	ChartFlaggerGrafana       = "flagger/grafana"
	DefaultFlaggerReleaseName = "flagger"

	// ChartIstio the default chart for the Istio chart
	ChartIstio = "install/kubernetes/helm/istio"

	// ChartKubeless the default chart for kubeless
	ChartKubeless = "incubator/kubeless"

	// ChartProw the default chart for Prow
	ChartProw = "jenkins-x/prow"

	// ChartKnative the default chart for knative
	ChartKnativeBuild = "jenkins-x/knative-build"

	// ChartBuildTemplates the build templates for Knative Build
	ChartBuildTemplates = "jenkins-x/jx-build-templates"

	// ChartTekton the default chart for tekton
	ChartTekton = "jenkins-x/tekton"

	// DefaultProwReleaseName the default helm release name for Prow
	DefaultProwReleaseName = "jx-prow"

	// DefaultKnativeBuildReleaseName the default helm release name for knative build
	DefaultKnativeBuildReleaseName = "knative-build"

	// DefaultTektonReleaseName the default helm release name for tekton
	DefaultTektonReleaseName = "tekton"

	// DefaultBuildTemplatesReleaseName the default helm release name for the knative build templates
	DefaultBuildTemplatesReleaseName = "jx-build-templates"

	// Charts Single Sign-On addon
	ChartSsoOperator              = "jenkins-x/sso-operator"
	DefaultSsoOperatorReleaseName = "jx-sso-operator"
	ChartSsoDex                   = "jenkins-x/dex"
	DefaultSsoDexReleaseName      = "jx-sso-dex"

	// ChartVaultOperator the default chart for vault opeator
	ChartVaultOperator              = "jenkins-x/vault-operator"
	DefaultVaultOperatorReleaseName = "vault-operator"

	//ChartExternalDNS the default chart for external-dns
	ChartOwnerExternalDNS         = "bitnami"
	ChartURLExternalDNS           = "https://charts.bitnami.com/bitnami"
	ChartExternalDNS              = "bitnami/external-dns"
	DefaultExternalDNSReleaseName = "external-dns"
	DefaultExternalDNSTag         = "1.5.2"

	// SecretKaniko the name of the secret containing the kaniko service account
	SecretKaniko = "kaniko-secret"

	// SecretVelero the name of the secret containing the velero service account
	SecretVelero = "velero-secret" // #nosec

	// ServiceJenkins is the name of the Jenkins Service
	ServiceJenkins = "jenkins"

	// SecretJenkins is the name of the Jenkins secret
	SecretJenkins = "jenkins"

	// ServiceChartMuseum the service name of the Helm ChartMuseum service
	ServiceChartMuseum = "jenkins-x-chartmuseum"

	// ServiceKubernetesDashboard the Kubernetes dashboard
	ServiceKubernetesDashboard = "jenkins-x-kubernetes-dashboard"

	// SecretJenkinsChartMuseum the chart museum secret
	SecretJenkinsChartMuseum = "jenkins-x-chartmuseum"

	// SecretBucketRepo the bucket repo secret if using it as a chart repositoru
	SecretBucketRepo = "jenkins-x-bucketrepo"

	// SecretJenkinsReleaseGPG the GPG secrets for doing releases
	SecretJenkinsReleaseGPG = "jenkins-release-gpg"

	// SecretJenkinsPipelinePrefix prefix for a jenkins pipeline secret name
	SecretJenkinsPipelinePrefix = "jx-pipeline-"

	// SecretJenkinsPipelineAddonCredentials the chat credentials secret
	SecretJenkinsPipelineAddonCredentials = "jx-pipeline-addon-" // #nosec

	// SecretJenkinsPipelineChatCredentials the chat credentials secret
	SecretJenkinsPipelineChatCredentials = "jx-pipeline-chat-"

	// SecretJenkinsPipelineGitCredentials the git credentials secret
	SecretJenkinsPipelineGitCredentials = "jx-pipeline-git-" // #nosec

	// SecretJenkinsPipelineIssueCredentials the issue tracker credentials secret
	SecretJenkinsPipelineIssueCredentials = "jx-pipeline-issues-" // #nosec

	// ConfigMapExposecontroller the name of the ConfigMap with the Exposecontroller configuration
	ConfigMapExposecontroller = "exposecontroller"

	// ConfigMapIngressConfig the new name of the ConfigMap with the Exposecontroller configuration
	ConfigMapIngressConfig = "ingress-config"

	// ConfigMapJenkinsX the name of the ConfigMap with the Jenkins configuration
	ConfigMapJenkinsX = "jenkins"

	// ConfigMapJenkinsPodTemplates is the ConfigMap containing all the Pod Templates available
	ConfigMapJenkinsPodTemplates = "jenkins-x-pod-templates"

	// ConfigMapJenkinsTeamController is the ConfigMap containing the TeamController config files
	ConfigMapJenkinsTeamController = "jenkins-x-team-controller"

	// ConfigMapJenkinsDockerRegistry is the ConfigMap containing the Docker Registry configuration
	ConfigMapJenkinsDockerRegistry = "jenkins-x-docker-registry"

	// ConfigMapNameJXInstallConfig is the ConfigMap containing the jx installation's CA and server url. Used by jx login
	ConfigMapNameJXInstallConfig = "jx-install-config"

	// LocalHelmRepoName is the default name of the local chart repository where CI/CD releases go to
	LocalHelmRepoName = "releases"

	// DeploymentTektonController the name of the Deployment for the Tekton Pipeline controller
	DeploymentTektonController = "tekton-pipelines-controller"

	// DeploymentExposecontrollerService the name of the Deployment for the Exposecontroller Service
	DeploymentExposecontrollerService = "exposecontroller-service"

	// DeploymentProwBuild the name of the Deployment for the Prow webhook engine
	DeploymentProwBuild = "prow-build"

	DefaultEnvironmentGitRepoURL = "https://github.com/jenkins-x/default-environment-charts.git"

	DefaultOrganisationGitRepoURL = "https://github.com/jenkins-x/default-organisation.git"

	// AnnotationTitle the human readable name of a resource which can include mixed case, spaces and punctuation
	AnnotationTitle = "title"

	// AnnotationDescription the tooltip / texual description of an resource
	AnnotationDescription = "description"

	// LabelGitSync to indicate whether or not to sync this resource to GitOps
	LabelGitSync = "jenkins.io/gitSync"

	// LabelKind to indicate the kind of auth, such as Git or Issue
	LabelKind = "jenkins.io/kind"

	// ValueKindAddon an addon auth secret/credentials
	ValueKindAddon = "addon"

	// ValueKindChat a chat auth secret/credentials
	ValueKindChat = "chat"

	// ValueKindCVE an CVS App secret/credentials
	ValueKindCVE = "cve"

	// ValueKindEnvironmentRole to indicate a Role which maps to an EnvironmentRoleBinding
	ValueKindEnvironmentRole = "EnvironmentRole"

	// ValueKindGit a git auth secret/credentials
	ValueKindGit = "git"

	// ValueKindIssue an issue auth secret/credentials
	ValueKindIssue = "issue"

	// ValueKindChartmuseum a chartmuseum auth secret/credentials
	ValueKindChartmuseum = "chartmuseum"

	// ValueKindJenkins an Jenkins App secret/credentials
	ValueKindJenkins = "jenkins"

	// ValueKindCVE an addon auth PipelineEvent
	ValueKindPipelineEvent = "PipelineEvent"

	// ValueKindPodTemplate a PodTemplate in a ConfigMap
	ValueKindPodTemplate = "podTemplate"

	// ValueKindPodTemplateXML a PodTemplate XML in a ConfigMap
	ValueKindPodTemplateXML = "podTemplateXml"

	// ValueKindCVE an addon auth PipelineEvent
	ValueKindRelease = "Release"

	// ValueKindEditNamespace for edit namespace
	ValueKindEditNamespace = "editspace"

	// LabelServiceKind the label to indicate the auto Server's Kind
	LabelServiceKind = "jenkins.io/service-kind"

	// LabelGithubAppOwner the label to indicate the owner of a repository for github app token secrets
	LabelGithubAppOwner = "jenkins.io/githubapp-owner"

	// LabelCreatedBy indicates the service that created this resource
	LabelCreatedBy = "jenkins.io/created-by"

	// LabelPodTemplate the name of the pod template for a DevPod
	LabelPodTemplate = "jenkins.io/pod_template"

	// LabelDevPodName the name of a dev pod
	LabelDevPodName = "jenkins.io/devpod"

	// LabelDevPodUsername the user name owner of the DeVPod
	LabelDevPodUsername = "jenkins.io/devpod_user"

	// LabelDevPodGitPrefix used to label a devpod with the repository host, owner, repo
	LabelDevPodGitPrefix = "jenkins.io/repo"

	// LabelUsername the user name owner of a namespace or resource
	LabelUsername = "jenkins.io/user"

	// ValueCreatedByJX for resources created by the Jenkins X CLI
	ValueCreatedByJX = "jx"

	// LabelCredentialsType the kind of jenkins credential for a secret
	LabelCredentialsType = "jenkins.io/credentials-type"

	// ValueCredentialTypeUsernamePassword for user password credential secrets
	ValueCredentialTypeUsernamePassword = "usernamePassword"

	// ValueCredentialTypeSecretFile for secret files
	ValueCredentialTypeSecretFile = "secretFile"

	// LabelTeam indicates the team name an environment belongs to
	LabelTeam = "team"

	// LabelEnvironment indicates the name of the environment
	LabelEnvironment = "env"

	// LabelValueDevEnvironment is the value of the LabelTeam label for Development environments (system namespace)
	LabelValueDevEnvironment = "dev"

	// LabelValueThisEnvironment is the value of the LabelTeam label for the current environment in remote clusters
	LabelValueThisEnvironment = "this"

	// LabelJobKind the kind of job
	LabelJobKind = "jenkins.io/job-kind"

	// ValueJobKindPostPreview
	ValueJobKindPostPreview = "post-preview-step"

	// AnnotationURL indicates a service/server's URL
	AnnotationURL = "jenkins.io/url"

	// AnnotationExpose used to expose service using exposecontroller
	AnnotationExpose = "fabric8.io/expose"

	// AnnotationIngress tells exposecontroller to annotate generated ingress rule with values
	AnnotationIngress = "fabric8.io/ingress.annotations"

	// AnnotationExposePort indicates to the exposecontroller which service port to expose
	//in case a service has multiple prots
	AnnotationExposePort = "fabric8.io/exposePort"

	// AnnotationName indicates a service/server's textual name (can be mixed case, contain spaces unlike Kubernetes resources)
	AnnotationName = "jenkins.io/name"

	// AnnotationCredentialsDescription the description text for a Credential on a Secret
	AnnotationCredentialsDescription = "jenkins.io/credentials-description"

	// AnnotationWorkingDir the working directory, such as for a DevPod
	AnnotationWorkingDir = "jenkins.io/working-dir"
	// AnnotationLocalDir the local directory that is sync'd to the DevPod
	AnnotationLocalDir = "jenkins.io/local-dir"
	// AnnotationGitURLs the newline separated list of git URLs of the DevPods
	AnnotationGitURLs = "jenkins.io/git-urls"
	// AnnotationGitReportState used to annotate what state has been reported to git
	AnnotationGitReportState = "jenkins.io/git-report-state"
	// AnnotationGitReportRunningStages used to annotate what stages were last reported to git as running
	AnnotationGitReportRunningStages = "jenkins.io/git-report-running-stages"

	// AnnotationIsDefaultStorageClass used to indicate a storageclass is default
	AnnotationIsDefaultStorageClass = "storageclass.kubernetes.io/is-default-class"

	// AnnotationReleaseName is the name of the annotation that stores the release name in the preview environment
	AnnotationReleaseName = "jenkins.io/chart-release"

	// SecretDataUsername the username in a Secret/Credentials
	SecretDataUsername = "username"

	// SecretDataPassword the password in a Secret/Credentials
	SecretDataPassword = "password"

	// SecretBasicAuth the name for the Jenkins X basic auth secret
	SecretBasicAuth = "jx-basic-auth" // #nosec

	// JenkinsAdminApiToken the API token
	JenkinsAdminApiToken = "jenkins-admin-api-token"

	// JenkinsAdminUserField the admin user name
	JenkinsAdminUserField = "jenkins-admin-user"

	// JenkinsAdminPasswordField the password field
	JenkinsAdminPasswordField = "jenkins-admin-password"

	// JenkinsBearTokenField the bearer token
	JenkinsBearTokenField = "jenkins-bearer-token"

	AUTH = "auth"

	// Region stores the cloud region the cluster is installed on
	Region = "region"

	// Zone stores the cloud zone of the install
	Zone = "zone"

	// ProjectID stores the project ID used to install the cluster (a GKE thing mostly)
	ProjectID = "projectID"

	// ClusterName stores the name of the cluster that is created
	ClusterName = "clusterName"

	// KubeProvider stores the kubernetes provider name
	KubeProvider = "kubeProvider"

	// SystemVaultName stores the name of the system Vault created on install
	SystemVaultName = "systemVaultName"
)

var (
	AddonCharts = map[string]string{
		"ambassador":                    ChartAmbassador,
		"anchore":                       ChartAnchore,
		DefaultFlaggerReleaseName:       ChartFlagger,
		"gitea":                         ChartGitea,
		"istio":                         ChartIstio,
		"kubeless":                      ChartKubeless,
		"prometheus":                    "stable/prometheus",
		"grafana":                       "stable/grafana",
		"jx-build-templates":            "jenkins-x/jx-build-templates",
		DefaultProwReleaseName:          ChartProw,
		DefaultKnativeBuildReleaseName:  ChartKnativeBuild,
		DefaultTektonReleaseName:        ChartTekton,
		DefaultSsoDexReleaseName:        ChartSsoDex,
		DefaultSsoOperatorReleaseName:   ChartSsoOperator,
		DefaultVaultOperatorReleaseName: ChartVaultOperator,
	}

	AddonServices = map[string]string{
		"anchore":         "anchore-anchore-engine",
		"pipeline-events": "jx-pipeline-events-elasticsearch-client",
		"grafana":         "grafana",
	}
)
