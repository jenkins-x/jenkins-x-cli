package opts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/buildpacks"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/config"
	jxdraft "github.com/jenkins-x/jx/pkg/draft"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/jenkinsfile/gitresolver"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// InvokeDraftPack used to pass arguments into the draft pack invocation
type InvokeDraftPack struct {
	Dir                         string
	CustomDraftPack             string
	Jenkinsfile                 string
	DefaultJenkinsfile          string
	WithRename                  bool
	InitialisedGit              bool
	DisableJenkinsfileCheck     bool
	DisableAddFiles             bool
	UseNextGenPipeline          bool
	CreateJenkinsxYamlIfMissing bool
	ProjectConfig               *config.ProjectConfig
}

// InitBuildPacks initialise the build packs
func (o *CommonOptions) InitBuildPacks(i *InvokeDraftPack) (string, *v1.TeamSettings, error) {
	settings, err := o.TeamSettings()
	if err != nil {
		return "", settings, err
	}
	buildPackURL := settings.BuildPackURL
	if i != nil && i.ProjectConfig != nil && i.ProjectConfig.BuildPackGitURL != "" {
		buildPackURL = i.ProjectConfig.BuildPackGitURL
	}
	dir, err := gitresolver.InitBuildPack(o.Git(), buildPackURL, settings.BuildPackRef)
	return dir, settings, err
}

