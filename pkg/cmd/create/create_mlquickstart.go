package create

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/importcmd"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/quickstarts"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

const (
	// JenkinsXMLQuickstartsOrganisation is the default organisation for machine-learning quickstarts
	JenkinsXMLQuickstartsOrganisation = "machine-learning-quickstarts"
)

var (
	// DefaultMLQuickstartLocation is the default organisation for machine-learning quickstarts
	DefaultMLQuickstartLocation = v1.QuickStartLocation{
		GitURL:   gits.GitHubURL,
		GitKind:  gits.KindGitHub,
		Owner:    JenkinsXMLQuickstartsOrganisation,
		Includes: []string{"ML-*"},
		Excludes: []string{"WIP-*"},
	}
)

var (
	createMLQuickstartLong = templates.LongDesc(`
		Create a new machine learning project from a sample/starter (found in https://github.com/machine-learning-quickstarts)

		This will create two new projects for you from the selected template. One for training and one for deploying a model as a service.
		It will exclude any work-in-progress repos (containing the "WIP-" pattern)

		For more documentation see: [https://jenkins-x.io/developing/create-mlquickstart/](https://jenkins-x.io/developing/create-mlquickstart/)

` + helper.SeeAlsoText("jx create project"))

	createMLQuickstartExample = templates.Examples(`
		Create a new machine learning project from a sample/starter (found in https://github.com/machine-learning-quickstarts)

		This will create a new machine learning project for you from the selected template.
		It will exclude any work-in-progress repos (containing the "WIP-" pattern)

		jx create mlquickstart

		jx create mlquickstart -f pytorch
	`)
)

// CreateMLQuickstartOptions the options for the create quickstart command
type CreateMLQuickstartOptions struct {
	CreateProjectOptions

	GitHubOrganisations []string
	Filter              quickstarts.QuickstartFilter
	GitProvider         gits.GitProvider
	GitHost             string
	IgnoreTeam          bool
}

type projectset struct {
	Repo string
	Tail string
}

