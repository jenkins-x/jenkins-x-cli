package controller

import (
	"io/ioutil"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/step/git/credentials"

	"github.com/jenkins-x/jx/pkg/environments"

	"github.com/jenkins-x/jx/pkg/cmd/create"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ControllerTeamOptions are the flags for the commands
type ControllerTeamOptions struct {
	ControllerOptions
	create.InstallOptions

	GitRepositoryOptions gits.GitRepositoryOptions
}

// NewCmdControllerTeam creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdControllerTeam(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ControllerTeamOptions{
		ControllerOptions: ControllerOptions{
			CommonOptions: commonOpts,
		},
		InstallOptions: create.CreateInstallOptions(commonOpts),
	}

	cmd := &cobra.Command{
		Use:   "team",
		Short: "Runs the team controller",
		Run: func(cmd *cobra.Command, args []string) {
			options.ControllerOptions.Cmd = cmd
			options.ControllerOptions.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		Aliases: []string{"team"},
	}

	options.InstallOptions.AddInstallFlags(cmd, true)

	return cmd
}

// Run implements this command
func (o *ControllerTeamOptions) Run() error {
	co := &o.ControllerOptions

	// lets ensure helm is initialised
	err := co.Helm().Init(true, "", "", false)
	if err != nil {
		return errors.Wrapf(err, "failed to initialise helm")
	}

	err = co.RegisterTeamCRD()
	if err != nil {
		return err
	}

	jxClient, adminNs, err := co.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	log.Logger().Infof("Using the admin namespace %s", adminNs)

	client, err := co.KubeClient()
	if err != nil {
		return err
	}

	// lets validate we have git configured
	_, _, err = gits.EnsureUserAndEmailSetup(co.Git())
	if err != nil {
		return err
	}

	// now lets setup the git secrets
	if co.InCluster() {
		sgc := &credentials.StepGitCredentialsOptions{}
		sgc.CommonOptions = co.CommonOptions
		log.Logger().Info("Setting up git credentials")
		err = sgc.Run()
		if err != nil {
			return errors.Wrapf(err, "Failed to run: jx step git credentials")
		}
	}

	log.Logger().Infof("Watching for teams in all namespaces")

	stop := make(chan struct{})

	_, teamController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo metav1.ListOptions) (runtime.Object, error) {
				return jxClient.JenkinsV1().Teams(adminNs).List(lo)
			},
			WatchFunc: func(lo metav1.ListOptions) (watch.Interface, error) {
				return jxClient.JenkinsV1().Teams(adminNs).Watch(lo)
			},
		},
		&v1.Team{},
		time.Minute*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onTeamChange(obj, client, jxClient, adminNs)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onTeamChange(newObj, client, jxClient, adminNs)
			},
			DeleteFunc: func(obj interface{}) {
				// do nothing, already handled by 'jx delete team'
			},
		},
	)

	go teamController.Run(stop)

	// Wait forever
	select {}
}

