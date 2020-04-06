package util

const (
	// PlaceHolderPrefix is prefix for placeholders
	PlaceHolderPrefix = "REPLACE_ME"

	// PlaceHolderAppName placeholder for app name
	PlaceHolderAppName = PlaceHolderPrefix + "_APP_NAME"

	// PlaceHolderGitProvider placeholder for git provider
	PlaceHolderGitProvider = PlaceHolderPrefix + "_GIT_PROVIDER"

	// PlaceHolderOrg placeholder for org
	PlaceHolderOrg = PlaceHolderPrefix + "_ORG"

	// PlaceHolderDockerRegistryOrg placeholder for docker registry
	PlaceHolderDockerRegistryOrg = PlaceHolderPrefix + "_DOCKER_REGISTRY_ORG"

	// DefaultGitUserName default value to use for git "user.name"
	DefaultGitUserName = "jenkins-x-bot"

	// DefaultGitUserEmail default value to use for git "user.email"
	DefaultGitUserEmail = "jenkins-x@googlegroups.com"

	// EnvVarBranchName is the environment variable that will hold the name of the branch being built during pipelines
	EnvVarBranchName = "BRANCH_NAME"
)