// InvokeDraftPack invokes a draft pack copying in a Jenkinsfile if required
func (o *CommonOptions) InvokeDraftPack(i *InvokeDraftPack) (string, error) {
	packsDir, settings, err := o.InitBuildPacks(i)
	if err != nil {
		return "", err
	}

	// lets configure the draft pack mode based on the team settings
	if settings.GetImportMode() == v1.ImportModeTypeYAML {
		i.UseNextGenPipeline = true
		i.CreateJenkinsxYamlIfMissing = true
	}

	dir := i.Dir
	customDraftPack := i.CustomDraftPack
	disableJenkinsfileCheck := i.DisableJenkinsfileCheck
	backupJenkinsfile := true
	initialisedGit := i.InitialisedGit
	withRename := i.WithRename
	jenkinsfilePath := i.Jenkinsfile
	defaultJenkinsfile := i.DefaultJenkinsfile
	if defaultJenkinsfile == "" {
		defaultJenkinsfile = filepath.Join(dir, jenkinsfile.Name)
	}

	pomName := filepath.Join(dir, "pom.xml")
	gradleName := filepath.Join(dir, "build.gradle")
	jenkinsPluginsName := filepath.Join(dir, "plugins.txt")
	packagerConfigName := filepath.Join(dir, "packager-config.yml")
	jenkinsxYaml := filepath.Join(dir, config.ProjectConfigFileName)
	envChart := filepath.Join(dir, "env/Chart.yaml")
	lpack := ""
	if len(customDraftPack) == 0 {
		if i.ProjectConfig == nil {
			i.ProjectConfig, _, err = config.LoadProjectConfig(dir)
			if err != nil {
				return "", err
			}
		}
		customDraftPack = i.ProjectConfig.BuildPack
	}

	if len(customDraftPack) > 0 {
		log.Logger().Infof("trying to use draft pack: %s", customDraftPack)
		lpack = filepath.Join(packsDir, customDraftPack)
		f, err := util.DirExists(lpack)
		if err != nil {
			log.Logger().Error(err.Error())
			return "", err
		}
		if f == false {
			log.Logger().Error("Could not find pack: " + customDraftPack + " going to try detect which pack to use")
			lpack = ""
		}
	}

	if len(lpack) == 0 {
		if exists, err := util.FileExists(pomName); err == nil && exists {
			pack, err := util.PomFlavour(pomName)
			if err != nil {
				return "", err
			}
			lpack = filepath.Join(packsDir, pack)

			exists, _ = util.DirExists(lpack)
			if !exists {
				log.Logger().Warn("defaulting to maven pack")
				lpack = filepath.Join(packsDir, "maven")
			}
		} else if exists, err := util.FileExists(gradleName); err == nil && exists {
			lpack = filepath.Join(packsDir, "gradle")
		} else if exists, err := util.FileExists(jenkinsPluginsName); err == nil && exists {
			lpack = filepath.Join(packsDir, "jenkins")
		} else if exists, err := util.FileExists(packagerConfigName); err == nil && exists {
			lpack = filepath.Join(packsDir, "cwp")
		} else if exists, err := util.FileExists(envChart); err == nil && exists {
			lpack = filepath.Join(packsDir, "environment")
		} else {
			// pack detection time
			lpack, err = jxdraft.DoPackDetectionForBuildPack(o.Out, dir, packsDir)

			if err != nil {
				if lpack == "" {
					// lets detect docker and/or helm

					// TODO one day when our pipelines can include steps conditional on the presence of a file glob
					// we can just use a single docker/helm package that does docker and/or helm
					// but for now we've 3 separate packs for docker, docker-helm and helm
					hasDocker := false
					hasHelm := false

					if exists, err2 := util.FileExists(filepath.Join(dir, "Dockerfile")); err2 == nil && exists {
						hasDocker = true
					}

					// lets check for a helm pack
					files, err2 := filepath.Glob(filepath.Join(dir, "charts/*/Chart.yaml"))
					if err2 != nil {
						return "", errors.Wrapf(err, "failed to detect if there was a chart file in dir %s", dir)
					}
					if len(files) == 0 {
						files, err2 = filepath.Glob(filepath.Join(dir, "*/Chart.yaml"))
						if err2 != nil {
							return "", errors.Wrapf(err, "failed to detect if there was a chart file in dir %s", dir)
						}
					}
					if len(files) > 0 {
						hasHelm = true
					}

					if hasDocker {
						if hasHelm {
							lpack = filepath.Join(packsDir, "docker-helm")
							err = nil
						} else {
							lpack = filepath.Join(packsDir, "docker")
							err = nil
						}
					} else if hasHelm {
						lpack = filepath.Join(packsDir, "helm")
						err = nil
					}
				}
				if lpack == "" {
					// lets check for custom jenkinsfile build pack
					exists, err2 := util.FileExists(filepath.Join(dir, jenkinsfile.Name))
					if exists && err2 == nil {
						i.CreateJenkinsxYamlIfMissing = true
						disableJenkinsfileCheck = false
						backupJenkinsfile = false
						jenkinsfilePath = defaultJenkinsfile
						lpack = filepath.Join(packsDir, "custom-jenkins")
						err = nil
					}
				}
				if err != nil {
					return "", err
				}
			}
		}
	}
	log.Logger().Infof("selected pack: %s", lpack)
	draftPack := filepath.Base(lpack)
	i.CustomDraftPack = draftPack

	if i.DisableAddFiles {
		return draftPack, nil
	}

	chartsDir := filepath.Join(dir, "charts")
	jenkinsxYamlExists, err := util.FileExists(jenkinsxYaml)
	if err != nil {
		return draftPack, err
	}

	jenkinsfileExists, err := util.FileExists(jenkinsfilePath)
	if err != nil {
		return draftPack, err
	}
	exists, err := util.DirExists(chartsDir)
	if exists && err == nil {
		exists, err = util.FileExists(filepath.Join(dir, "Dockerfile"))
		if exists && err == nil {
			if jenkinsfileExists || disableJenkinsfileCheck {
				log.Logger().Warn("existing Dockerfile, Jenkinsfile and charts folder found so skipping 'draft create' step")
				return draftPack, nil
			}
		}
	}

	generateJenkinsPath := jenkinsfilePath
	jenkinsfileBackup := ""
	defaultJenkinsfileExists, err := util.FileExists(defaultJenkinsfile)
	if defaultJenkinsfileExists && !disableJenkinsfileCheck && backupJenkinsfile {
		// lets copy the old Jenkinsfile in case we override it
		jenkinsfileBackup = defaultJenkinsfile + jenkinsfile.BackupSuffix
		err = util.RenameFile(defaultJenkinsfile, jenkinsfileBackup)
		if err != nil {
			return "", fmt.Errorf("Failed to rename old Jenkinsfile: %s", err)
		}
		generateJenkinsPath = defaultJenkinsfile
	}

	err = buildpacks.CopyBuildPack(dir, lpack)
	if err != nil {
		log.Logger().Warnf("Failed to apply the build pack in %s due to %s", dir, err)
	}

	// lets delete empty charts dir if a draft pack created one
	exists, err = util.DirExists(chartsDir)
	if err == nil && exists {
		files, err := ioutil.ReadDir(chartsDir)
		if err != nil {
			return draftPack, errors.Wrapf(err, "failed to read charts dir %s", chartsDir)
		}
		if len(files) == 0 {
			err = os.Remove(chartsDir)
			if err != nil {
				return draftPack, errors.Wrapf(err, "failed to remove empty charts dir %s", chartsDir)
			}
		}
	}

	if !jenkinsxYamlExists && i.CreateJenkinsxYamlIfMissing {
		pipelineConfig, err := config.LoadProjectConfigFile(jenkinsxYaml)
		if err != nil {
			return draftPack, err
		}
		if pipelineConfig.BuildPack != draftPack {
			pipelineConfig.BuildPack = draftPack
			err = pipelineConfig.SaveConfig(jenkinsxYaml)
			if err != nil {
				return draftPack, err
			}
		}
	}

	if !i.UseNextGenPipeline {
		if !jenkinsfileExists || jenkinsfileBackup != "" {
			// lets check if we have a pipeline.yaml in the build pack so we can generate one dynamically
			pipelineFile := filepath.Join(lpack, jenkinsfile.PipelineConfigFileName)
			exists, err := util.FileExists(pipelineFile)
			if err != nil {
				return draftPack, err
			}
			if exists {
				modules, err := gitresolver.LoadModules(packsDir)
				if err != nil {
					return draftPack, err
				}

				// lets find the Jenkinsfile template
				tmplFileName := jenkinsfile.PipelineTemplateFileName
				templateFileNames := []string{filepath.Join(lpack, tmplFileName), filepath.Join(packsDir, tmplFileName)}

				moduleResolver, err := gitresolver.ResolveModules(modules, o.Git())
				if err != nil {
					return draftPack, err
				}
				for _, mr := range moduleResolver.Modules {
					templateFileNames = append(templateFileNames, filepath.Join(mr.PacksDir, draftPack, tmplFileName), filepath.Join(mr.PacksDir, tmplFileName))
				}
				templateFile, err := util.FirstFileExists(templateFileNames...)
				if err != nil {
					return draftPack, err
				}
				prow, err := o.IsProw()
				if err != nil {
					return draftPack, err
				}

				if templateFile != "" {
					arguments := &jenkinsfile.CreateJenkinsfileArguments{
						ConfigFile:          pipelineFile,
						TemplateFile:        templateFile,
						OutputFile:          generateJenkinsPath,
						IsTekton:            prow,
						ClearContainerNames: prow,
					}
					err = arguments.GenerateJenkinsfile(moduleResolver.AsImportResolver())
					if err != nil {
						return draftPack, err
					}
				}
			}
		}

		unpackedDefaultJenkinsfile := defaultJenkinsfile
		if unpackedDefaultJenkinsfile != jenkinsfilePath {
			unpackedDefaultJenkinsfileExists := false
			unpackedDefaultJenkinsfileExists, err = util.FileExists(unpackedDefaultJenkinsfile)
			if unpackedDefaultJenkinsfileExists {
				err = util.RenameFile(unpackedDefaultJenkinsfile, jenkinsfilePath)
				if err != nil {
					return "", fmt.Errorf("Failed to rename Jenkinsfile file from '%s' to '%s': %s", unpackedDefaultJenkinsfile, jenkinsfilePath, err)
				}
				if jenkinsfileBackup != "" {
					err = util.RenameFile(jenkinsfileBackup, defaultJenkinsfile)
					if err != nil {
						return "", fmt.Errorf("Failed to rename Jenkinsfile backup file: %s", err)
					}
				}
			}
		} else if jenkinsfileBackup != "" {
			// if there's no Jenkinsfile created then rename it back again!
			jenkinsfileExists, err = util.FileExists(jenkinsfilePath)
			if err != nil {
				log.Logger().Warnf("Failed to check for Jenkinsfile %s", err)
			} else {
				if jenkinsfileExists {
					if !initialisedGit && !withRename {
						err = os.Remove(jenkinsfileBackup)
						if err != nil {
							log.Logger().Warnf("Failed to remove Jenkinsfile backup %s", err)
						}
					}
				} else {
					// lets put the old one back again
					err = util.RenameFile(jenkinsfileBackup, jenkinsfilePath)
					if err != nil {
						return "", fmt.Errorf("Failed to rename Jenkinsfile backup file: %s", err)
					}
				}
			}
		}
	}
	return draftPack, nil
}

// DiscoverBuildPack discovers the build pack given the build pack configuration
func (o *CommonOptions) DiscoverBuildPack(dir string, projectConfig *config.ProjectConfig, packConfig string) (string, error) {
	if packConfig != "" {
		return packConfig, nil
	}
	args := &InvokeDraftPack{
		Dir:             dir,
		CustomDraftPack: packConfig,
		ProjectConfig:   projectConfig,
		DisableAddFiles: true,
	}
	pack, err := o.InvokeDraftPack(args)
	if err != nil {
		return pack, errors.Wrapf(err, "failed to discover task pack in dir %s", dir)
	}
	return pack, nil
}