// NewCmdCreateMLQuickstart creates a command object for the "create" command
func NewCmdCreateMLQuickstart(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateMLQuickstartOptions{
		CreateProjectOptions: CreateProjectOptions{
			ImportOptions: importcmd.ImportOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "mlquickstart",
		Short:   "Create a new machine learning app from a set of quickstarts and import the generated code into Git and Jenkins for CI/CD",
		Long:    createMLQuickstartLong,
		Example: createMLQuickstartExample,
		Aliases: []string{"arch"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addCreateAppFlags(cmd)

	cmd.Flags().StringArrayVarP(&options.GitHubOrganisations, "organisations", "g", []string{}, "The GitHub organisations to query for quickstarts")
	cmd.Flags().StringArrayVarP(&options.Filter.Tags, "tag", "t", []string{}, "The tags on the quickstarts to filter")
	cmd.Flags().StringVarP(&options.Filter.Owner, "owner", "", "", "The owner to filter on")
	cmd.Flags().StringVarP(&options.Filter.Language, "language", "l", "", "The language to filter on")
	cmd.Flags().StringVarP(&options.Filter.Framework, "framework", "", "", "The framework to filter on")
	cmd.Flags().StringVarP(&options.GitHost, "git-host", "", "", "The Git server host if not using GitHub when pushing created project")
	cmd.Flags().StringVarP(&options.Filter.Text, "filter", "f", "", "The text filter")
	cmd.Flags().StringVarP(&options.Filter.ProjectName, "project-name", "p", "", "The project name (for use with -b batch mode)")
	return cmd
}

// Run implements the generic Create command
func (o *CreateMLQuickstartOptions) Run() error {
	log.Logger().Debugf("Running CreateMLQuickstart...")

	interactive := true
	if o.BatchMode {
		interactive = false
		log.Logger().Debugf("In batch mode.")
	}

	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	var locations []v1.QuickStartLocation
	if !o.IgnoreTeam {
		jxClient, ns, err := o.JXClientAndDevNamespace()
		if err != nil {
			return err
		}

		locations, err = kube.GetQuickstartLocations(jxClient, ns)
		if err != nil {
			return err
		}
		foundDefault := false
		for _, location := range locations {
			if isMLRepo(location) {
				log.Logger().Debugf("Location: %s ", location)
			} else {
				// Protect generic quickstart repos from
			}
			if location.GitURL == gits.GitHubURL && location.Owner == JenkinsXMLQuickstartsOrganisation {
				foundDefault = true
			}
		}

		// Add the default MLQuickstarts repo if it is missing
		if !foundDefault {
			locations = append(locations, DefaultMLQuickstartLocation)

			callback := func(env *v1.Environment) error {
				env.Spec.TeamSettings.QuickstartLocations = locations
				log.Logger().Infof("Adding the default ml quickstart repo %s", util.ColorInfo(util.UrlJoin(DefaultMLQuickstartLocation.GitURL, DefaultMLQuickstartLocation.Owner)))
				return nil
			}
			o.ModifyDevEnvironment(callback)
		}
	}

	// lets add any extra github organisations from the CLI if they are not already configured
	for _, org := range o.GitHubOrganisations {
		found := false
		for _, loc := range locations {
			if loc.GitURL == gits.GitHubURL && loc.Owner == org {
				found = true
				break
			}
		}
		if !found {
			locations = append(locations, v1.QuickStartLocation{
				GitURL:   gits.GitHubURL,
				GitKind:  gits.KindGitHub,
				Owner:    org,
				Includes: []string{"ML-*"},
				Excludes: []string{"WIP-*"},
			})
		}
	}

	gitMap := map[string]map[string]v1.QuickStartLocation{}
	for _, loc := range locations {
		m := gitMap[loc.GitURL]
		if m == nil {
			m = map[string]v1.QuickStartLocation{}
			gitMap[loc.GitURL] = m
		}
		m[loc.Owner] = loc

	}

	var details *gits.CreateRepoData

	if !o.BatchMode {
		details, err = o.GetGitRepositoryDetails()
		if err != nil {
			return err
		}

		o.Filter.ProjectName = details.RepoName
	}

	o.Filter.AllowML = true

	model, err := o.LoadQuickstartsFromMap(config, gitMap)
	if err != nil {
		return fmt.Errorf("failed to load quickstarts: %s", err)
	}
	var q *quickstarts.QuickstartForm
	if o.BatchMode {
		q, err = pickMLProject(model, &o.Filter, o.BatchMode)
	} else {
		q, err = model.CreateSurvey(&o.Filter, o.BatchMode, o.GetIOFileHandles())
	}

	if err != nil {
		return err
	}
	if q == nil {
		return fmt.Errorf("no quickstart chosen")
	}

	dir := o.OutDir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	w := &CreateQuickstartOptions{}
	w.CreateProjectOptions = o.CreateProjectOptions
	w.CommonOptions = o.CommonOptions
	w.ImportOptions = o.ImportOptions
	w.GitHubOrganisations = o.GitHubOrganisations
	w.Filter = o.Filter
	w.Filter.Text = q.Quickstart.Name
	w.GitProvider = o.GitProvider
	w.GitHost = o.GitHost
	w.IgnoreTeam = o.IgnoreTeam

	w.BatchMode = true

	// Check to see if the selection is a project set
	ps, err := o.getMLProjectSet(q.Quickstart)

	var e error
	if err == nil {
		// We have a projectset so create all the associated quickstarts
		stub := o.Filter.ProjectName
		for _, project := range ps {
			w.ImportOptions = o.ImportOptions // Reset the options each time as they are modified by Import (DraftPack)
			if interactive {
				log.Logger().Debugf("Setting Quickstart from surveys.")
				w.ImportOptions.Organisation = details.Organisation
				w.GitRepositoryOptions = o.GitRepositoryOptions
				w.GitRepositoryOptions.ServerURL = details.GitServer.URL
				w.GitRepositoryOptions.ServerKind = details.GitServer.Kind
				w.GitRepositoryOptions.Username = details.User.Username
				w.GitRepositoryOptions.ApiToken = details.User.ApiToken
				w.GitRepositoryOptions.Owner = details.Organisation
				w.GitRepositoryOptions.Public = details.Public
				w.GitProvider = details.GitProvider
				w.GitServer = details.GitServer
			}
			w.Filter.Text = project.Repo
			w.Filter.ProjectName = stub + project.Tail
			w.Filter.Language = ""
			log.Logger().Debugf("Invoking CreateQuickstart for %s...", project.Repo)

			e = w.Run()

			if e != nil {
				return e
			}
		}
	} else {
		// Must be a conventional quickstart
		log.Logger().Debugf("Invoking CreateQuickstart...")
		return w.Run()
	}

	return e

}

// Pairs of Training and Service projects can be declared by creating a dedicated repository that shares the same root name as the -Training and -Service repositories
// but which contains only a 'projectset' file that specifies the names of the associated projects.
// Selecting the projectset project as a quickstart automatically creates both related -Training and -Service projects with a common name prefix.
func (o *CreateMLQuickstartOptions) getMLProjectSet(q *quickstarts.Quickstart) ([]projectset, error) {
	var ps []projectset

	// Look at https://raw.githubusercontent.com/:owner/:repo/master/projectset
	client := http.Client{}
	u := "https://raw.githubusercontent.com/" + q.Owner + "/" + q.Name + "/master/projectset"

	req, err := http.NewRequest(http.MethodGet, u, strings.NewReader(""))
	if err != nil {
		log.Logger().Debugf("Projectset not found because %+#v", err)
		return nil, err
	}
	gitProvider := q.GitProvider
	if gitProvider != nil {
		userAuth := gitProvider.UserAuth()
		token := userAuth.ApiToken
		username := userAuth.Username
		if token != "" && username != "" {
			log.Logger().Debugf("Downloading project zip from %s with basic auth for user: %s", u, username)
			req.SetBasicAuth(username, token)
		}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &ps)
	return ps, err
}

// LoadQuickstartsFromMap Load all quickstarts
func (o *CreateMLQuickstartOptions) LoadQuickstartsFromMap(config *auth.AuthConfig, gitMap map[string]map[string]v1.QuickStartLocation) (*quickstarts.QuickstartModel, error) {
	model := quickstarts.NewQuickstartModel()

	mlOnly := []string{"ML-*"} // Filter for ML repos

	for gitURL, m := range gitMap {
		for _, location := range m {
			kind := location.GitKind
			if kind == "" {
				kind = gits.KindGitHub
			}
			gitProvider, err := o.GitProviderForGitServerURL(gitURL, kind, "")
			if err != nil {
				return model, err
			}
			log.Logger().Debugf("Searching for repositories in Git server %s owner %s includes %s excludes %s as user %s ", gitProvider.ServerURL(), location.Owner, strings.Join(mlOnly, ", "), strings.Join(location.Excludes, ", "), gitProvider.CurrentUsername())
			err = model.LoadGithubQuickstarts(gitProvider, location.Owner, mlOnly, location.Excludes)
			if err != nil {
				log.Logger().Debugf("Quickstart load error: %s", err.Error())
			}
		}
	}

	return model, nil
}

// PickMLProject picks a mlquickstart project set from filtered results
func pickMLProject(model *quickstarts.QuickstartModel, filter *quickstarts.QuickstartFilter, batchMode bool) (*quickstarts.QuickstartForm, error) {
	mlquickstarts := model.Filter(filter)
	names := []string{}
	m := map[string]*quickstarts.Quickstart{}
	for _, qs := range mlquickstarts {
		name := qs.SurveyName()
		m[name] = qs
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return nil, fmt.Errorf("No quickstarts match filter")
	}
	answer := ""
	// Pick the first option as this is the project set
	answer = names[0]
	if answer == "" {
		return nil, fmt.Errorf("No quickstart chosen")
	}
	q := m[answer]
	if q == nil {
		return nil, fmt.Errorf("Could not find chosen quickstart for %s", answer)
	}
	form := &quickstarts.QuickstartForm{
		Quickstart: q,
		Name:       q.Name,
	}
	return form, nil
}

// isMLRepo returns true if the git location has "ML-*" defined within Includes:
func isMLRepo(location v1.QuickStartLocation) bool {
	for _, v := range location.Includes {
		if v == "ML-*" {
			return true
		}
	}
	return false
}
