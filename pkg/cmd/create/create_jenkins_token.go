package create

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	JenkinsCookieName    = "JSESSIONID"
	JenkinsVersionHeader = "X-Jenkins"
)

var JenkinsReferenceVersion = semver.Version{Major: 2, Minor: 140, Patch: 0}

var (
	createJEnkinsUserLong = templates.LongDesc(`
		Creates a new user and API Token for the current Jenkins server
`)

	createJEnkinsUserExample = templates.Examples(`
		# Add a new API Token for a user for the current Jenkins server
        # prompting the user to find and enter the API Token
		jx create jenkins token someUserName

		# Add a new API Token for a user for the current Jenkins server
 		# using browser automation to login to the Git server
		# with the username an password to find the API Token
		jx create jenkins token -p somePassword someUserName	
	`)
)

// CreateJenkinsUserOptions the command line options for the command
type CreateJenkinsUserOptions struct {
	options.CreateOptions

	ServerFlags     opts.ServerFlags
	JenkinsSelector opts.JenkinsSelectorOptions
	Namespace       string
	Username        string
	Password        string
	APIToken        string
	BearerToken     string
	Timeout         string
	NoREST          bool
	RecreateToken   bool
	HealthTimeout   time.Duration
}

// NewCmdCreateJenkinsUser creates a command
func NewCmdCreateJenkinsUser(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateJenkinsUserOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token [username]",
		Short:   "Adds a new username and API token for a Jenkins server",
		Aliases: []string{"api-token"},
		Long:    createJEnkinsUserLong,
		Example: createJEnkinsUserExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.ServerFlags.AddGitServerFlags(cmd)
	options.JenkinsSelector.AddFlags(cmd)

	cmd.Flags().StringVarP(&options.APIToken, "api-token", "t", "", "The API Token for the user")
	cmd.Flags().StringVarP(&options.Password, "password", "p", "", "The User password to try automatically create a new API Token")
	cmd.Flags().StringVarP(&options.Timeout, "timeout", "", "", "The timeout if using REST to generate the API token (by passing username and password)")
	cmd.Flags().BoolVarP(&options.NoREST, "no-rest", "", false, "Disables the use of REST calls to automatically find the API token if the user and password are known")
	cmd.Flags().BoolVarP(&options.RecreateToken, "recreate-token", "", false, "Should we recreate the API token if it already exists")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "The namespace of the secret where the Jenkins API token will be stored")
	cmd.Flags().DurationVarP(&options.HealthTimeout, "health-timeout", "", 30*time.Minute, "The maximum duration to wait for the Jenkins service to be healthy before trying to create the API token")
	return cmd
}

// Run implements the command
func (o *CreateJenkinsUserOptions) Run() error {
	args := o.Args
	if len(args) > 0 {
		o.Username = args[0]
	}
	if len(args) > 1 {
		o.APIToken = args[1]
	}
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return errors.Wrap(err, "connecting to Kubernetes cluster")
	}
	if o.Namespace != "" {
		ns = o.Namespace
	}

	authConfigSvc, err := o.JenkinsAuthConfigService(ns, &o.JenkinsSelector)
	if err != nil {
		return errors.Wrap(err, "creating Jenkins Auth configuration")
	}
	config := authConfigSvc.Config()

	var server *auth.AuthServer
	if o.ServerFlags.IsEmpty() {
		url, err := o.CustomJenkinsURL(&o.JenkinsSelector, kubeClient, ns)
		if err != nil {
			return err
		}
		server = config.GetOrCreateServer(url)
	} else {
		server, err = o.FindServer(config, &o.ServerFlags, "Jenkins server", "Try installing one via: jx create team", false)
		if err != nil {
			return errors.Wrapf(err, "searching server %s: %s", o.ServerFlags.ServerName, o.ServerFlags.ServerURL)
		}
	}

	if o.Username == "" {
		// lets default the user if there's only 1
		userAuths := config.FindUserAuths(server.URL)
		if len(userAuths) == 1 {
			ua := userAuths[0]
			o.Username = ua.Username
			if o.Password == "" {
				o.Password = ua.Password
			}
		}
	}
	if o.Username == "" {
		return fmt.Errorf("no Username specified")
	}

	userAuth := config.GetOrCreateUserAuth(server.URL, o.Username)

	if o.RecreateToken {
		userAuth.ApiToken = ""
		userAuth.BearerToken = ""
	} else {
		if o.APIToken != "" {
			userAuth.ApiToken = o.APIToken
		}
		if o.BearerToken != "" {
			userAuth.BearerToken = o.BearerToken
		}
	}

	if o.Password != "" {
		userAuth.Password = o.Password
	}

	if userAuth.IsInvalid() && o.Password != "" && !o.NoREST {
		err := jenkins.CheckHealth(server.URL, o.HealthTimeout)
		if err != nil {
			return errors.Wrapf(err, "checking health of Jenkins server %q", server.URL)
		}
		err = o.getAPITokenFromREST(server.URL, userAuth)
		if err != nil {
			if o.BatchMode {
				return errors.Wrapf(err, "generating the API token from REST API of server %q", server.URL)
			}
			log.Logger().Warnf("failed to generate API token from REST API of server %s due to: %s", server.URL, err.Error())
			log.Logger().Info("So unfortunately you will have to provide this by hand...\n")
		}
	}

	if userAuth.IsInvalid() {
		f := func(username string) error {
			jenkins.PrintGetTokenFromURL(o.Out, jenkins.JenkinsTokenURL(server.URL))
			log.Logger().Infof("Then COPY the token and enter in into the form below:\n")
			return nil
		}

		err = config.EditUserAuth("Jenkins", userAuth, o.Username, false, o.BatchMode, f, o.GetIOFileHandles())
		if err != nil {
			return errors.Wrapf(err, "updating the Jenkins auth configuration for user %q", o.Username)
		}
		if userAuth.IsInvalid() {
			return fmt.Errorf("you did not properly define the user authentication")
		}
	}

	config.CurrentServer = server.URL
	err = authConfigSvc.SaveConfig()
	if err != nil {
		return errors.Wrap(err, "saving the auth config")
	}

	err = o.saveJenkinsAuthInSecret(kubeClient, userAuth)
	if err != nil {
		return errors.Wrap(err, "saving the auth config in a Kubernetes secret")
	}

	log.Logger().Infof("Created user %s API Token for Jenkins server %s at %s",
		util.ColorInfo(o.Username), util.ColorInfo(server.Name), util.ColorInfo(server.URL))
	return nil
}

