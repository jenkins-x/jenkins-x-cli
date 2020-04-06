package controller

import (
	"reflect"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ControllerRoleOptions the command line options
type ControllerRoleOptions struct {
	ControllerOptions

	NoWatch bool

	Roles           map[string]*rbacv1.Role
	EnvRoleBindings map[string]*v1.EnvironmentRoleBinding
	TeamNs          string
}

type ControllerRoleFlags struct {
	Version string
}

const blankSting = ""

var (
	controllerRoleLong = templates.LongDesc(`
		Controller which replicas Role and EnvironmentRoleBinding resources to Roles and RoleBindings in all matching Environment namespaces. 

		RBAC in Kubernetes is either global with ClusterRoles or is namespace based with Roles per Namespace.

		We use a Custom Resource called EnvironmentRoleBinding which binds Roles and its bindings from the development 
		environment into each Environment's Namespace. 

		e.g. each EnvironmentRoleBinding will result in a RoleBinding and Role resource being create in each matching Environment. 
		So when a Preview environment is created it will have the correct Role and RoleBinding resources added. 

`)

	controllerRoleExample = templates.Examples(`
		# watch for changes in Role and EnvironmentRoleBindings in the dev namespace
		# and update the Role + RoleBinding resources in each environment namespace 
		jx controller role

		# update the current RoleBinding resources in each environment based on the current EnvironmentRoleBindings
		jx controller role --no-watch

`)
)

func NewCmdControllerRole(commonOpts *opts.CommonOptions) *cobra.Command {
	options := ControllerRoleOptions{
		ControllerOptions: ControllerOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "role",
		Short:   "Controller which mirrors Role & EnvironmentRoleBinding resources to Roles and RoleBindings in all matching Environment namespaces",
		Long:    controllerRoleLong,
		Example: controllerRoleExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().BoolVarP(&options.NoWatch, "no-watch", "n", false, "To disable watching of the resources - to enable one-shot mode")
	return cmd
}

func (o *ControllerRoleOptions) Run() error {
	apiClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterEnvironmentCRD(apiClient)
	if err != nil {
		return err
	}
	err = kube.RegisterEnvironmentRoleBindingCRD(apiClient)
	if err != nil {
		return err
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	o.TeamNs = ns
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	if !o.NoWatch {
		err = o.WatchRoles(kubeClient, ns)
		if err != nil {
			return err
		}
		err = o.WatchEnvironmentRoleBindings(jxClient, ns)
		if err != nil {
			return err
		}
		err = o.WatchEnvironments(kubeClient, jxClient, ns)
		if err != nil {
			return err
		}
	}

	roles, err := kubeClient.RbacV1().Roles(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, role := range roles.Items {
		o.UpsertRole(&role)
	}
	bindings, err := jxClient.JenkinsV1().EnvironmentRoleBindings(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range bindings.Items {
		o.UpsertEnvironmentRoleBinding(&bindings.Items[i])
	}
	envList, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, env := range envList.Items {
		err = o.upsertEnvironment(kubeClient, ns, &env)
		if err != nil {
			return err
		}
	}
	o.upsertRoleIntoEnvRole(ns, jxClient)
	return nil
}

func (o *ControllerRoleOptions) WatchRoles(kubeClient kubernetes.Interface, ns string) error {
	role := &rbacv1.Role{}
	listWatch := cache.NewListWatchFromClient(kubeClient.RbacV1().RESTClient(), "roles", ns, fields.Everything())
	kube.SortListWatchByName(listWatch)
	_, controller := cache.NewInformer(
		listWatch,
		role,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onRole(nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onRole(oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				o.onRole(obj, nil)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	return nil
}

func (o *ControllerRoleOptions) WatchEnvironmentRoleBindings(jxClient versioned.Interface, ns string) error {
	environmentRoleBinding := &v1.EnvironmentRoleBinding{}
	listWatch := cache.NewListWatchFromClient(jxClient.JenkinsV1().RESTClient(), "environmentrolebindings", ns, fields.Everything())
	kube.SortListWatchByName(listWatch)
	_, controller := cache.NewInformer(
		listWatch,
		environmentRoleBinding,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onEnvironmentRoleBinding(nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onEnvironmentRoleBinding(oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				o.onEnvironmentRoleBinding(obj, nil)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)
	return nil
}

func (o *ControllerRoleOptions) WatchEnvironments(kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string) error {
	environment := &v1.Environment{}
	listWatch := cache.NewListWatchFromClient(jxClient.JenkinsV1().RESTClient(), "environments", ns, fields.Everything())
	kube.SortListWatchByName(listWatch)
	_, controller := cache.NewInformer(
		listWatch,
		environment,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onEnvironment(kubeClient, ns, nil, obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onEnvironment(kubeClient, ns, oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				o.onEnvironment(kubeClient, ns, obj, nil)
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}

func (o *ControllerRoleOptions) onEnvironment(kubeClient kubernetes.Interface, ns string, oldObj interface{}, newObj interface{}) {
	var newEnv *v1.Environment
	if newObj != nil {
		newEnv = newObj.(*v1.Environment)
	}
	if oldObj != nil {
		oldEnv := oldObj.(*v1.Environment)
		if oldEnv != nil {
			if newEnv == nil || newEnv.Spec.Namespace != oldEnv.Spec.Namespace {
				err := o.removeEnvironment(kubeClient, ns, oldEnv)
				if err != nil {
					log.Logger().Warnf("Failed to remove role bindings for environment %s: %s", oldEnv.Name, err)
				}
			}
		}
	}
	if newEnv != nil {
		err := o.upsertEnvironment(kubeClient, ns, newEnv)
		if err != nil {
			log.Logger().Warnf("Failed to upsert role bindings for environment %s: %s", newEnv.Name, err)
		}
	}
}

func (o *ControllerRoleOptions) upsertEnvironment(kubeClient kubernetes.Interface, teamNs string, env *v1.Environment) error {
	errors := []error{}
	ns := env.Spec.Namespace
	if ns != "" {
		for _, binding := range o.EnvRoleBindings {
			err := o.upsertEnvironmentRoleBindingRolesInEnvironments(env, binding, teamNs, ns, kubeClient)
			if err != nil {
				errors = append(errors, err)
			}

		}
	}
	return util.CombineErrors(errors...)
}

// upsertEnvironmentRoleBindingRolesInEnvironments for the given environment and environment role binding lets update any role or role bindings if required
func (o *ControllerRoleOptions) upsertEnvironmentRoleBindingRolesInEnvironments(env *v1.Environment, binding *v1.EnvironmentRoleBinding, teamNs string, ns string, kubeClient kubernetes.Interface) error {
	errors := []error{}
	if kube.EnvironmentMatchesAny(env, binding.Spec.Environments) {
		var err error
		if ns != teamNs {
			roleName := binding.Spec.RoleRef.Name
			role := o.Roles[roleName]
			if role == nil {
				log.Logger().Warnf("Cannot find role %s in namespace %s", roleName, teamNs)
			} else {
				roles := kubeClient.RbacV1().Roles(ns)
				var oldRole *rbacv1.Role
				oldRole, err = roles.Get(roleName, metav1.GetOptions{})
				if err == nil && oldRole != nil {
					// lets update it
					changed := false
					if !reflect.DeepEqual(oldRole.Rules, role.Rules) {
						oldRole.Rules = role.Rules
						changed = true
					}
					if changed {
						log.Logger().Infof("Updating Role %s in namespace %s", roleName, ns)
						_, err = roles.Update(oldRole)
					}
				} else {
					log.Logger().Infof("Creating Role %s in namespace %s", roleName, ns)
					newRole := &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name: roleName,
							Labels: map[string]string{
								kube.LabelCreatedBy: kube.ValueCreatedByJX,
								kube.LabelTeam:      teamNs,
							},
						},
						Rules: role.Rules,
					}
					_, err = roles.Create(newRole)
				}
			}
		}
		if err != nil {
			log.Logger().Warnf("Failed: %s", err)
			errors = append(errors, err)
		}

		bindingName := binding.Name
		roleBindings := kubeClient.RbacV1().RoleBindings(ns)
		var old *rbacv1.RoleBinding
		old, err = roleBindings.Get(bindingName, metav1.GetOptions{})
		if err == nil && old != nil {
			// lets update it
			changed := false

			if !reflect.DeepEqual(old.RoleRef, binding.Spec.RoleRef) {
				old.RoleRef = binding.Spec.RoleRef
				changed = true
			}
			if !reflect.DeepEqual(old.Subjects, binding.Spec.Subjects) {
				old.Subjects = binding.Spec.Subjects
				changed = true
			}
			if changed {
				log.Logger().Infof("Updating RoleBinding %s in namespace %s", bindingName, ns)
				_, err = roleBindings.Update(old)
			}
		} else {
			log.Logger().Infof("Creating RoleBinding %s in namespace %s", bindingName, ns)
			newBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bindingName,
					Labels: map[string]string{
						kube.LabelCreatedBy: kube.ValueCreatedByJX,
						kube.LabelTeam:      teamNs,
					},
				},
				Subjects: binding.Spec.Subjects,
				RoleRef:  binding.Spec.RoleRef,
			}
			_, err = roleBindings.Create(newBinding)
		}
		if err != nil {
			log.Logger().Warnf("Failed: %s", err)
			errors = append(errors, err)
		}
	}
	return util.CombineErrors(errors...)
}

func (o *ControllerRoleOptions) removeEnvironment(kubeClient kubernetes.Interface, curNs string, env *v1.Environment) error {
	ns := env.Spec.Namespace
	if ns != "" {
		for _, binding := range o.EnvRoleBindings {
			if kube.EnvironmentMatchesAny(env, binding.Spec.Environments) {
				// ignore errors
				kubeClient.RbacV1().RoleBindings(ns).Delete(binding.Name, nil)
			}
		}
	}
	return nil
}

func (o *ControllerRoleOptions) onEnvironmentRoleBinding(oldObj interface{}, newObj interface{}) {
	if o.EnvRoleBindings == nil {
		o.EnvRoleBindings = map[string]*v1.EnvironmentRoleBinding{}
	}
	if oldObj != nil {
		oldEnv := oldObj.(*v1.EnvironmentRoleBinding)
		if oldEnv != nil {
			delete(o.EnvRoleBindings, oldEnv.Name)
		}
	}
	if newObj != nil {
		newEnv := newObj.(*v1.EnvironmentRoleBinding)
		o.UpsertEnvironmentRoleBinding(newEnv)
	}
}

// UpsertEnvironmentRoleBinding processes an insert/update of the EnvironmentRoleBinding resource
// its public so that we can make testing easier
func (o *ControllerRoleOptions) UpsertEnvironmentRoleBinding(newEnv *v1.EnvironmentRoleBinding) error {
	if newEnv != nil {
		if o.EnvRoleBindings == nil {
			o.EnvRoleBindings = map[string]*v1.EnvironmentRoleBinding{}
		}
		o.EnvRoleBindings[newEnv.Name] = newEnv
	}

	ns := o.TeamNs
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	jxClient, _, err := o.JXClient()
	if err != nil {
		return err
	}

	// now lets update any roles in any environment we may need to change
	envList, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	errors := []error{}
	for _, env := range envList.Items {
		err = o.upsertEnvironmentRoleBindingRolesInEnvironments(&env, newEnv, ns, env.Spec.Namespace, kubeClient)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return util.CombineErrors(errors...)
}

func (o *ControllerRoleOptions) onRole(oldObj interface{}, newObj interface{}) {
	if o.Roles == nil {
		o.Roles = map[string]*rbacv1.Role{}
	}
	if oldObj != nil {
		oldRole := oldObj.(*rbacv1.Role)
		if oldRole != nil {
			delete(o.Roles, oldRole.Name)
		}
	}
	if newObj != nil {
		newRole := newObj.(*rbacv1.Role)
		o.UpsertRole(newRole)
	}
}

// UpsertRole processes the insert/update of a Role
// this function is public for easier testing
func (o *ControllerRoleOptions) UpsertRole(newRole *rbacv1.Role) error {
	if newRole == nil {
		return nil
	}
	if o.Roles == nil {
		o.Roles = map[string]*rbacv1.Role{}
	}
	o.Roles[newRole.Name] = newRole

	if newRole.Labels == nil || newRole.Labels[kube.LabelKind] != kube.ValueKindEnvironmentRole {
		return nil
	}

	ns := o.TeamNs
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	jxClient, _, err := o.JXClient()
	if err != nil {
		return err
	}

	// now lets update any roles in any environment we may need to change
	envList, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	errors := []error{}
	for _, env := range envList.Items {
		err = o.upsertRoleInEnvironments(&env, newRole, ns, env.Spec.Namespace, kubeClient)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return util.CombineErrors(errors...)
}

// upsertRoleInEnvironments updates the Role in the team environment in the other environment namespaces if it has changed
func (o *ControllerRoleOptions) upsertRoleInEnvironments(env *v1.Environment, role *rbacv1.Role, teamNs string, ns string, kubeClient kubernetes.Interface) error {
	if ns == teamNs {
		return nil
	}
	var err error
	roleName := role.Name
	roles := kubeClient.RbacV1().Roles(ns)
	var oldRole *rbacv1.Role
	oldRole, err = roles.Get(roleName, metav1.GetOptions{})
	if err == nil && oldRole != nil {
		// lets update it
		changed := false
		if !reflect.DeepEqual(oldRole.Rules, role.Rules) {
			oldRole.Rules = role.Rules
			changed = true
		}
		if changed {
			log.Logger().Infof("Updating Role %s in namespace %s", roleName, ns)
			_, err = roles.Update(oldRole)
		}
	} else {
		log.Logger().Infof("Creating Role %s in namespace %s", roleName, ns)
		newRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
				Labels: map[string]string{
					kube.LabelCreatedBy: kube.ValueCreatedByJX,
					kube.LabelTeam:      teamNs,
				},
			},
			Rules: role.Rules,
		}
		_, err = roles.Create(newRole)
	}
	return err
}

func (o *ControllerRoleOptions) upsertRoleIntoEnvRole(ns string, jxClient versioned.Interface) {
	foundRole := 0
	for _, roleValue := range o.Roles {
		for labelK, labelV := range roleValue.Labels {
			if util.StringMatchesPattern(labelK, kube.LabelKind) && util.StringMatchesPattern(labelV, kube.ValueKindEnvironmentRole) {
				for _, envRoleValue := range o.EnvRoleBindings {
					if util.StringMatchesPattern(strings.Trim(roleValue.GetName(), blankSting), strings.Trim(envRoleValue.Spec.RoleRef.Name, blankSting)) {
						foundRole = 1
						break
					}
				}
				if foundRole == 0 {
					log.Logger().Infof("Environment binding doesn't exist for role %s , creating it.", util.ColorInfo(roleValue.GetName()))
					newSubject := rbacv1.Subject{
						Name:      roleValue.GetName(),
						Kind:      kube.ValueKindEnvironmentRole,
						Namespace: ns,
					}
					newEnvRoleBinding := &v1.EnvironmentRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleValue.GetName(),
							Namespace: ns,
						},
						Spec: v1.EnvironmentRoleBindingSpec{
							Subjects: []rbacv1.Subject{
								newSubject,
							},
						},
					}
					jxClient.JenkinsV1().EnvironmentRoleBindings(ns).Create(newEnvRoleBinding)
				}
			}
		}

	}
}
