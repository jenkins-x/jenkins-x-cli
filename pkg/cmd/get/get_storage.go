package get

import (
	"sort"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/step"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// GetStorageOptions contains the CLI options
type GetStorageOptions struct {
	GetOptions
}

var (
	getStorageLong = templates.LongDesc(`
		Display the storage configuration for different classifications.
` + step.StorageSupportDescription + helper.SeeAlsoText("jx step stash", "jx edit storage"))

	getStorageExample = templates.Examples(`
		# List the storage configurations for different classifications for the current team
		jx get storage
	`)
)

// NewCmdGetStorage creates the new command for: jx get env
func NewCmdGetStorage(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetStorageOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "storage",
		Short:   "Display the storage configuration for different classifications",
		Long:    getStorageLong,
		Example: getStorageExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetStorageOptions) Run() error {
	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}
	m := map[string]v1.StorageLocation{}
	for i, ls := range settings.StorageLocations {
		m[ls.Classifier] = settings.StorageLocations[i]
	}
	for _, name := range kube.Classifications {
		if _, ok := m[name]; !ok {
			m[name] = settings.StorageLocation(name)
		}
	}
	names := []string{}
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	table := o.CreateTable()
	table.AddRow("CLASSIFICATION", "LOCATION")
	for _, n := range names {
		ls, ok := m[n]
		if ok {
			table.AddRow(n, ls.Description())
		}
	}
	table.Render()
	return nil
}