func (o *CreateJenkinsUserOptions) saveJenkinsAuthInSecret(kubeClient kubernetes.Interface, auth *auth.UserAuth) error {
	ns := o.Namespace
	if ns == "" {
		_, currentNamespace, err := o.KubeClientAndNamespace()
		if err != nil {
			return errors.Wrap(err, "getting the current namespace")
		}
		ns = currentNamespace
	}
	serviceName := kube.ServiceJenkins
	secretName := kube.SecretJenkins
	customJenkinsName := o.JenkinsSelector.CustomJenkinsName
	if customJenkinsName != "" {
		serviceName = customJenkinsName
		secretName = customJenkinsName + "-auth"
	}
	create := false

	secretInterface := kubeClient.CoreV1().Secrets(ns)
	secret, err := secretInterface.Get(secretName, metav1.GetOptions{})
	if err != nil {
		create = true
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
		}
	}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	if secret.Labels[kube.LabelKind] == "" {
		secret.Labels[kube.LabelKind] = kube.ValueKindJenkins
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	svc, err := kubeClient.CoreV1().Services(ns).Get(serviceName, metav1.GetOptions{})
	if err == nil && svc != nil {
		hasOwnerRef := false
		for _, ref := range secret.OwnerReferences {
			if ref.Name == svc.Name && ref.Kind == "Service" {
				hasOwnerRef = true
			}
		}
		if !hasOwnerRef {
			secret.OwnerReferences = append(secret.OwnerReferences, kube.ServiceOwnerRef(svc))
		}
	} else {
		log.Logger().Warnf("Could not find service %s in namespace %s: %v", serviceName, ns, err)
	}

	secret.Data[kube.JenkinsAdminApiToken] = []byte(auth.ApiToken)
	secret.Data[kube.JenkinsBearTokenField] = []byte(auth.BearerToken)
	secret.Data[kube.JenkinsAdminUserField] = []byte(auth.Username)

	if create {
		_, err = secretInterface.Create(secret)
		if err != nil {
			return errors.Wrapf(err, "creating the Jenkins auth configuration in secret %s/%s", ns, secretName)
		}
		return nil
	}
	_, err = secretInterface.Update(secret)
	if err != nil {
		return errors.Wrapf(err, "updating the Jenkins auth configuration in secret %s/%s", ns, secretName)
	}
	return nil
}

