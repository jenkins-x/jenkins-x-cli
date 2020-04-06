package kube

import (
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/pkg/errors"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const adminNamespaceAnnotation = "jenkins-x.io/admin-namespace"

// GetAdminNamespace tries to find the admin namespace that corresponds to this team.
// in other words this is the namespace where the team CRD was initially created when this team was created,
// or the current team namespace for the case where this team was just created with a standalone `jx install`
func GetAdminNamespace(kubeClient kubernetes.Interface, teamNs string) (string, error) {
	namespace, err := kubeClient.CoreV1().Namespaces().Get(teamNs, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "obtaining the team namespace (%s) when getting the admin namespace", teamNs)
	}
	adminNs := namespace.Annotations[adminNamespaceAnnotation]
	if adminNs != "" {
		return adminNs, nil
	}
	return teamNs, nil
}

// SetAdminNamespace annotates the given namespace with a backlink to the admin namespace.
// it does not make any changes if the current annotation points to the same admin namespace
func SetAdminNamespace(kubeClient kubernetes.Interface, teamNs string, adminNs string) error {
	namespace, err := kubeClient.CoreV1().Namespaces().Get(teamNs, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "obtaining the namespace (%s) when updating the admin namespace", teamNs)
	}
	oldAdminNs := namespace.Annotations[adminNamespaceAnnotation]
	if oldAdminNs == adminNs {
		// nothing to do
		return nil
	}
	namespace.Annotations[adminNamespaceAnnotation] = adminNs
	// TODO use patch
	_, err = kubeClient.CoreV1().Namespaces().Update(namespace)
	return err
}

// GetPendingTeams returns the pending teams with the sorted order of names
func GetPendingTeams(jxClient versioned.Interface, ns string) (map[string]*v1.Team, []string, error) {
	m := map[string]*v1.Team{}

	names := []string{}
	teamList, err := jxClient.JenkinsV1().Teams(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, names, err
	}
	for _, team := range teamList.Items {
		n := team.Name
		copy := team
		m[n] = &copy
		if n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return m, names, nil
}

// CreateTeam creates a new default Team
func CreateTeam(ns string, name string, members []string) *v1.Team {
	kind := v1.TeamKindTypeCD
	team := &v1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.ToValidName(name),
			Namespace: ns,
		},
		Spec: v1.TeamSpec{
			Label:   strings.Title(name),
			Members: members,
			Kind:    kind,
		},
	}
	return team
}

// DeleteTeam deletes the team resource but does not uninstall the underlying namespaces
func DeleteTeam(jxClient versioned.Interface, ns string, teamName string) error {
	teamInterface := jxClient.JenkinsV1().Teams(ns)
	_, err := teamInterface.Get(teamName, metav1.GetOptions{})
	if err == nil {
		err = teamInterface.Delete(teamName, nil)
	}
	return err
}
