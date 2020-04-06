package boot

const (
	// PullRequestLabel is the label used on pull requests created by boot
	PullRequestLabel = "jx/boot"
	// OverrideTLSWarningEnvVarName is an environment variable set in BDD tests to override the error (in batch mode)
	// that is created if TLS is not enabled
	OverrideTLSWarningEnvVarName = "TESTING_ONLY_OVERRIDE_TLS_WARNING"
	// ConfigBaseRefEnvVarName is the env var name used in the pipeline to reference the base used for the config
	ConfigBaseRefEnvVarName = "CONFIG_BASE_REF"
	// ConfigRepoURLEnvVarName is the env var name used in the pipeline to reference the URL of the config
	ConfigRepoURLEnvVarName = "CONFIG_REPO_URL"
	// VersionsRepoURLEnvVarName is the env var name used in the pipeline to reference the URL of versions repo
	VersionsRepoURLEnvVarName = "VERSIONS_REPO_URL"
	// VersionsRepoBaseRefEnvVarName is the env var name used in the pipeline to reference the ref of versions repo
	VersionsRepoBaseRefEnvVarName = "VERSIONS_BASE_REF"
	// DisablePushUpdatesToDevEnvironment is the env var name used to disable pushing changes to the local git clone
	// of the dev environment to the master branch of the git repository in the `jx step verify env` command
	// e.g. to test out local changes to git in a local fork of the boot config with `jx boot --no-update-git`
	DisablePushUpdatesToDevEnvironment = "JX_NO_DEV_GIT_UPDATES"
)
