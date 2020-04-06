package verify

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	verifyRequirementsLong = templates.LongDesc(`
		Verifies all the helm requirements.yaml files have a version number populated from the Version Stream.




` + helper.SeeAlsoText("jx create project"))

	verifyRequirementsExample = templates.Examples(`
		Verifies all the helm requirements.yaml files have a version number populated from the Version Stream

		# verify packages and fail if any are not valid:
		jx step verify packages

		# override the error if the 'jx' binary is out of range (e.g. for development)
        export JX_DISABLE_VERIFY_JX="true"
		jx step verify packages
	`)
)

// StepVerifyRequirementsOptions contains the command line flags
type StepVerifyRequirementsOptions struct {
	step.StepOptions

	Dir string
}

// NewCmdStepVerifyRequirements creates the `jx step verify pod` command
func NewCmdStepVerifyRequirements(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyRequirementsOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "requirements",
		Aliases: []string{"requirement", "req"},
		Short:   "Verifies all the helm requirements.yaml files have a version number populated from the Version Stream",
		Long:    verifyRequirementsLong,
		Example: verifyRequirementsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to recursively look for 'requirements.yaml' files")

	return cmd
}

// Run implements this command
func (o *StepVerifyRequirementsOptions) Run() error {
	if o.Dir == "" {
		var err error
		o.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	requirements, _, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load boot requirements")
	}
	vs := requirements.VersionStream

	log.Logger().Debugf("Verifying the helm requirements versions in dir: %s using version stream URL: %s and git ref: %s\n", o.Dir, vs.URL, vs.Ref)

	resolver, err := o.CreateVersionResolver(vs.URL, vs.Ref)
	if err != nil {
		return errors.Wrapf(err, "failed to create version resolver")
	}

	repoPrefixes, err := resolver.GetRepositoryPrefixes()
	if err != nil {
		return errors.Wrapf(err, "failed to load repository prefixes")
	}

	err = filepath.Walk(o.Dir, func(path string, info os.FileInfo, err error) error {
		name := info.Name()
		if info.IsDir() || name != helm.RequirementsFileName {
			return nil
		}

		log.Logger().Infof("found %s", path)
		return o.verifyRequirementsYAML(resolver, repoPrefixes, path)
	})

	return err
}

func (o *StepVerifyRequirementsOptions) verifyRequirementsYAML(resolver *versionstream.VersionResolver, prefixes *versionstream.RepositoryPrefixes, fileName string) error {
	req, err := helm.LoadRequirementsFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to load %s", fileName)
	}

	modified := false
	for _, dep := range req.Dependencies {
		if dep.Version == "" {
			name := dep.Alias
			if name == "" {
				name = dep.Name
			}
			repo := dep.Repository
			if repo == "" {
				return fmt.Errorf("cannot to find a version for dependency %s in file %s as there is no 'repository'", name, fileName)
			}

			prefix := prefixes.PrefixForURL(repo)
			if prefix == "" {
				return fmt.Errorf("the helm repository %s does not have an associated prefix in in the 'charts/repositories.yml' file the version stream, so we cannot default the version in file %s", repo, fileName)
			}
			newVersion := ""
			fullChartName := prefix + "/" + dep.Name
			newVersion, err := resolver.StableVersionNumber(versionstream.KindChart, fullChartName)
			if err != nil {
				return errors.Wrapf(err, "failed to find version of chart %s in file %s", fullChartName, fileName)
			}
			if newVersion == "" {
				return fmt.Errorf("failed to find a version for dependency %s in file %s in the current version stream - please either add an explicit version to this file or add chart %s to the version stream", name, fileName, fullChartName)
			}
			dep.Version = newVersion
			modified = true
			log.Logger().Debugf("adding version %s to dependency %s in file %s", newVersion, name, fileName)
		}
	}

	if modified {
		err = helm.SaveFile(fileName, req)
		if err != nil {
			return errors.Wrapf(err, "failed to save %s", fileName)
		}
		log.Logger().Infof("adding dependency versions to file %s", fileName)
	}
	return nil
}
