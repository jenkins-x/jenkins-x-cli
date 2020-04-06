package controller

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// ControllerBackupOptions are the flags for the commands
type ControllerBackupOptions struct {
	ControllerOptions

	GitRepositoryOptions gits.GitRepositoryOptions

	Namespace    string
	Organisation string
}

// NewCmdControllerBackup creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdControllerBackup(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ControllerBackupOptions{
		ControllerOptions: ControllerOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Runs the backup controller",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		Aliases: []string{"backups"},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to watch or defaults to the current namespace")
	cmd.Flags().StringVarP(&options.Organisation, "organisation", "o", "", "The organisation to backup")

	return cmd
}

// Run implements this command
func (o *ControllerBackupOptions) Run() error {
	// ensure Environment / Team / User CRDs are registered before we start
	err := o.RegisterEnvironmentCRD()
	if err != nil {
		return err
	}

	err = o.RegisterTeamCRD()
	if err != nil {
		return err
	}

	err = o.RegisterUserCRD()
	if err != nil {
		return err
	}

	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if ns == "" {
		ns = devNs
	}

	dir, err := o.getOrCreateBackupRepository()

	log.Logger().Infof("Watching for users/teams/environments in namespace %s", util.ColorInfo(ns))

	_, environmentController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return jxClient.JenkinsV1().Environments(ns).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return jxClient.JenkinsV1().Environments(ns).Watch(lo)
			},
		},
		&v1.Environment{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onEnvironmentChange(obj, ns, dir)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onEnvironmentChange(newObj, ns, dir)
			},
			DeleteFunc: func(obj interface{}) {
				//o.onEnvironmentChange(obj, jxClient, ns)
			},
		},
	)

	stop := make(chan struct{})
	go environmentController.Run(stop)

	_, teamController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return jxClient.JenkinsV1().Teams(ns).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return jxClient.JenkinsV1().Teams(ns).Watch(lo)
			},
		},
		&v1.Team{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onTeamChange(obj, ns, dir)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onTeamChange(newObj, ns, dir)
			},
			DeleteFunc: func(obj interface{}) {
				//o.onEnvironmentChange(obj, jxClient, ns)
			},
		},
	)

	go teamController.Run(stop)

	_, userController := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(lo meta_v1.ListOptions) (runtime.Object, error) {
				return jxClient.JenkinsV1().Users(ns).List(lo)
			},
			WatchFunc: func(lo meta_v1.ListOptions) (watch.Interface, error) {
				return jxClient.JenkinsV1().Users(ns).Watch(lo)
			},
		},
		&v1.User{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onUserChange(obj, ns, dir)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onUserChange(newObj, ns, dir)
			},
			DeleteFunc: func(obj interface{}) {
				//o.onEnvironmentChange(obj, jxClient, ns)
			},
		},
	)

	go userController.Run(stop)

	// Wait forever
	select {}
}

func (o *ControllerBackupOptions) onEnvironmentChange(obj interface{}, ns string, dir string) {
	env, ok := obj.(*v1.Environment)
	if !ok {
		log.Logger().Infof("Object is not a Environment %#v", obj)
		return
	}
	o.writeResourceToBackupFile(env, "environment", env.ObjectMeta.Name, ns, dir)
}

func (o *ControllerBackupOptions) onTeamChange(obj interface{}, ns string, dir string) {
	env, ok := obj.(*v1.Team)
	if !ok {
		log.Logger().Infof("Object is not a Team %#v", obj)
		return
	}
	o.writeResourceToBackupFile(env, "team", env.ObjectMeta.Name, ns, dir)
}

func (o *ControllerBackupOptions) onUserChange(obj interface{}, ns string, dir string) {
	env, ok := obj.(*v1.User)
	if !ok {
		log.Logger().Infof("Object is not a User %#v", obj)
		return
	}
	o.writeResourceToBackupFile(env, "user", env.ObjectMeta.Name, ns, dir)
}

