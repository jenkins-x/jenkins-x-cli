package create

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/runner"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/nodes"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var (
	create_git_token_long = templates.LongDesc(`
		Creates a new API Token for a user on a Git Server
`)

	create_git_token_example = templates.Examples(`
		# Add a new API Token for a user for the local Git server
        # prompting the user to find and enter the API Token
		jx create git token -n local someUserName

		# Add a new API Token for a user for the local Git server 
 		# using browser automation to login to the Git server
		# with the username and password to find the API Token
		jx create git token -n local -p somePassword someUserName	
	`)
)

// CreateGitTokenOptions the command line options for the command
type CreateGitTokenOptions struct {
	options.CreateOptions

	ServerFlags opts.ServerFlags
	Username    string
	Password    string
	ApiToken    string
	Timeout     string
}

// NewCmdCreateGitToken creates a command
func NewCmdCreateGitToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateGitTokenOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token [username]",
		Short:   "Adds a new API token for a user on a Git server",
		Aliases: []string{"api-token"},
		Long:    create_git_token_long,
		Example: create_git_token_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.ServerFlags.AddGitServerFlags(cmd)
	cmd.Flags().StringVarP(&options.ApiToken, "api-token", "t", "", "The API Token for the user")
	cmd.Flags().StringVarP(&options.Password, "password", "p", "", "The User password to try automatically create a new API Token")
	cmd.Flags().StringVarP(&options.Timeout, "timeout", "", "", "The timeout if using browser automation to generate the API token (by passing username and password)")

	return cmd
}

