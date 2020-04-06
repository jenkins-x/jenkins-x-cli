package opts

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	survey "gopkg.in/AlecAivazis/survey.v1"
)

// GkeClusterListHeader header name for GCP project ID when listing the GKE clusters
const GkeClusterListHeader = "PROJECT_ID"

// GetGoogleProjectID asks to chose from existing projects or optionally creates one if none exist
func (o *CommonOptions) GetGoogleProjectID(defaultProject string) (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	out, err := o.GetCommandOutput("", "gcloud", "projects", "list")
	if err != nil {
		return "", err
	}

	lines := strings.Split(out, "\n")
	var existingProjects []string
	for _, l := range lines {
		if strings.Contains(l, GkeClusterListHeader) {
			continue
		}
		fields := strings.Fields(l)
		existingProjects = append(existingProjects, fields[0])
	}

	var projectId string
	if len(existingProjects) == 0 {
		confirm := &survey.Confirm{
			Message: fmt.Sprintf("No existing Google Projects exist, create one now?"),
			Default: true,
		}
		flag := true
		err = survey.AskOne(confirm, &flag, nil, surveyOpts)
		if err != nil {
			return "", err
		}
		if !flag {
			return "", errors.New("no google project to create cluster in, please manual create one and rerun this wizard")
		}

		if flag {
			return "", errors.New("auto creating projects not yet implemented, please manually create one and rerun the wizard")
		}
	} else if len(existingProjects) == 1 {
		projectId = existingProjects[0]
		log.Logger().Infof("Using the only Google Cloud Project %s to create the cluster", util.ColorInfo(projectId))
	} else {
		prompts := &survey.Select{
			Message: "Google Cloud Project:",
			Options: existingProjects,
			Help:    "Select a Google Project to create the cluster in",
		}

		if defaultProject != "" {
			prompts.Default = defaultProject
		} else if currentProject, err := gke.GetCurrentProject(); err == nil && currentProject != "" {
			prompts.Default = currentProject
		}

		err := survey.AskOne(prompts, &projectId, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}

	if projectId == "" {
		return "", errors.New("no Google Cloud Project to create cluster in, please manual create one and rerun this wizard")
	}

	return projectId, nil
}

// GetGoogleZone returns the GCP zone
func (o *CommonOptions) GetGoogleZone(projectID string, defaultZone string) (string, error) {
	if defaultZone == "" {
		var err error
		defaultZone, err = o.GetCommandOutput("", "gcloud", "config", "get-value", "compute/zone")
		if err != nil {
			return "", errors.Wrap(err, "getting google zone")
		}
	}

	return o.GetGoogleZoneWithDefault(projectID, defaultZone)
}

// GetGoogleRegion returns the GCP region
func (o *CommonOptions) GetGoogleRegion(projectId string) (string, error) {
	configuredRegion, err := o.GetCommandOutput("", "gcloud", "config", "get-value", "compute/region")
	if err != nil {
		return "", errors.Wrap(err, "getting google region")
	}
	return o.GetGoogleRegionWithDefault(projectId, configuredRegion)
}

// GetGoogleZoneWithDefault returns the GCP zone, if not zone is found returns the default zone
func (o *CommonOptions) GetGoogleZoneWithDefault(projectId string, defaultZone string) (string, error) {
	availableZones, err := gke.GetGoogleZones(projectId)
	if err != nil {
		return "", err
	}
	prompts := &survey.Select{
		Message:  "Google Cloud Zone:",
		Options:  availableZones,
		PageSize: 10,
		Help:     "The compute zone (e.g. us-central1-a) for the cluster",
		Default:  defaultZone,
	}
	zone := ""
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	err = survey.AskOne(prompts, &zone, nil, surveyOpts)
	if err != nil {
		return "", err
	}
	return zone, nil
}

func (o *CommonOptions) GetGoogleRegionWithDefault(projectId string, defaultRegion string) (string, error) {
	availableRegions, err := gke.GetGoogleRegions(projectId)
	if err != nil {
		return "", err
	}
	prompts := &survey.Select{
		Message:  "Google Cloud Region:",
		Options:  availableRegions,
		PageSize: 10,
		Help:     "The compute region (e.g. us-central1) for the cluster",
		Default:  defaultRegion,
	}
	region := ""
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	err = survey.AskOne(prompts, &region, nil, surveyOpts)
	if err != nil {
		return "", err
	}
	return region, nil
}

// GetGKEClusterNameFromContext returns the GKE cluster name from current Kubernetes context
func (o *CommonOptions) GetGKEClusterNameFromContext() (string, error) {
	return cluster.ShortName(o.kuber)
}
