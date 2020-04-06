package restore

import (
	"fmt"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube/velero"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/client-go/kubernetes"
)

// FromBackupOptions contains the command line options
type FromBackupOptions struct {
	*StepRestoreOptions

	Namespace       string
	UseLatestBackup bool
}

var (
	restoreFromBackupLong = templates.LongDesc(`
		Restores the cluster custom data from the a backup.

`)

	restoreFromBackupExample = templates.Examples(`
		# executes the step which restores data from a backup 
		jx step restore from-backup
	`)
)

// NewCmdStepRestoreFromBackup creates the command
func NewCmdStepRestoreFromBackup(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &FromBackupOptions{
		StepRestoreOptions: &StepRestoreOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "from-backup [flags]",
		Short:   "This step attempts a velero restore from a selected velero backup",
		Long:    restoreFromBackupLong,
		Example: restoreFromBackupExample,
		Aliases: []string{"from-backups"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "velero", "The namespace where velero has been installed")
	cmd.Flags().BoolVarP(&options.UseLatestBackup, "latest", "", false, "This indicates whether to use the latest velero backup as the restore point")
	return cmd
}

func performVeleroRestore(apiClient apiextensionsclientset.Interface, kubeClient kubernetes.Interface, backupName string, namespace string) error {
	log.Logger().Infof("Using backup '%s' as the backup to restore", util.ColorInfo(backupName))
	err := velero.RestoreFromBackup(apiClient, kubeClient, namespace, backupName)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("when attempting to restore from '%s' backup", backupName))
	}
	return nil
}

// Run implements this command
func (o *FromBackupOptions) Run() error {

	// create the api extensions client
	apiClient, err := o.ApiExtensionsClient()
	if err != nil {
		return errors.Wrap(err, "while creating api extensions client")
	}

	// create the kubernetes client
	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "while creating kube client")
	}

	// check if a velero schedule exists
	scheduleExists, err := velero.DoesVeleroBackupScheduleExist(apiClient, o.Namespace)
	if err != nil {
		errors.Wrap(err, "when trying to check for velero schedules")
	}

	// However, if a Velero Schedule exists then we should be confident that this is an existing operational cluster
	// and therefore abort the restore.
	if scheduleExists {
		fmt.Println("A velero schedule exists for this cluster")
		fmt.Println("Aborting restore as it would be dangerous to apply the a backup")
		fmt.Println("If you expected this command to execute automatically - perhaps the backup schedule apply step comes before this step?")

		return nil
	}
	latestBackupName, err := velero.GetLatestBackupFromBackupResource(apiClient, o.Namespace)
	if err != nil {
		errors.Wrap(err, "when trying to get the latest backup")
	}
	if o.UseLatestBackup {
		if latestBackupName == "" {
			fmt.Println("unable to locate latest backup - it's possible there may not be any yet")
			return nil
		}
		err = performVeleroRestore(apiClient, kubeClient, latestBackupName, o.Namespace)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("when attempting to automatically restore from '%s' backup", latestBackupName))
		}
	} else {
		backupNames, err := velero.GetBackupsFromBackupResource(apiClient, o.Namespace)
		if err != nil {
			return errors.Wrap(err, "when attempting to retrieve the backups")
		}
		if len(backupNames) == 0 {
			return errors.Errorf("unable to locate backups")
		}

		selectedBackup, err := util.PickNameWithDefault(backupNames, "Which backup do you want to restore from?: ", latestBackupName, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		args := []string{selectedBackup}
		if len(args) == 0 {
			return fmt.Errorf("No backup chosen")
		}
		selectedBackupName := args[0]
		err = performVeleroRestore(apiClient, kubeClient, selectedBackupName, o.Namespace)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("when attempting to automatically restore from '%s' backup", selectedBackupName))
		}
	}
	return nil
}