// Run implements the command
func (o *CreateGitTokenOptions) Run() error {
	args := o.Args
	if len(args) > 0 {
		o.Username = args[0]
	}
	if len(args) > 1 {
		o.ApiToken = args[1]
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	server, err := o.FindGitServer(config, &o.ServerFlags)
	if err != nil {
		return err
	}
	err = o.EnsureGitServiceCRD(server)
	if err != nil {
		return err
	}

	// TODO add the API thingy...
	if o.Username == "" {
		return fmt.Errorf("No Username specified")
	}

	userAuth := config.GetOrCreateUserAuth(server.URL, o.Username)
	if o.ApiToken != "" {
		userAuth.ApiToken = o.ApiToken
	}

	tokenUrl := gits.ProviderAccessTokenURL(server.Kind, server.URL, userAuth.Username)

	if userAuth.IsInvalid() && o.Password != "" {
		err = o.tryFindAPITokenFromBrowser(tokenUrl, userAuth)
	}

	if err != nil || userAuth.IsInvalid() {
		f := func(username string) error {
			tokenUrl := gits.ProviderAccessTokenURL(server.Kind, server.URL, username)

			log.Logger().Infof("Please generate an API Token for %s server %s", server.Kind, server.Label())
			log.Logger().Infof("Click this URL %s\n", util.ColorInfo(tokenUrl))
			log.Logger().Infof("Then COPY the token and enter in into the form below:\n")
			return nil
		}

		err = config.EditUserAuth(server.Label(), userAuth, o.Username, false, o.BatchMode, f, o.GetIOFileHandles())
		if err != nil {
			return err
		}
		if userAuth.IsInvalid() {
			return fmt.Errorf("You did not properly define the user authentication!")
		}
	}

	config.CurrentServer = server.URL
	err = authConfigSvc.SaveConfig()
	if err != nil {
		return err
	}

	log.Logger().Infof("Created user %s API Token for Git server %s at %s",
		util.ColorInfo(o.Username), util.ColorInfo(server.Name), util.ColorInfo(server.URL))

	return nil
}

// lets try use the users browser to find the API token
func (o *CreateGitTokenOptions) tryFindAPITokenFromBrowser(tokenUrl string, userAuth *auth.UserAuth) error {
	var ctxt context.Context
	var cancel context.CancelFunc
	if o.Timeout != "" {
		duration, err := time.ParseDuration(o.Timeout)
		if err != nil {
			return err
		}
		ctxt, cancel = context.WithTimeout(context.Background(), duration)
	} else {
		ctxt, cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	log.Logger().Infof("Trying to generate an API token for user: %s", util.ColorInfo(userAuth.Username))

	c, err := o.createChromeClient(ctxt)
	if err != nil {
		return err
	}

	err = c.Run(ctxt, chromedp.Tasks{
		chromedp.Navigate(tokenUrl),
	})
	if err != nil {
		return err
	}

	nodeSlice := []*cdp.Node{}
	err = c.Run(ctxt, chromedp.Nodes("//input", &nodeSlice))
	if err != nil {
		return err
	}

	login := false
	for _, node := range nodeSlice {
		name := node.AttributeValue("name")
		if name == "user_name" {
			login = true
		}
	}

	if login {
		o.captureScreenshot(ctxt, c, "screenshot-git-login.png", "//div")

		log.Logger().Infof("logging in")
		err = c.Run(ctxt, chromedp.Tasks{
			chromedp.WaitVisible("user_name", chromedp.ByID),
			chromedp.SendKeys("user_name", userAuth.Username, chromedp.ByID),
			chromedp.SendKeys("password", o.Password+"\n", chromedp.ByID),
		})
		if err != nil {
			return err
		}
	}

	o.captureScreenshot(ctxt, c, "screenshot-git-api-token.png", "//div")

	log.Logger().Info("Generating new token")

	tokenId := "jx-" + string(uuid.NewUUID())
	generateNewTokenButtonSelector := "//div[normalize-space(text())='Generate New Token']" // #nosec

	tokenResultDivSelector := "//div[@class='ui info message']/p" // #nosec
	err = c.Run(ctxt, chromedp.Tasks{
		chromedp.WaitVisible(generateNewTokenButtonSelector),
		chromedp.Click(generateNewTokenButtonSelector),
		chromedp.WaitVisible("name", chromedp.ByID),
		chromedp.SendKeys("name", tokenId+"\n", chromedp.ByID),
		chromedp.WaitVisible(tokenResultDivSelector),
		chromedp.Nodes(tokenResultDivSelector, &nodeSlice),
	})
	if err != nil {
		return err
	}
	token := ""
	for _, node := range nodeSlice {
		text := nodes.NodeText(node)
		if text != "" && token == "" {
			token = text
			break
		}
	}
	log.Logger().Info("Found API Token")
	if token != "" {
		userAuth.ApiToken = token
	}

	err = c.Shutdown(ctxt)
	if err != nil {
		return err
	}
	return nil
}

// lets try use the users browser to find the API token
func (o *CreateGitTokenOptions) createChromeClient(ctxt context.Context) (*chromedp.CDP, error) {
	if o.BatchMode {
		options := func(m map[string]interface{}) error {
			m["remote-debugging-port"] = 9222
			m["no-sandbox"] = true
			m["headless"] = true
			return nil
		}

		return chromedp.New(ctxt, chromedp.WithRunnerOptions(runner.CommandLineOption(options)))
	}
	return chromedp.New(ctxt)
}

func (o *CreateGitTokenOptions) captureScreenshot(ctxt context.Context, c *chromedp.CDP, screenshotFile string, selector interface{}, options ...chromedp.QueryOption) error {
	log.Logger().Info("Creating a screenshot...")

	var picture []byte
	err := c.Run(ctxt, chromedp.Tasks{
		chromedp.Sleep(2 * time.Second),
		chromedp.Screenshot(selector, &picture, options...),
	})
	if err != nil {
		return err
	}
	log.Logger().Info("Saving a screenshot...")

	err = ioutil.WriteFile(screenshotFile, picture, util.DefaultWritePermissions)
	if err != nil {
		log.Logger().Fatal(err.Error())
	}

	log.Logger().Infof("Saved screenshot: %s", util.ColorInfo(screenshotFile))
	return err
}
