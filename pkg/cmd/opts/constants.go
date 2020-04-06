package opts

const (
	MinimumMavenDeployVersion = "2.8.2"

	MasterBranch         = "master"
	DefaultGitIgnoreFile = `
.project
.classpath
.idea
.cache
.DS_Store
*.im?
target
work
`

	// DefaultIngressNamesapce default namespace fro ingress controller
	DefaultIngressNamesapce = "kube-system"
	// DefaultIngressServiceName default name for ingress controller service and deployment
	DefaultIngressServiceName = "jxing-nginx-ingress-controller"

	// DeployKindKnative for knative serve based deployments
	DeployKindKnative = "knative"

	// DeployKindDefault for default kubernetes Deployment + Service deployment kinds
	DeployKindDefault = "default"

	// OptionKind to specify the kind of something (such as the kind of a deployment)
	OptionKind = "kind"

	// OptionCanary should we enable canary rollouts (progressive delivery)
	OptionCanary = "canary"

	// OptionHPA should we enable horizontal pod autoscaler for deployments
	OptionHPA = "hpa"
)
