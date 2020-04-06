package gke

import (
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/util"
)

var PROJECT_LIST_HEADER = "PROJECT_ID"

func GetGoogleZones(project string) ([]string, error) {
	var zones []string
	args := []string{"compute", "zones", "list"}

	if "" != project {
		args = append(args, "--project")
		args = append(args, project)
	}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}

	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return nil, err
	}

	for _, item := range strings.Split(out, "\n") {
		zone := strings.Split(item, " ")[0]
		if strings.Contains(zone, "-") {
			zones = append(zones, zone)
		}
		sort.Strings(zones)
	}
	return zones, nil
}

func GetGoogleRegions(project string) ([]string, error) {
	var regions []string
	args := []string{"compute", "regions", "list"}

	if "" != project {
		args = append(args, "--project")
		args = append(args, project)
	}

	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}

	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return nil, err
	}

	regions = append(regions, "none")
	for _, item := range strings.Split(out, "\n") {
		region := strings.Split(item, " ")[0]
		if strings.Contains(region, "-") {
			regions = append(regions, region)
		}
		sort.Strings(regions)
	}
	return regions, nil
}

func GetGoogleProjects() ([]string, error) {
	cmd := util.Command{
		Name: "gcloud",
		Args: []string{"projects", "list"},
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return nil, err
	}

	if out == "Listed 0 items." {
		return []string{}, nil
	}

	lines := strings.Split(out, "\n")
	var existingProjects []string
	for _, l := range lines {
		if strings.Contains(l, PROJECT_LIST_HEADER) {
			continue
		}
		fields := strings.Fields(l)
		existingProjects = append(existingProjects, fields[0])
	}
	return existingProjects, nil
}

func GetCurrentProject() (string, error) {
	cmd := util.Command{
		Name: "gcloud",
		Args: []string{"config", "get-value", "project"},
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return "", err
	}

	index := strings.LastIndex(out, "\n")
	if index >= 0 {
		return out[index+1:], nil
	}

	return out, nil
}

func GetGoogleMachineTypes() []string {

	return []string{
		"g1-small",
		"n1-standard-1",
		"n1-standard-2",
		"n1-standard-4",
		"n1-standard-8",
		"n1-standard-16",
		"n1-standard-32",
		"n1-standard-64",
		"n1-standard-96",
		"n1-highmem-2",
		"n1-highmem-4",
		"n1-highmem-8",
		"n1-highmem-16",
		"n1-highmem-32",
		"n1-highmem-64",
		"n1-highmem-96",
		"n1-highcpu-2",
		"n1-highcpu-4",
		"n1-highcpu-8",
		"n1-highcpu-16",
		"n1-highcpu-32",
		"n1-highcpu-64",
		"n1-highcpu-96",
	}
}

// ParseContext parses the context string for GKE and gets the GKE project, GKE zone and cluster name
func ParseContext(context string) (string, string, string, error) {
	parts := strings.Split(context, "_")
	if len(parts) != 4 {
		return "", "", "", errors.Errorf("unable to parse %s as <project id>_<zone>_<cluster name>", context)
	}
	return parts[1], parts[2], parts[3], nil
}