// Uses Jenkins REST(ish) calls to obtain an API token given a username and password.
func (o *CreateJenkinsUserOptions) getAPITokenFromREST(serverURL string, userAuth *auth.UserAuth) error {
	var ctx context.Context
	var cancel context.CancelFunc
	if o.Timeout != "" {
		duration, err := time.ParseDuration(o.Timeout)
		if err != nil {
			return errors.Wrap(err, "parsing the timeout value")
		}
		ctx, cancel = context.WithTimeout(context.Background(), duration)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	log.Logger().Info("Generating the API token...")
	decorator, err := loginLegacy(ctx, serverURL, o.Verbose, userAuth.Username, o.Password)
	if err != nil {
		// Might be a modern realm, which would normally support BasicHeaderRealPasswordAuthenticator.
		decorator = func(req *http.Request) {
			req.SetBasicAuth(userAuth.Username, o.Password)
		}
		err2 := verifyLogin(ctx, serverURL, o.Verbose, decorator)
		if err2 != nil {
			// That did not work either.
			log.Logger().Warnf("Failed to log in via modern security realm: %s", err2)
			return errors.Wrap(err, "logging in")
		}
		log.Logger().Infof("Logged in %s to Jenkins server at %s via modern security realm",
			util.ColorInfo(username), util.ColorInfo(serverURL))
	}
	decorator = checkForCrumb(ctx, serverURL, o.Verbose, decorator)
	token, err := generateNewAPIToken(ctx, serverURL, o.Verbose, decorator)
	if err != nil {
		return errors.Wrap(err, "generating the API token")
	}
	if token == "" {
		return errors.New("received an empty API token")
	}
	userAuth.ApiToken = token
	return nil
}

// Try logging in as if LegacySecurityRealm were configured. This uses the old Servlet API login cookies.
func loginLegacy(ctx context.Context, serverURL string, verbose bool, username string, password string) (func(req *http.Request), error) {
	client := http.Client{
		// https://stackoverflow.com/a/38150816/12916 Jenkins returns a 303, but you cannot actually follow it
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodPost, util.UrlJoin(serverURL, fmt.Sprintf("/j_security_check?j_username=%s&j_password=%s",
		url.QueryEscape(username), url.QueryEscape(password))), nil)
	if err != nil {
		return nil, errors.Wrap(err, "building request to log in")
	}
	req = req.WithContext(ctx)
	if verbose {
		req.Write(os.Stderr)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "execute log in")
	}
	defer resp.Body.Close()
	cookies := resp.Cookies()
	decorator := func(req *http.Request) {
		for _, c := range cookies {
			req.AddCookie(c)
		}
	}
	// We get the same response even if Jenkins is actually using a modern security realm, so verify it:
	err = verifyLogin(ctx, serverURL, verbose, decorator)
	if err != nil {
		return nil, errors.Wrap(err, "cookies did not work; bad login or not using legacy security realm")
	}
	log.Logger().Infof("Logged in %s to Jenkins server at %s via legacy security realm",
		util.ColorInfo(username), util.ColorInfo(serverURL))
	return decorator, nil
}

// Checks whether a purported login decorator actually seems to work.
func verifyLogin(ctx context.Context, serverURL string, verbose bool, decorator func(req *http.Request)) error {
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, util.UrlJoin(serverURL, "/me/api/json?tree=id"), nil)
	if err != nil {
		return errors.Wrap(err, "building request to verify login")
	}
	req = req.WithContext(ctx)
	decorator(req)
	if verbose {
		req.Write(os.Stderr)
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "execute verify login")
	}
	defer resp.Body.Close()
	if verbose {
		resp.Write(os.Stderr)
	}
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

// Checks if CSRF defense is enabled, and if so, amends the decorator to include a crumb.
func checkForCrumb(ctx context.Context, serverURL string, verbose bool, decorator func(req *http.Request)) func(req *http.Request) {
	client := http.Client{}
	req, err := http.NewRequest(http.MethodGet, util.UrlJoin(serverURL, "/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)"), nil)
	if err != nil {
		log.Logger().Warnf("Failed to build request to check for crumb: %s", err)
		return decorator
	}
	req = req.WithContext(ctx)
	decorator(req)
	if verbose {
		req.Write(os.Stderr)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Logger().Warnf("Failed to execute request to check for crumb: %s", err)
		return decorator
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		log.Logger().Infof("Enable CSRF protection at: %s/configureSecurity/", serverURL)
		return decorator
	} else if resp.StatusCode != 200 {
		log.Logger().Warnf("Could not find CSRF crumb: %d %s", resp.StatusCode, resp.Status)
		return decorator
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Logger().Warnf("Failed to read crumb: %s", err)
		return decorator
	}
	crumbPieces := strings.SplitN(string(body), ":", 2)
	if len(crumbPieces) != 2 {
		log.Logger().Warnf("Malformed crumb: %s", body)
		return decorator
	}
	log.Logger().Infof("Obtained crumb")
	return func(req *http.Request) {
		decorator(req)
		req.Header.Add(crumbPieces[0], crumbPieces[1])
	}
}

// Actually generates a new API token.
func generateNewAPIToken(ctx context.Context, serverURL string, verbose bool, decorator func(req *http.Request)) (string, error) {
	client := http.Client{}
	req, err := http.NewRequest(http.MethodPost, util.UrlJoin(serverURL, fmt.Sprintf("/me/descriptorByName/jenkins.security.ApiTokenProperty/generateNewToken?newTokenName=%s", url.QueryEscape("jx create jenkins token"))), nil)
	if err != nil {
		return "", errors.Wrap(err, "building request to generate the API token")
	}
	req = req.WithContext(ctx)
	decorator(req)
	if verbose {
		req.Write(os.Stderr)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "execute generate API token request")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "reading API token from response body")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("generate API token status code: %d, error: %s", resp.StatusCode, string(body))
	}

	type TokenData struct {
		TokenName  string `json:"tokenName"`
		TokenUuid  string `json:"tokenUuid"`
		TokenValue string `json:"tokenValue"`
	}

	type TokenResponse struct {
		Status string    `json:"status"`
		Data   TokenData `json:"data"`
	}
	tokenResponse := &TokenResponse{}
	if err := json.Unmarshal(body, tokenResponse); err != nil {
		return "", errors.Wrap(err, "parsing the API token from response")
	}
	return tokenResponse.Data.TokenValue, nil
}