func (o *ControllerBackupOptions) writeResourceToBackupFile(obj interface{}, resource string, key string, ns string, dir string) {
	out, err := yaml.Marshal(obj)
	if err != nil {
		log.Logger().Errorf("Unable to marshall %s %s", resource, err)
		return
	}

	log.Logger().Debugf("Dumping %s with key %s...", util.ColorInfo(resource), util.ColorInfo(key))
	log.Logger().Debugf("%s", string(out))

	nsDir := path.Join(dir, fmt.Sprintf("%ss", resource), ns)
	err = os.MkdirAll(nsDir, os.FileMode(0755))
	if err != nil {
		log.Logger().Errorf("Unable to create directory %s", err)
		return
	}

	envFile := path.Join(nsDir, fmt.Sprintf("%s.yaml", key))
	err = ioutil.WriteFile(envFile, out, 0644)
	if err != nil {
		log.Logger().Errorf("Unable to write file %s", err)
		return
	}

	o.commitDirIfChanges(dir, fmt.Sprintf("Updating %s %s", resource, key))
}

func (o *ControllerBackupOptions) commitDirIfChanges(dir string, message string) {
	changes, err := o.Git().HasChanges(dir)
	if err != nil {
		log.Logger().Errorf("Unable to determine changes %s", err)
		return
	}

	if changes {
		err = o.Git().Add(dir, "*")
		if err != nil {
			log.Logger().Errorf("Unable to add files %s", err)
			return
		}

		err = o.Git().CommitDir(dir, message)
		if err != nil {
			log.Logger().Errorf("Unable to commit dir %s", err)
			return
		}

		err = o.Git().PushMaster(dir)
		if err != nil {
			log.Logger().Errorf("Unable to push master %s", err)
			return
		}

		fmt.Fprintf(o.Out, "Pushed update '%s' Git repository %s\n", util.ColorInfo(message), util.ColorInfo(dir))
	}
}

func (o *ControllerBackupOptions) getOrCreateBackupRepository() (string, error) {
	backupDir, err := util.BackupDir()
	if err != nil {
		return "", err
	}

	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return "", err
	}

	defaultRepoName := fmt.Sprintf("organisation-%s-backup", o.Organisation)

	details, err := gits.PickNewOrExistingGitRepository(o.BatchMode, authConfigSvc,
		defaultRepoName, &o.GitRepositoryOptions, nil, nil, o.Git(), true, o.GetIOFileHandles())
	if err != nil {
		return "", err
	}
	org := details.Organisation
	repoName := details.RepoName
	owner := org
	if owner == "" {
		owner = details.User.Username
	}
	provider := details.GitProvider
	repo, err := provider.GetRepository(owner, repoName)
	remoteRepoExists := err == nil
	var dir string

	if !remoteRepoExists {
		fmt.Fprintf(o.Out, "Creating Git repository %s/%s\n", util.ColorInfo(owner), util.ColorInfo(repoName))

		repo, err = details.CreateRepository()
		if err != nil {
			return "", err
		}

		dir, err = util.CreateUniqueDirectory(backupDir, details.RepoName, util.MaximumNewDirectoryAttempts)
		if err != nil {
			return "", err
		}

		err = o.Git().Init(dir)
		if err != nil {
			return "", err
		}

		pushGitURL, err := o.Git().CreateAuthenticatedURL(repo.CloneURL, details.User)
		if err != nil {
			return "", err
		}

		err = o.Git().SetRemoteURL(dir, "origin", pushGitURL)
		if err != nil {
			return "", err
		}
	} else {
		fmt.Fprintf(o.Out, "Git repository %s/%s already exists\n", util.ColorInfo(owner), util.ColorInfo(repoName))

		dir = path.Join(backupDir, details.RepoName)
		localDirExists, err := util.DirExists(dir)
		if err != nil {
			return "", err
		}

		if localDirExists {
			// if remote repo does exist & local does exist, git pull the local repo
			fmt.Fprintf(o.Out, "local directory already exists\n")

			err = o.Git().Pull(dir)
			if err != nil {
				return "", err
			}
		} else {
			fmt.Fprintf(o.Out, "cloning repository locally\n")
			err = os.MkdirAll(dir, os.FileMode(0755))
			if err != nil {
				return "", err
			}

			// if remote repo does exist & local directory does not exist, clone locally
			pushGitURL, err := o.Git().CreateAuthenticatedURL(repo.CloneURL, details.User)
			if err != nil {
				return "", err
			}

			err = o.Git().Clone(pushGitURL, dir)
			if err != nil {
				return "", err
			}
		}
	}

	return dir, nil
}
