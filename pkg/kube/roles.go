package kube

import (
	"fmt"
	"sort"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	clusterRoleBindingKind = "ClusterRoleBinding"
	clusterRoleKind        = "ClusterRole"
	apiGroup               = "rbac.authorization.k8s.io"
	subjectKind            = "ServiceAccount"
)

// GetTeamRoles returns the roles for the given team dev namespace
func GetTeamRoles(kubeClient kubernetes.Interface, ns string) (map[string]*rbacv1.Role, []string, error) {
	m := map[string]*rbacv1.Role{}

	names := []string{}
	resources, err := kubeClient.RbacV1().Roles(ns).List(metav1.ListOptions{
		LabelSelector: LabelKind + "=" + ValueKindEnvironmentRole,
	})
	if err != nil {
		return m, names, err
	}
	for _, env := range resources.Items {
		n := env.Name
		copy := env
		m[n] = &copy
		if n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return m, names, nil
}

// GetEnvironmentRoles returns all the environment role binding names and details
func GetEnvironmentRoles(jxClient versioned.Interface, ns string) (map[string]*v1.EnvironmentRoleBinding, []string, error) {
	names := []string{}
	m := map[string]*v1.EnvironmentRoleBinding{}
	envRoleBindingsList, err := jxClient.JenkinsV1().EnvironmentRoleBindings(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, names, fmt.Errorf("Failed to retrieve EnvironmentRoleBinding list for namespace %s: %s", ns, err)
	}
	for _, envRoleBinding := range envRoleBindingsList.Items {
		copy := envRoleBinding
		name := copy.Name
		m[name] = &copy
		names = append(names, name)
	}
	return m, names, err
}

// UpdateUserRoles updates the EnvironmentRoleBinding values based on the given userRoles
// userKind is "User" or "ServiceAccount"
func GetUserRoles(kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string, userKind string, userName string) ([]string, error) {
	envRoles, _, err := GetEnvironmentRoles(jxClient, ns)
	if err != nil {
		return nil, err
	}
	adminNs, err := GetAdminNamespace(kubeClient, ns)
	if err != nil {
		return nil, err
	}
	currentRoles := userRolesFor(userKind, userName, adminNs, envRoles)
	return currentRoles, err
}

// UpdateUserRoles updates the EnvironmentRoleBinding values based on the given userRoles.
// userKind is "User" or "ServiceAccount"
func UpdateUserRoles(kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string, userKind string, userName string, userRoles []string, roles map[string]*rbacv1.Role) error {

	envRoleInterface := jxClient.JenkinsV1().EnvironmentRoleBindings(ns)
	envRoles, _, err := GetEnvironmentRoles(jxClient, ns)

	adminNs, err := GetAdminNamespace(kubeClient, ns)
	if err != nil {
		return errors.Wrapf(err, "Obtaining admin namespace for user lookup")
	}

	// make sure all of the EnvironmentRoleBinding are created
	for name := range roles {
		envRole := envRoles[name]
		if envRole == nil {
			envRole = &v1.EnvironmentRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: ns,
					Labels: map[string]string{
						LabelKind: ValueKindEnvironmentRole,
					},
				},
				Spec: v1.EnvironmentRoleBindingSpec{
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     name,
						APIGroup: "rbac.authorization.k8s.io",
					},
					Subjects: []rbacv1.Subject{},
				},
			}
			_, err = envRoleInterface.Create(envRole)
			if err != nil {
				return errors.Wrapf(err, "Failed to create EnvironmentRoleBinding %s", name)
			}
			envRoles[name] = envRole
		}
	}

	oldRoles := userRolesFor(userKind, userName, adminNs, envRoles)

	deleteRoles, createRoles := util.DiffSlices(oldRoles, userRoles)

	// should we use a single patch or create at the end and be atomic for the whole operation?
	for _, name := range deleteRoles {
		envRole := envRoles[name]
		if envRole == nil {
			log.Logger().Warnf("Could not remove user %s kind %s from EnvironmentRoleBinding %s as it does not exist", userName, userKind, name)
		} else {
			found := false
			for idx, subject := range envRole.Spec.Subjects {
				if subject.Kind == userKind && subject.Name == userName && subject.Namespace == adminNs {
					found = true
					envRole.Spec.Subjects = append(envRole.Spec.Subjects[0:idx], envRole.Spec.Subjects[idx+1:]...)
					_, err = envRoleInterface.PatchUpdate(envRole)
					if err != nil {
						return errors.Wrapf(err, "Failed to remove User %s kind %s as a Subject of EnvironmentRoleBinding %s: %s", userName, userKind, name, err)
					}
					break
				}
			}
			if !found {
				log.Logger().Warnf("User %s kind %s is not a Subject of EnvironmentRoleBinding %s", userName, userKind, name)
			}
		}
	}
	for _, name := range createRoles {
		envRole := envRoles[name]
		if envRole == nil {
			// TODO lazily create the EnvironmentRoleBinding?
			log.Logger().Warnf("Could not add user %s to EnvironmentRoleBinding %s as it does not exist!", userName, name)
		} else {
			found := false
			for _, subject := range envRole.Spec.Subjects {
				if subject.Kind == userKind && subject.Name == userName && subject.Namespace == adminNs {
					found = true
				}
			}
			if found {
				log.Logger().Warnf("User %s kind %s is already a Subject of EnvironmentRoleBinding %s", userName, userKind, name)
			} else {
				newSubject := rbacv1.Subject{
					Name:      userName,
					Kind:      userKind,
					Namespace: adminNs,
				}
				newEnvRole, err := envRoleInterface.Get(envRole.Name, metav1.GetOptions{})
				create := false
				if err != nil {
					create = true
					newEnvRole = envRole
				} else {
					newEnvRole.Spec = envRole.Spec
				}
				newEnvRole.Spec.Subjects = append(newEnvRole.Spec.Subjects, newSubject)
				if create {
					_, err = envRoleInterface.Create(newEnvRole)
					if err != nil {
						return errors.Wrapf(err, "Failed to create EnvironmentRoleBinding %s with Subject User %s kind %s: %s", name, userName, userKind, err)
					}
				} else {
					_, err = envRoleInterface.PatchUpdate(newEnvRole)
					if err != nil {
						return errors.Wrapf(err, "Failed to add User %s kind %s as a Subject of EnvironmentRoleBinding %s: %s", userName, userKind, name, err)
					}
				}
			}
		}
	}
	return nil
}

