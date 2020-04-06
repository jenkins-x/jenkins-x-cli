package features

import (
	"errors"
	"strings"

	"reflect"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/rollout/rox-go/core/context"
	"github.com/rollout/rox-go/server"
	"github.com/spf13/cobra"
)

// API key, populated at build-time.
var FeatureFlagToken string

//flag to indicate if it's the oss version
var oss bool

// Features Flags
type Features struct {

	// types of Jenkins X installations
	Tekton        server.RoxFlag
	StaticJenkins server.RoxFlag

	// Supported Cloud Providers
	AKS        server.RoxFlag
	AWS        server.RoxFlag
	EKS        server.RoxFlag
	GKE        server.RoxFlag
	ICP        server.RoxFlag
	IKS        server.RoxFlag
	OKE        server.RoxFlag
	Kubernetes server.RoxFlag
	Minikube   server.RoxFlag
	Openshift  server.RoxFlag

	// Supported build packs
	Java server.RoxFlag
	Go   server.RoxFlag
	Node server.RoxFlag
}

var features = &Features{
	Tekton:        server.NewRoxFlag(false),
	StaticJenkins: server.NewRoxFlag(false),
	AKS:           server.NewRoxFlag(false),
	AWS:           server.NewRoxFlag(false),
	EKS:           server.NewRoxFlag(false),
	GKE:           server.NewRoxFlag(false),
	ICP:           server.NewRoxFlag(false),
	IKS:           server.NewRoxFlag(false),
	OKE:           server.NewRoxFlag(false),
	Kubernetes:    server.NewRoxFlag(false),
	Minikube:      server.NewRoxFlag(false),
	Openshift:     server.NewRoxFlag(false),
	Java:          server.NewRoxFlag(false),
	Go:            server.NewRoxFlag(false),
	Node:          server.NewRoxFlag(false),
}

var rox *server.Rox

var ctx context.Context

// SetFeatureFlagToken - used to set the API key in the tests
// todo remove this I have a better idea
func SetFeatureFlagToken(token string) {
	FeatureFlagToken = token
}

// IsFeatureEnabled - determines if the feature flag mechanism is enabled
func IsFeatureEnabled() bool {
	return FeatureFlagToken != "oss" && FeatureFlagToken != ""
}

// Init - initialise the feature flag mechanism
func Init() {
	if IsFeatureEnabled() {
		log.Logger().Infof("Cloudbees Jenkins X distribution - only supported features enabled")
		oss = false
		// todo probably want the cloudbees login here
		ctx = context.NewContext(map[string]interface{}{"user": "N/A"})
		rox = server.NewRox()
		rox.Register("jx", features)

		roxOptions := server.NewRoxOptions(server.RoxOptionsBuilder{})
		<-rox.Setup(FeatureFlagToken, roxOptions)
	} else {
		log.Logger().Debugf("OSS version - all features enabled")
		oss = true
	}
}

// CheckTektonEnabled checks if tekton is enabled
func CheckTektonEnabled() error {
	if !oss {
		log.Logger().Debug("Checking if Tekton enabled")
		if !features.Tekton.IsEnabled(ctx) {
			return errors.New("tekton not supported in CloudBees Distribution of Jenkins X")
		}
		log.Logger().Debug("Tekton enabled")
	}
	return nil
}

// CheckStaticJenkins checks if static jenkins master is enabled
func CheckStaticJenkins() error {
	if !oss {
		log.Logger().Debug("Checking if static jenkins master enabled")
		if !features.StaticJenkins.IsEnabled(ctx) {
			return errors.New("static jenkins master not supported in CloudBees Distribution of Jenkins X")
		}
		log.Logger().Debug("Static Jenkins Master enabled")
	}
	return nil
}

func isProviderEnabled(provider string) bool {
	log.Logger().Debugf("Is Provider enabled for %s", provider)
	v := reflect.ValueOf(features).Elem()
	f := v.FieldByName(strings.ToUpper(provider))
	original, ok := f.Interface().(server.RoxFlag)
	enabled := false
	if ok {
		enabled = original.IsEnabled(ctx)
	}
	log.Logger().Debugf("Provider is enabled %t", enabled)
	return enabled
}

//Checks if a Cobra command has been enabled
func IsEnabled(cmd *cobra.Command) error {
	if !oss {
		parent := cmd.Parent()
		if parent.Name() == "cluster" {
			log.Logger().Debug("Checking if provider is enabled")
			provider := cmd.Name()
			log.Logger().Debugf("Provider %s", provider)
			enabled := isProviderEnabled(provider)
			if !enabled {
				return errors.New("command not supported in CloudBees Distribution of Jenkins X")
			}
		}
	}
	return nil
}
