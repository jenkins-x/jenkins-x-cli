package users

import (
	"fmt"
	"sort"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetUsers returns the pending users with the sorted order of names
func GetUsers(jxClient versioned.Interface, ns string) (map[string]*jenkinsv1.User, []string, error) {
	m := map[string]*jenkinsv1.User{}

	names := []string{}
	userList, err := jxClient.JenkinsV1().Users(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, names, err
	}
	for _, user := range userList.Items {
		n := user.Spec.Login
		if n == "" {
			n = user.Name
		}
		copy := user
		m[n] = &copy
		if n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return m, names, nil
}

// CreateUser creates a new default User
func CreateUser(ns string, login string, name string, email string) *jenkinsv1.User {
	id := naming.ToValidName(login)
	user := &jenkinsv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: ns,
		},
		Spec: jenkinsv1.UserDetails{
			Login: login,
			Name:  name,
			Email: email,
		},
	}
	return user
}

// AddAccountReference adds an account reference and label (created from gitProviderKey and id) to the user
func AddAccountReference(user *jenkinsv1.User, gitProviderKey string, id string) *jenkinsv1.User {
	if user.Spec.Accounts == nil {
		user.Spec.Accounts = make([]jenkinsv1.AccountReference, 0)
	}
	user.Spec.Accounts = append(user.Spec.Accounts, jenkinsv1.AccountReference{
		Provider: gitProviderKey,
		ID:       id,
	})
	if user.ObjectMeta.Labels == nil {
		user.ObjectMeta.Labels = make(map[string]string)
	}
	user.ObjectMeta.Labels[gitProviderKey] = id
	return user
}

// DeleteUser deletes the user resource but does not uninstall the underlying namespaces
func DeleteUser(jxClient versioned.Interface, ns string, userName string) error {
	id := naming.ToValidName(userName)
	userInterface := jxClient.JenkinsV1().Users(ns)
	_, err := userInterface.Get(id, metav1.GetOptions{})
	if err == nil {
		err = userInterface.Delete(id, nil)
	}
	return err
}

// Resolve does the heavy lifting for user resolution.
// This function is not normally called directly but by a dedicated user resolver (e.g. GitUserResolver)
// * checking the user custom resources to see if the user is present there
// * calling selectUsers to try to identify the user
// as often user info is not complete in a git response
func Resolve(id string, providerKey string, jxClient versioned.Interface,
	namespace string, selectUsers func(id string, users []jenkinsv1.User) (string,
		[]jenkinsv1.User, *jenkinsv1.User, error)) (*jenkinsv1.User, error) {

	id = naming.ToValidValue(id)
	if id != "" {

		labelSelector := fmt.Sprintf("%s=%s", providerKey, id)

		// First try to find by label - this is much faster as it uses an index
		users, err := jxClient.JenkinsV1().Users(namespace).List(metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return nil, err
		}
		if len(users.Items) > 1 {
			return nil, fmt.Errorf("more than one user found in users.jenkins.io with label %s, found %v", labelSelector,
				users.Items)
		} else if len(users.Items) == 1 {
			return &users.Items[0], nil
		}
	}

	// Next try without the label - this might occur if someone manually updated the list
	users, err := jxClient.JenkinsV1().Users(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if id != "" {

		possibles := make([]jenkinsv1.User, 0)
		for _, u := range users.Items {
			for _, a := range u.Spec.Accounts {
				if a.Provider == providerKey && a.ID == id {
					possibles = append(possibles, u)
				}
			}
		}
		if len(possibles) > 1 {
			possibleUsers := make([]string, 0)
			for _, p := range possibles {
				possibleUsers = append(possibleUsers, p.Name)
			}
			return nil, fmt.Errorf("more than one user found in users.jenkins.io with login %s for provider %s, "+
				"found %s", id, providerKey, possibleUsers)
		} else if len(possibles) == 1 {
			// Add the label for next time
			found := &possibles[0]
			if found.Labels == nil {
				found.Labels = make(map[string]string)
			}
			found.Labels[providerKey] = id
			found, err := jxClient.JenkinsV1().Users(namespace).Update(found)
			if err != nil {
				return nil, err
			}
			log.Logger().Infof("Adding label %s=%s to user %s in users.jenkins.io", providerKey, id, found.Name)
			return found, nil
		}
	}

	// Finally, try to resolve by callback
	//var possibles []jenkinsv1.User
	//var err error
	id, possibles, new, err := selectUsers(id, users.Items)
	if err != nil {
		return nil, err
	}
	if len(possibles) > 1 {
		possibleStrings := make([]string, 0)
		for _, p := range possibles {
			possibleStrings = append(possibleStrings, p.Name)
		}
		return nil, fmt.Errorf("selected more than one user from users.jenkins.io: %v", possibleStrings)
	} else if len(possibles) == 1 {
		found := &possibles[0]
		if id != "" {
			// Add the git id to the user
			if found.Spec.Accounts == nil {
				found.Spec.Accounts = make([]jenkinsv1.AccountReference, 0)
			}
			found.Spec.Accounts = append(found.Spec.Accounts, jenkinsv1.AccountReference{
				ID:       id,
				Provider: providerKey,
			})
			// Add the label as well
			if found.Labels == nil {
				found.Labels = make(map[string]string)
			}
			found.Labels[providerKey] = id
			log.Logger().Infof("Associating user %s in users.jenkins.io with email %s to git GitProvider user with login %s as "+
				"emails match", found.Name, found.Spec.Email, id)
			log.Logger().Infof("Adding label %s=%s to user %s in users.jenkins.io", providerKey, id, found.Name)
			_, err := jxClient.JenkinsV1().Users(namespace).Update(found)
			if err != nil {
				return nil, err
			}
		}
		return found, nil
	} else {
		// Otherwise, create a new user
		return jxClient.JenkinsV1().Users(namespace).Create(new)
	}
}
