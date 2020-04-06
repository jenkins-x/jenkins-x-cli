package github

import (
	"fmt"

	"encoding/json"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

//GithubApp represents a Github App install
type GithubApp struct {
	Factory clients.Factory
}

type response struct {
	Installed    bool
	AccessToRepo bool
	URL          string
	AppName      string
}

func (gh *GithubApp) isGithubAppEnabled() (bool, error) {
	requirementConfig, err := gh.getRequirementConfig()
	if err != nil {
		return false, err
	}
	if requirementConfig != nil && requirementConfig.GithubApp != nil {
		return requirementConfig.GithubApp.Enabled, nil
	}
	return false, nil
}

// Install - confirms that the github app is installed and if it isn't then prints out a url for the user to install
func (gh *GithubApp) Install(owner string, repo string, fileHandles util.IOFileHandles, newRepo bool) (bool, error) {
	installed, accessToRepo, url, appName, err := gh.isInstalled(owner, repo)
	if err != nil {
		return false, errors.Wrap(err, "when querying whether the Github App is installed")
	}
	if appName == "" {
		// if the appName is empty lets use Jenkins X as the default
		appName = "Jenkins X"
	}

	if installed {
		fmt.Println(fmt.Sprintf("'%s' Github App installed", util.ColorInfo(appName)))
		if newRepo {
			// if this is a new repo we can't confirm if it has access at this stage
			return false, nil
		}
		if !accessToRepo {
			fmt.Fprintf(fileHandles.Out, "Please confirm '%s' Github App has access to repository %s. Click this url \n%s\n\n", util.ColorInfo(appName), repo, util.ColorInfo(url))
		}
	} else {
		fmt.Fprintf(fileHandles.Out, "Please install '%s' Github App into your organisation/account %s and allow access to repository %s. Click this url \n%s\n\n", util.ColorInfo(appName), owner, repo, util.ColorInfo(url))
	}
	if !accessToRepo {
		accessToRepo, err = util.Confirm(fmt.Sprintf("Does the '%s' Github App have access to repository", util.ColorInfo(appName)), false,
			fmt.Sprintf("Please install '%s' Github App into your organisation and grant access to repositories", util.ColorInfo(appName)), fileHandles)
		if err != nil {
			return false, err
		}
	}
	if !accessToRepo {
		return false, errors.New(fmt.Sprintf("Please install '%s' Github App", util.ColorInfo(appName)))
	}
	return accessToRepo, err
}

func (gh *GithubApp) isInstalled(owner string, repo string) (bool, bool, string, string, error) {
	requirementConfig, err := gh.getRequirementConfig()
	if err != nil {
		return false, false, "", "", err
	}

	if requirementConfig.GithubApp != nil {
		url := requirementConfig.GithubApp.URL + "/installed/" + owner + "/" + repo

		if url != "" {
			respBody, err := util.CallWithExponentialBackOff(url, "", "GET", []byte{}, nil)
			log.Logger().Debug(string(respBody))
			if err != nil {
				return false, false, "", "", errors.Wrapf(err, "error getting response from github app via %s", url)
			}

			response := &response{}

			err = json.Unmarshal(respBody, response)

			if err != nil {
				return false, false, "", "", errors.Wrapf(err, "error marshalling response %s", url)
			}
			return response.Installed, response.AccessToRepo, response.URL, response.AppName, nil
		}
	}
	return false, false, "", "", errors.New("unable to locate github app ")
}

func (gh *GithubApp) getRequirementConfig() (*config.RequirementsConfig, error) {
	jxClient, ns, err := gh.Factory.CreateJXClient()
	if err != nil {
		log.Logger().Errorf("error creating jx client %v", err)
		return nil, err
	}

	teamSettings, err := kube.GetDevEnvTeamSettings(jxClient, ns)
	if err != nil {
		log.Logger().Errorf("error getting team settings from jx client %v", err)
		return nil, err
	}

	requirementConfig, err := config.GetRequirementsConfigFromTeamSettings(teamSettings)
	if err != nil {
		log.Logger().Errorf("error getting requirement config from team settings %v", err)
		return nil, err
	}
	return requirementConfig, nil
}