func userRolesFor(userKind string, userName string, userNamespace string, envRoles map[string]*v1.EnvironmentRoleBinding) []string {
	answer := []string{}
	for _, envRole := range envRoles {
		for _, subject := range envRole.Spec.Subjects {
			if subject.Kind == userKind && subject.Name == userName && subject.Namespace == userNamespace {
				answer = append(answer, envRole.Name)
			}
		}
	}
	return answer
}

// IsClusterRoleBinding checks if the cluster role binding exists
func IsClusterRoleBinding(kubeClient kubernetes.Interface, name string) bool {
	_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	return true
}

// CreateClusterRoleBinding creates acluster role binding in a given namespace for a service account
func CreateClusterRoleBinding(kubeClient kubernetes.Interface, namespace string, name string,
	serviceAccountName string, clusterRoleName string) error {
	rb := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       clusterRoleBindingKind,
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      subjectKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: apiGroup,
			Kind:     clusterRoleKind,
			Name:     clusterRoleName,
		},
	}

	_, err := kubeClient.RbacV1().ClusterRoleBindings().Create(rb)
	if err != nil {
		return errors.Wrap(err, "creating cluster role binding")
	}
	return nil
}

// DeleteClusterRoleBinding deltes a cluster role binding
func DeleteClusterRoleBinding(kubeClient kubernetes.Interface, name string) error {
	_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(name, metav1.GetOptions{})
	if err == nil {
		return kubeClient.RbacV1().ClusterRoleBindings().Delete(name, &metav1.DeleteOptions{})
	}
	return nil
}

// IsClusterRole checks if a cluster role exists
func IsClusterRole(kubeClient kubernetes.Interface, name string) bool {
	_, err := kubeClient.RbacV1().ClusterRoles().Get(name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	return true
}

// CreateClusterRole creates a new cluster role
func CreateClusterRole(kubeClient kubernetes.Interface, namesapce string, name string,
	apiGroups []string, resources []string, verbs []string) error {
	role := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       clusterRoleKind,
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: apiGroups,
				Resources: resources,
				Verbs:     verbs,
			},
		},
	}
	_, err := kubeClient.RbacV1().ClusterRoles().Create(role)
	if err != nil {
		return errors.Wrap(err, "creating custer role")
	}
	return nil
}

// DeleteClusterRole deletes a cluster role if exists
func DeleteClusterRole(kubeClient kubernetes.Interface, name string) error {
	if IsClusterRole(kubeClient, name) {
		return kubeClient.RbacV1().ClusterRoles().Delete(name, &metav1.DeleteOptions{})
	}
	return nil
}