func (o *ControllerTeamOptions) onTeamChange(obj interface{}, kubeClient kubernetes.Interface, jxClient versioned.Interface, adminNs string) {
	team, ok := obj.(*v1.Team)
	if !ok {
		log.Logger().Infof("Object is not a Team %#v", obj)
		return
	}

	teamNs := team.Name
	log.Logger().Infof("Adding / Updating Team %s, Namespace %s, Status '%s'", util.ColorInfo(teamNs), util.ColorInfo(team.Namespace), util.ColorInfo(team.Status.ProvisionStatus))

	if v1.TeamProvisionStatusNone == team.Status.ProvisionStatus {
		// update first
		oc := &o.ControllerOptions
		oc.SetDevNamespace(adminNs)

		// lets default the team settings based on the current team settings
		settings, err := oc.TeamSettings()
		if err != nil {
			log.Logger().Errorf("Failed to get TeamSettings: %s", err)
		}
		if settings == nil {
			log.Logger().Errorf("No TeamSettings found!")
		}

		// lets default to no tiller as we can only support > 1 dev teams with no-tiller or helm3 today
		// due to the globally unique naming of release in helm with a global tiller
		o.InstallOptions.InitOptions.Flags.NoTiller = true
		if err == nil && settings != nil {
			if settings.HelmBinary == "helm3" {
				o.InstallOptions.InitOptions.Flags.Helm3 = true
				o.InstallOptions.InitOptions.Flags.NoTiller = false
			}
			if settings.NoTiller {
				o.InstallOptions.InitOptions.Flags.RemoteTiller = false
			} else if settings.PromotionEngine == v1.PromotionEngineProw {
				o.InstallOptions.Flags.Prow = true
			}
			if settings.ProwEngine == v1.ProwEngineTypeTekton {
				o.InstallOptions.Flags.Tekton = true
			}
		}

		err = oc.ModifyTeam(adminNs, team.Name, func(team *v1.Team) error {
			team.Status.ProvisionStatus = v1.TeamProvisionStatusPending
			team.Status.Message = "Installing resources"
			return nil
		})
		if err != nil {
			log.Logger().Errorf("Unable to update team %s to %s - %s", util.ColorInfo(teamNs), v1.TeamProvisionStatusPending, err)
			return
		}

		// ensure that the namespace exists
		err = kube.EnsureNamespaceCreated(kubeClient, teamNs, nil, nil)
		if err != nil {
			log.Logger().Errorf("Unable to create namespace %s: %s", util.ColorInfo(teamNs), err)
			return
		}

		// lets default the login/pwd for Jenkins from the admin cluster
		io := &o.InstallOptions
		io.SetDevNamespace(teamNs)
		oc.SetDevNamespace(teamNs)

		io.AdminSecretsService.Flags.DefaultAdminPassword, err = oc.GetDefaultAdminPassword(adminNs)
		if err != nil {
			log.Logger().Warnf("Failed to load the default admin password from namespace %s: %s", adminNs, err)
		}

		if io.CreateEnvOptions.HelmValuesConfig.ExposeController == nil {
			io.CreateEnvOptions.HelmValuesConfig.ExposeController = &config.ExposeController{}
		}
		ec := io.CreateEnvOptions.HelmValuesConfig.ExposeController
		// lets load the exposecontroller configuration
		ingressConfig, err := kube.GetIngressConfig(kubeClient, adminNs)
		if err != nil {
			log.Logger().Errorf("Failed to load the IngressConfig in namespace %s: %s", adminNs, err)
			return
		}
		ec.Config.Domain = ingressConfig.Domain
		ec.Config.Exposer = ingressConfig.Exposer
		if ingressConfig.TLS {
			ec.Config.HTTP = "false"
			ec.Config.TLSAcme = "true"
		} else {
			ec.Config.HTTP = "true"
			ec.Config.TLSAcme = "false"
		}

		io.BatchMode = true
		io.InitOptions.Flags.SkipIngress = true

		adminTeamSettings, _ := oc.TeamSettings()

		// TODO lets load this from the current team
		provider := ""
		if adminTeamSettings != nil {
			provider = adminTeamSettings.KubeProvider
			io.GitRepositoryOptions.Public = adminTeamSettings.GitPublic
			io.GitRepositoryOptions.ServerURL = adminTeamSettings.GitServer
			io.GitRepositoryOptions.Username = adminTeamSettings.PipelineUsername
		}
		if provider == "" {
			log.Logger().Warnf("No kube provider specified on admin team settings %s\n. Defaulting to gke", adminNs)
			provider = "gke"
		}
		io.Flags.Provider = provider
		io.Flags.DisableSetKubeContext = true

		//o.InstallOptions.Flags.NoDefaultEnvironments = true
		io.Flags.Namespace = teamNs
		io.Flags.DefaultEnvironmentPrefix = teamNs
		io.CommonOptions.InstallDependencies = true

		if io.Flags.Prow {
			oauthToken, err := oc.LoadProwOAuthConfig(adminNs)
			if err != nil {
				log.Logger().Errorf("Failed to load the Prow OAuth Token in namespace %s: %s", adminNs, err)
			} else {
				io.OAUTHToken = oauthToken
				log.Logger().Infof("Loaded the Prow OAuth Token in namespace %s with %d digits", adminNs, len(oauthToken))

			}
		}

		// lets copy the myvalues.yaml file from the ConfigMap
		cm, err := kubeClient.CoreV1().ConfigMaps(adminNs).Get(kube.ConfigMapJenkinsTeamController, metav1.GetOptions{})
		if err != nil {
			log.Logger().Errorf("Failed to load the ConfigMap %s from namespace %s: %s", kube.ConfigMapJenkinsTeamController, adminNs, err)
		} else {
			if cm.Data != nil {
				valuesYaml := cm.Data["myvalues.yaml"]
				if valuesYaml != "" {
					err = ioutil.WriteFile("myvalues.yaml", []byte(valuesYaml), util.DefaultWritePermissions)
					if err != nil {
						log.Logger().Errorf("Failed to write the myvalues.yaml file: %s", err)
					}
				}
			}
		}

		// call jx install
		io.SetDevNamespace(teamNs)
		io.CreateEnvOptions.SetDevNamespace(teamNs)
		err = io.Run()
		if err != nil {
			log.Logger().Errorf("Unable to install jx for team %s: %s", util.ColorInfo(teamNs), err)
			err = oc.ModifyTeam(adminNs, team.Name, func(team *v1.Team) error {
				team.Status.ProvisionStatus = v1.TeamProvisionStatusError
				team.Status.Message = err.Error()
				return nil
			})
			if err != nil {
				log.Logger().Errorf("Unable to update team %s to %s - %s", util.ColorInfo(teamNs), v1.TeamProvisionStatusError, err)
				return
			}
			return
		}

		if adminTeamSettings != nil {
			callback := func(env *v1.Environment) error {
				env.Spec.TeamSettings.BuildPackRef = adminTeamSettings.BuildPackRef
				env.Spec.TeamSettings.BuildPackURL = adminTeamSettings.BuildPackURL
				return nil
			}
			err = environments.ModifyDevEnvironmentWithNs(jxClient, teamNs, callback)
			if err != nil {
				log.Logger().Errorf("Failed to update team settings in namespace %s: %s", teamNs, err)
			}
		}

		err = kube.SetAdminNamespace(kubeClient, teamNs, adminNs)
		if err != nil {
			log.Logger().Errorf("Unable set admin namespace on team %s to %s - %s", util.ColorInfo(teamNs), util.ColorInfo(adminNs), err)
			return
		}
		err = oc.ModifyTeam(adminNs, team.Name, func(team *v1.Team) error {
			team.Status.ProvisionStatus = v1.TeamProvisionStatusComplete
			team.Status.Message = "Installation complete"
			return nil
		})
		if err != nil {
			log.Logger().Errorf("Unable to update team %s to %s - %s", util.ColorInfo(teamNs), v1.TeamProvisionStatusComplete, err)
			return
		}
	} else if v1.TeamProvisionStatusComplete == team.Status.ProvisionStatus {
		err := kube.SetAdminNamespace(kubeClient, teamNs, adminNs)
		if err != nil {
			log.Logger().Errorf("Unable set admin namespace on team %s to %s - %s", util.ColorInfo(teamNs), util.ColorInfo(adminNs), err)
			return
		}
	}

}

// LoadProwOAuthConfig returns the OAuth Token for Prow
func (o *ControllerOptions) LoadProwOAuthConfig(ns string) (string, error) {
	options := *o
	options.SetDevNamespace(ns)
	options.SkipAuthSecretsMerge = false
	authConfigSvc, err := options.GitAuthConfigService()
	if err != nil {
		return "", err
	}

	config := authConfigSvc.Config()
	// lets assume github.com for now so ignore config.CurrentServer
	server := config.GetOrCreateServer("https://github.com")
	userAuth, err := config.PickServerUserAuth(server, "Git account to be used to send webhook events", true, "", o.GetIOFileHandles())
	if err != nil {
		return "", err
	}
	return userAuth.ApiToken, nil
}
