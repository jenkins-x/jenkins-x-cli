package velero

import (
	"encoding/json"
	"fmt"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type veleroSchedule struct {
	Metadata veleroScheduleMetadata `json:"metadata"`
}

type veleroScheduleMetadata struct {
	Name string `json:"name"`
}

type veleroScheduleList struct {
	Items []veleroSchedule `json:"items"`
}

type veleroBackupMetadata struct {
	Name string `json:"name"`
}

type veleroBackup struct {
	Metadata veleroBackupMetadata `json:"metadata"`
}

type veleroBackupList struct {
	Items []veleroBackup `json:"items"`
}

var (
	veleroBackupsResource   = "backups.velero.io"
	veleroSchedulesResource = "schedules.velero.io"
)

// RestoreFromBackup restores from a named velero backup
func RestoreFromBackup(apiClient apiextensionsclientset.Interface, kubeClient kubernetes.Interface, namespace string, backupName string) error {
	if backupName == "" {
		return errors.Errorf("")
	}
	log.Logger().Infof("Using backup '%s'", backupName)

	args := []string{"create", "restore", "--from-backup", backupName, "--namespace", namespace}
	cmd := util.Command{
		Name: "velero",
		Args: args,
	}

	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("executing '%s %v' command", cmd.Name, cmd.Args))
	}

	log.Logger().Infof(output)

	return nil
}

// DoesVeleroBackupScheduleExist checks whether a velero schedule exists
func DoesVeleroBackupScheduleExist(apiClient apiextensionsclientset.Interface, namespace string) (bool, error) {

	if doesVeleroSchedulesResourceExist(apiClient) {
		// kubectl get schedules.velero.io -n velero -o json
		args := []string{"get", veleroSchedulesResource, "-n", namespace, "-o", "json"}
		cmd := util.Command{
			Name: "kubectl",
			Args: args,
		}

		output, err := cmd.RunWithoutRetry()
		if err != nil {
			return false, errors.Wrap(err, fmt.Sprintf("executing kubectl get %s command", veleroSchedulesResource))
		}

		var veleroShedules veleroScheduleList
		err = json.Unmarshal([]byte(output), &veleroShedules)
		if err != nil {
			return false, errors.Wrap(err, "unmarshalling kubectl response")
		}

		if len(veleroShedules.Items) > 0 {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func doesVeleroBackupsResourceExist(apiClient apiextensionsclientset.Interface) bool {
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", veleroBackupsResource),
	}
	backupList, err := apiClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(listOptions)
	if err != nil {
		return false
	}

	if len(backupList.Items) > 0 {
		return true
	}
	return false
}

func doesVeleroSchedulesResourceExist(apiClient apiextensionsclientset.Interface) bool {
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", veleroSchedulesResource),
	}
	schedulesList, err := apiClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(listOptions)
	if err != nil {
		return false
	}

	if len(schedulesList.Items) > 0 {
		return true
	}
	return false
}

// GetBackupsFromBackupResource returns a list of all velero backups
func GetBackupsFromBackupResource(apiClient apiextensionsclientset.Interface, namespace string) ([]string, error) {

	if doesVeleroBackupsResourceExist(apiClient) {
		// kubectl get backups.velero.io -n velero -o json
		args := []string{"get", veleroBackupsResource, "-n", namespace, "-o", "json"}
		cmd := util.Command{
			Name: "kubectl",
			Args: args,
		}

		output, err := cmd.RunWithoutRetry()
		if err != nil {
			return []string{}, errors.Wrap(err, fmt.Sprintf("executing '%s %v' command", cmd.Name, cmd.Args))
		}

		var veleroBackups veleroBackupList
		err = json.Unmarshal([]byte(output), &veleroBackups)
		if err != nil {
			return []string{}, errors.Wrap(err, "unmarshalling kubectl response for backups")
		}

		if len(veleroBackups.Items) > 0 {
			backups := make([]string, len(veleroBackups.Items))
			// there must be a nicer way to do this
			for index, veleroBackup := range veleroBackups.Items {
				backups[index] = veleroBackup.Metadata.Name
			}
			return backups, nil
		}
		return []string{}, nil
	}
	return []string{}, nil
}

// GetLatestBackupFromBackupResource returns the latest velero backup name
func GetLatestBackupFromBackupResource(apiClient apiextensionsclientset.Interface, namespace string) (string, error) {

	if doesVeleroBackupsResourceExist(apiClient) {

		backups, err := GetBackupsFromBackupResource(apiClient, namespace)
		if err != nil {
			errors.Wrap(err, "when attempting to retrieve velero backup list")
		}

		return backups[len(backups)-1], nil
	}
	return "", nil
}
