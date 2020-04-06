package config

type Kind string

const (
	// Application adds an application
	Application Kind = "APPLICATION"

	// Environment a local environment
	Environment Kind = "ENVIRONMENT"

	// RemoteEnvironment a remote environment
	RemoteEnvironment Kind = "REMOTE_ENVIRONMENT"

	// Protection for the protection kind
	Protection Kind = "PROTECTION"

	// ServerlessJenkins serverless jenkins
	ServerlessJenkins = "serverless-jenkins"

	// PromotionBuild for a promotion build
	PromotionBuild = "promotion-build"
)
