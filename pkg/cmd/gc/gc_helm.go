package gc

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/ghodss/yaml"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
)

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type GCHelmOptions struct {
	*opts.CommonOptions

	RevisionHistoryLimit int
	OutDir               string
	DryRun               bool
	NoBackup             bool
}

var (
	GCHelmLong = templates.LongDesc(`
		Garbage collect Helm ConfigMaps.  To facilitate rollbacks, Helm leaves a history of chart versions in place in Kubernetes and these should be pruned at intervals to avoid consuming excessive system resources.

`)

	GCHelmExample = templates.Examples(`
		jx garbage collect helm
		jx gc helm
`)
)

// NewCmdGCHelm  a command object for the "garbage collect" command
func NewCmdGCHelm(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GCHelmOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "helm",
		Short:   "garbage collection for Helm ConfigMaps",
		Long:    GCHelmLong,
		Example: GCHelmExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().IntVarP(&options.RevisionHistoryLimit, "revision-history-limit", "", 10, "Minimum number of versions per release to keep")
	cmd.Flags().StringVarP(&options.OutDir, opts.OptionOutputDir, "o", "configmaps", "Relative directory to output backup to. Defaults to ./configmaps")
	cmd.Flags().BoolVarP(&options.DryRun, "dry-run", "", false, "Does not perform the delete operation on Kubernetes")
	cmd.Flags().BoolVarP(&options.NoBackup, "no-backup", "", false, "Does not perform the backup operation to store files locally")
	return cmd
}

func (o *GCHelmOptions) Run() error {
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	kubeNamespace := "kube-system"

	cms, err := kubeClient.CoreV1().ConfigMaps(kubeNamespace).List(metav1.ListOptions{LabelSelector: "OWNER=TILLER"})
	if err != nil {
		return err
	}
	if len(cms.Items) == 0 {
		// no configmaps found so lets return gracefully
		log.Logger().Debug("no config maps found")
		return nil
	}

	releases := ExtractReleases(cms)
	log.Logger().Debug(fmt.Sprintf("Found %d releases.", len(releases)))
	log.Logger().Debug(fmt.Sprintf("Releases: %v", releases))

	for _, release := range releases {
		log.Logger().Debug(fmt.Sprintf("Checking %s. ", release))
		versions := ExtractVersions(cms, release)
		log.Logger().Debug(fmt.Sprintf("Found %d.", len(versions)))
		log.Logger().Debug(fmt.Sprintf("%v", versions))
		to_delete := VersionsToDelete(versions, o.RevisionHistoryLimit)
		if len(to_delete) > 0 {
			if o.DryRun {
				log.Logger().Info("Would delete:")
				log.Logger().Infof("%v", to_delete)
			} else {
				// Backup and delete
				if o.NoBackup == false {
					// First make sure that destination path exists
					err3 := os.MkdirAll(o.OutDir, 0755)
					if err3 != nil {
						// Failed to create path
						return err3
					}
				}
				for _, version := range to_delete {
					cm, err1 := ExtractConfigMap(cms, version)
					if err1 == nil {
						if o.NoBackup == false {
							// Create backup for ConfigMap about to be deleted
							filename := o.OutDir + "/" + version + ".yaml"
							log.Logger().Info(fmt.Sprintf("Backing up %v. ", filename))
							y, err2 := yaml.Marshal(cm)
							if err2 != nil {
								// Failed to Marshall to YAML
								return err2
							}
							// Add apiVersion and Kind
							var b bytes.Buffer
							b.WriteString("apiVersion: v1\nkind: ConfigMap\n")
							b.Write(y)
							err4 := ioutil.WriteFile(filename, b.Bytes(), 0644)
							if err4 == nil {
								log.Logger().Info("Success. ")
							} else {
								// Failed to write backup so abort
								return err4
							}
						}
						// Now delete
						var opts *metav1.DeleteOptions
						err5 := kubeClient.CoreV1().ConfigMaps(kubeNamespace).Delete(version, opts)
						if err5 == nil {
							log.Logger().Info(fmt.Sprintf("ConfigMap %v deleted.", version))
						} else {
							// Failed to delete
							return err5
						}
					} else {
						// Failed to find a ConfigMap that we know was in memory. Unlikely to occur.
						log.Logger().Warn(fmt.Sprintf("Failed to find ConfigMap %s. ", version))
					}
				}
			}
		} else {
			log.Logger().Debug("Nothing to do.")
		}
	}
	return nil
}

// ExtractReleases Extract a set of releases from a list of ConfigMaps
func ExtractReleases(cms *v1.ConfigMapList) []string {
	found := make(map[string]bool)
	for _, cm := range cms.Items {
		if cmname, ok := cm.Labels["NAME"]; ok {
			// Collect unique names
			if _, seen := found[cmname]; !seen {
				found[cmname] = true
			}
		}
	}

	// Return a set of unique Helm releases
	releases := []string{}
	for key := range found {
		releases = append(releases, key)
	}
	return releases
}

// ExtractVersions Extract a set of versions of a named release from a list of ConfigMaps
func ExtractVersions(cms *v1.ConfigMapList, release string) []string {
	found := []string{}
	for _, cm := range cms.Items {
		if release == cm.Labels["NAME"] {
			found = append(found, cm.Name)
		}
	}
	return found
}

// VersionsToDelete returns a slice of strings
func VersionsToDelete(versions []string, desired int) []string {
	if desired >= len(versions) {
		// nothing to delete
		return []string{}
	}
	sort.Sort(ByVersion(versions))
	return versions[:len(versions)-desired]
}

// ExtractConfigMap extracts a configmap
func ExtractConfigMap(cms *v1.ConfigMapList, version string) (v1.ConfigMap, error) {
	for _, cm := range cms.Items {
		if version == cm.Name {
			return cm, nil
		}
	}
	return v1.ConfigMap{}, errors.New("Not found")
}

// Components for sorting versions by numeric version number where version name ends in .vddd where ddd is an arbitrary sequence of digits
type ByVersion []string

func (a ByVersion) Len() int      { return len(a) }
func (a ByVersion) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool {
	r, _ := regexp.Compile(`.v\d*$`)
	loc := r.FindStringIndex(a[i])
	if loc == nil {
		return false
	}
	trim := loc[0] + 2 // start of numeric
	version_number_i, err_i := strconv.Atoi(a[i][trim:])
	version_number_j, err_j := strconv.Atoi(a[j][trim:])
	if (err_i == nil) && (err_j == nil) {
		return version_number_i < version_number_j
	}

	return false
}
