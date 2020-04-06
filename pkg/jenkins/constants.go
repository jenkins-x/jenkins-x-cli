package jenkins

const (
	// DefaultJenkinsCredentialsPrefix prefix for jenkins credentials
	DefaultJenkinsCredentialsPrefix = "jenkins-x-"

	// Chartmuseum name for chartmuseum
	Chartmuseum = "chartmuseum"

	// BranchPatternMasterPRsAndFeatures only match master, PRs and features
	BranchPatternMasterPRsAndFeatures = "master|PR-.*|feature.*"

	// BranchPatternMatchEverything matches everything
	BranchPatternMatchEverything = ".*"
)
