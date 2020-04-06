package create

import (
	"fmt"
	"os"

	"github.com/jenkins-x/jx/pkg/cmd/importcmd"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/spring"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	createSpringLong = templates.LongDesc(`
		Creates a new Spring Boot application and then optionally setups CI/CD pipelines and GitOps promotion.

		You can see a demo of this command here: [https://jenkins-x.io/demos/create_spring/](https://jenkins-x.io/demos/create_spring/)

		For more documentation see: [https://jenkins-x.io/developing/create-spring/](https://jenkins-x.io/developing/create-spring/)

` + helper.SeeAlsoText("jx create project"))

	createSpringExample = templates.Examples(`
		# Create a Spring Boot application where you use the terminal to pick the values
		jx create spring

		# Creates a Spring Boot application passing in the required dependencies
		jx create spring -d web -d actuator

		# To pick the advanced options (such as what package type maven-project/gradle-project) etc then use
		jx create spring -x

		# To create a gradle project use:
		jx create spring --type gradle-project
	`)
)

// CreateSpringOptions the options for the create spring command
type CreateSpringOptions struct {
	CreateProjectOptions

	Advanced   bool
	SpringForm spring.SpringBootForm
}

// NewCmdCreateSpring creates a command object for the "create" command
func NewCmdCreateSpring(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateSpringOptions{
		CreateProjectOptions: CreateProjectOptions{
			ImportOptions: importcmd.ImportOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "spring",
		Short:   "Create a new Spring Boot application and import the generated code into Git and Jenkins for CI/CD",
		Long:    createSpringLong,
		Example: createSpringExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addCreateAppFlags(cmd)

	cmd.Flags().BoolVarP(&options.Advanced, "advanced", "x", false, "Advanced mode can show more detailed forms for some resource kinds like springboot")

	cmd.Flags().StringArrayVarP(&options.SpringForm.DependencyKinds, spring.OptionDependencyKind, "k", spring.DefaultDependencyKinds, "Default dependency kinds to choose from")
	cmd.Flags().StringArrayVarP(&options.SpringForm.Dependencies, spring.OptionDependency, "d", []string{}, "Spring Boot dependencies")
	cmd.Flags().StringVarP(&options.SpringForm.GroupId, spring.OptionGroupId, "g", "", "Group ID to generate")
	cmd.Flags().StringVarP(&options.SpringForm.ArtifactId, spring.OptionArtifactId, "a", "", "Artifact ID to generate")
	cmd.Flags().StringVarP(&options.SpringForm.Language, spring.OptionLanguage, "l", "", "Language to generate")
	cmd.Flags().StringVarP(&options.SpringForm.BootVersion, spring.OptionBootVersion, "t", "", "Spring Boot version")
	cmd.Flags().StringVarP(&options.SpringForm.JavaVersion, spring.OptionJavaVersion, "j", "", "Java version")
	cmd.Flags().StringVarP(&options.SpringForm.Packaging, spring.OptionPackaging, "p", "", "Packaging")
	cmd.Flags().StringVarP(&options.SpringForm.Type, spring.OptionType, "", "", "Project Type (such as maven-project or gradle-project)")

	return cmd
}

// Run implements the command
func (o *CreateSpringOptions) Run() error {
	cacheDir, err := util.CacheDir()
	if err != nil {
		return err
	}

	data := &o.SpringForm

	var details *gits.CreateRepoData

	if !o.BatchMode {
		details, err = o.GetGitRepositoryDetails()
		if err != nil {
			return err
		}

		data.ArtifactId = details.RepoName
	}

	model, err := spring.LoadSpringBoot(cacheDir)
	if err != nil {
		return fmt.Errorf("Failed to load Spring Boot model %s", err)
	}
	err = model.CreateSurvey(&o.SpringForm, o.Advanced, o.BatchMode)
	if err != nil {
		return err
	}

	// always add in actuator as its required for health checking
	if !util.Contains(o.SpringForm.Dependencies, "actuator") {
		o.SpringForm.Dependencies = append(o.SpringForm.Dependencies, "actuator")
	}
	// always add web as the JVM tends to terminate if its not added
	if !util.Contains(o.SpringForm.Dependencies, "web") {
		o.SpringForm.Dependencies = append(o.SpringForm.Dependencies, "web")
	}

	dir := o.OutDir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	outDir, err := data.CreateProject(dir)
	if err != nil {
		return err
	}
	log.Logger().Infof("Created Spring Boot project at %s", util.ColorInfo(outDir))

	if details != nil {
		o.ConfigureImportOptions(details)
	}

	return o.ImportCreatedProject(outDir)
}
