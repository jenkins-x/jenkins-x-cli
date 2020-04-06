package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"

	"strconv"
	"time"

	"github.com/jenkins-x/jx/pkg/log"
)

type RateLimits struct {
	Resources RateResources `json:"resources"`
}

type RateResources struct {
	Core    Rate `json:"core"`
	Search  Rate `json:"search"`
	GraphQL Rate `json:"graphql"`
}

type Rate struct {
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
	Reset     int `json:"reset"`
}

// GetAddonOptions the command line options
type GetLimitsOptions struct {
	GetOptions
}

var (
	get_limits_long = templates.LongDesc(`
		Display the github limits for users

`)

	get_limits_example = templates.Examples(`
		# List all git users with limits
		jx get limits
	`)
)

// NewCmdGetLimits creates the command
func NewCmdGetLimits(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetLimitsOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "limits [flags]",
		Short:   "Displays the git user limits",
		Long:    get_limits_long,
		Example: get_limits_example,
		Aliases: []string{"limit"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	return cmd
}

// Run implements this command
func (o *GetLimitsOptions) Run() error {
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	table := o.CreateTable()
	table.AddRow("Name", "URL", "Username", "Limit", "Remaining", "Reset")

	for _, s := range config.Servers {
		kind := s.Kind
		if kind == "" {
			kind = "github"
		}

		if kind == "github" {
			for _, u := range s.Users {
				r, err := o.GetLimits(s.URL, u.Username, u.ApiToken)
				if err != nil {
					return err
				}

				resetLabel := ""
				if 0 != r.Resources.Core.Reset {
					secondsUntilReset := int64(r.Resources.Core.Reset) - time.Now().Unix()
					d := time.Duration(1000 * 1000 * 1000 * secondsUntilReset)
					resetLabel = d.String()
				}

				table.AddRow(s.Name, s.URL, u.Username, strconv.Itoa(r.Resources.Core.Limit), strconv.Itoa(r.Resources.Core.Remaining), resetLabel)
			}
		}

	}
	table.Render()

	return nil
}

func (o *GetLimitsOptions) GetLimits(server string, username string, apitoken string) (RateLimits, error) {
	url := fmt.Sprintf("https://%s:%s@api.github.com/rate_limit", username, apitoken)

	// Build the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Logger().Errorf("NewRequest: %s", err)
		return RateLimits{}, err
	}

	// For control over HTTP client headers,
	// redirect policy, and other settings,
	// create a Client
	// A Client is an HTTP client
	client := &http.Client{}

	// Send the request via a client
	// Do sends an HTTP request and
	// returns an HTTP response
	resp, err := client.Do(req)
	if err != nil {
		log.Logger().Errorf("Do: %s", err)
		return RateLimits{}, err
	}

	// Callers should close resp.Body
	// when done reading from it
	// Defer the closing of the body
	defer resp.Body.Close()

	// Fill the record with the data from the JSON
	var limits RateLimits

	// Use json.Decode for reading streams of JSON data
	if err := json.NewDecoder(resp.Body).Decode(&limits); err != nil {
		log.Logger().Errorf("Decode: %s", err)
	}

	return limits, nil
}
