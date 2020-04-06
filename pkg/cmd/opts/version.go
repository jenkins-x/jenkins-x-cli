package opts

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/versionstream/versionstreamrepo"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/table"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/version"
)

// CreateVersionResolver creates a new VersionResolver service
func (o *CommonOptions) CreateVersionResolver(repo string, gitRef string) (*versionstream.VersionResolver, error) {
	versionsDir, _, err := o.CloneJXVersionsRepo(repo, gitRef)
	if err != nil {
		return nil, err
	}
	return &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}, nil
}

// GetVersionResolver gets a VersionResolver, lazy creating one if required so we can reuse it later
func (o *CommonOptions) GetVersionResolver() (*versionstream.VersionResolver, error) {
	var err error
	if o.versionResolver == nil {
		o.versionResolver, err = o.CreateVersionResolver("", "")
	}
	return o.versionResolver, err
}

// SetVersionResolver gets a VersionResolver, lazy creating one if required
func (o *CommonOptions) SetVersionResolver(resolver *versionstream.VersionResolver) {
	o.versionResolver = resolver
}

// GetPackageVersions returns the package versions and a table if they need to be rendered
func (o *CommonOptions) GetPackageVersions(ns string, helmTLS bool) (map[string]string, table.Table) {
	info := util.ColorInfo
	packages := map[string]string{}
	table := o.CreateTable()
	table.AddRow("NAME", "VERSION")
	jxVersion := version.GetVersion()
	table.AddRow("jx", info(jxVersion))
	packages["jx"] = jxVersion
	// Jenkins X version
	releases, _, err := o.Helm().ListReleases(ns)
	if err != nil {
		log.Logger().Warnf("Failed to find helm installs: %s", err)
	} else {
		for _, release := range releases {
			if release.Chart == "jenkins-x-platform" {
				table.AddRow("jenkins x platform", info(release.ChartVersion))
			}
		}
	}
	// Kubernetes version
	client, err := o.KubeClient()
	if err != nil {
		log.Logger().Warnf("Failed to connect to Kubernetes: %s", err)
	} else {
		serverVersion, err := client.Discovery().ServerVersion()
		if err != nil {
			log.Logger().Warnf("Failed to get Kubernetes server version: %s", err)
		} else if serverVersion != nil {
			version := serverVersion.String()
			packages["kubernetesCluster"] = version
			table.AddRow("Kubernetes cluster", info(version))
		}
	}
	// kubectl version
	output, err := o.GetCommandOutput("", "kubectl", "version", "--short")
	if err != nil {
		log.Logger().Warnf("Failed to get kubectl version: %s", err)
	} else {
		for i, line := range strings.Split(output, "\n") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				v := fields[2]
				if v != "" {
					switch i {
					case 0:
						table.AddRow("kubectl", info(v))
						packages["kubectl"] = v
					case 1:
						// Ignore K8S server details as we have these above
					}
				}
			}
		}
	}

	// helm version
	output, err = o.Helm().Version(helmTLS)
	if err != nil {
		log.Logger().Warnf("Failed to get helm version: %s", err)
	} else {
		helmBinary, noTiller, helmTemplate, _ := o.TeamHelmBin()
		if helmBinary == "helm3" || noTiller || helmTemplate {
			table.AddRow("helm client", info(output))
		} else {
			for i, line := range strings.Split(output, "\n") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					v := fields[1]
					if v != "" {
						switch i {
						case 0:
							table.AddRow("helm client", info(v))
							packages["helm"] = v
						case 1:
							table.AddRow("helm server", info(v))
						}
					}
				}
			}
		}
	}

	// git version
	version, err := o.Git().Version()
	if err != nil {
		log.Logger().Warnf("Failed to get git version: %s", err)
	} else {
		table.AddRow("git", info(version))
		packages["git"] = version
	}
	return packages, table
}

// GetVersionNumber returns the version number for the given kind and name or blank string if there is no locked version
func (o *CommonOptions) GetVersionNumber(kind versionstream.VersionKind, name, repo string, gitRef string) (string, error) {
	versioner, err := o.CreateVersionResolver(repo, gitRef)
	if err != nil {
		return "", err
	}
	return versioner.StableVersionNumber(kind, name)
}

// CloneJXVersionsRepo clones the jenkins-x versions repo to a local working dir
func (o *CommonOptions) CloneJXVersionsRepo(versionRepository string, versionRef string) (string, string, error) {
	settings, err := o.TeamSettings()
	if err != nil {
		log.Logger().Debugf("Unable to load team settings because %v", err)
	}
	return versionstreamrepo.CloneJXVersionsRepo(versionRepository, versionRef, settings, o.Git(), o.BatchMode, o.AdvancedMode, o.GetIOFileHandles())
}
