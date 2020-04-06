package opts

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureServiceAccount ensures service account exists in the given namespace
func (o *CommonOptions) EnsureServiceAccount(ns string, serviceAccountName string) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}

	_, err = client.CoreV1().ServiceAccounts(ns).Get(serviceAccountName, meta_v1.GetOptions{})
	if err != nil {
		// lets create a ServiceAccount for tiller
		sa := &corev1.ServiceAccount{
			ObjectMeta: meta_v1.ObjectMeta{Name: serviceAccountName,
				Namespace: ns,
			},
		}
		_, err = client.CoreV1().ServiceAccounts(ns).Create(sa)
		if err != nil {
			return fmt.Errorf("Failed to create ServiceAccount %s in namespace %s: %s", serviceAccountName, ns, err)
		}
		log.Logger().Infof("Created ServiceAccount %s in namespace %s", util.ColorInfo(serviceAccountName), util.ColorInfo(ns))
	}
	return err
}

// EnsureClusterRoleBinding ensures cluster role exists
// Todo: use permissions from somewhere, or provide common ones in a class that we can pass in here
// this is an unimplemented and unused method for building upon that may eventually be of use
func (o *CommonOptions) EnsureClusterRoleExists(roleName string, namespace string) error {
	log.Logger().Infof("Ensuring cluster role exists, role name: %s, namespace: %s", roleName, namespace)

	client, err := o.KubeClient()
	if err != nil {
		return err
	}

	_, err = client.RbacV1().ClusterRoles().Get(roleName, meta_v1.GetOptions{})
	if err != nil {
		log.Logger().Infof("Trying to create ClusterRole %s in namespace %s", roleName, namespace)

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:      roleName,
				Namespace: namespace,
			},
		}

		_, err = client.RbacV1().ClusterRoles().Create(clusterRole)
		if err != nil {
			return fmt.Errorf("Failed to create ClusterRole %s: %s", roleName, err)
		}
		log.Logger().Infof("Created ClusterRole %s in namespace %s", roleName, namespace)
	}
	return nil
}

// EnsureClusterRoleBinding ensures cluster role binding exists
func (o *CommonOptions) EnsureClusterRoleBinding(clusterRoleBindingName string, role string, serviceAccountNamespace string, serviceAccountName string) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}

	_, err = client.RbacV1().ClusterRoleBindings().Get(clusterRoleBindingName, meta_v1.GetOptions{})
	if err != nil {
		log.Logger().Infof("Trying to create ClusterRoleBinding %s for role: %s and ServiceAccount: %s/%s",
			clusterRoleBindingName, role, serviceAccountNamespace, serviceAccountName)

		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: clusterRoleBindingName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccountName,
					Namespace: serviceAccountNamespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     role,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
		_, err = client.RbacV1().ClusterRoleBindings().Create(clusterRoleBinding)
		if err != nil {
			return fmt.Errorf("Failed to create ClusterRoleBindings %s: %s", clusterRoleBindingName, err)
		}
		log.Logger().Infof("Created ClusterRoleBinding %s", clusterRoleBindingName)
	}
	return nil
}
