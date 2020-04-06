package create

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/chats"
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	createChatTokenLong = templates.LongDesc(`
		Creates a new User Token for a Chat service
`)

	createChatTokenExample = templates.Examples(`
		# Add a new User Token for a Chat service
		jx create chat token -n jira someUserName

		# As above with the password being passed in
		jx create git token -n jira -p somePassword someUserName	
	`)
)

// CreateChatTokenOptions the command line options for the command
type CreateChatTokenOptions struct {
	options.CreateOptions

	ServerFlags opts.ServerFlags
	Username    string
	Password    string
	ApiToken    string
	Timeout     string
}

// NewCmdCreateChatToken creates a command
func NewCmdCreateChatToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateChatTokenOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token [username]",
		Short:   "Adds a new token/login for a user on a Chat service server",
		Aliases: []string{"login"},
		Long:    createChatTokenLong,
		Example: createChatTokenExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.ServerFlags.AddGitServerFlags(cmd)
	cmd.Flags().StringVarP(&options.ApiToken, "api-token", "t", "", "The API Token for the user")
	cmd.Flags().StringVarP(&options.Timeout, "timeout", "", "", "The timeout if using browser automation to generate the API token (by passing username and password)")

	return cmd
}

// Run implements the command
func (o *CreateChatTokenOptions) Run() error {
	args := o.Args
	if len(args) > 0 {
		o.Username = args[0]
	}
	if len(args) > 1 {
		o.ApiToken = args[1]
	}
	authConfigSvc, err := o.CreateChatAuthConfigService("")
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	server, err := o.FindChatServer(config, &o.ServerFlags)
	if err != nil {
		return err
	}
	if o.Username == "" {
		return fmt.Errorf("No Username specified")
	}

	userAuth := config.GetOrCreateUserAuth(server.URL, o.Username)
	if o.ApiToken != "" {
		userAuth.ApiToken = o.ApiToken
	}

	tokenUrl := chats.ProviderAccessTokenURL(server.Kind, server.URL)

	if userAuth.IsInvalid() {
		f := func(username string) error {
			log.Logger().Infof("Please generate an API Token for %s server %s", server.Kind, server.Label())
			if tokenUrl != "" {
				log.Logger().Infof("Click this URL %s\n", util.ColorInfo(tokenUrl))
			}
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

	err = o.updateChatCredentialsSecret(server, userAuth)
	if err != nil {
		log.Logger().Warnf("Failed to update chat credentials secret: %v", err)
	}

	log.Logger().Infof("Created user %s API Token for chat server %s at %s",
		util.ColorInfo(o.Username), util.ColorInfo(server.Name), util.ColorInfo(server.URL))
	return nil
}

func (o *CreateChatTokenOptions) updateChatCredentialsSecret(server *auth.AuthServer, userAuth *auth.UserAuth) error {
	client, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	ns, _, err := kube.GetDevNamespace(client, curNs)
	if err != nil {
		return err
	}
	options := metav1.GetOptions{}
	name := naming.ToValidName(kube.SecretJenkinsPipelineChatCredentials + server.Kind + "-" + server.Name)
	secrets := client.CoreV1().Secrets(ns)
	secret, err := secrets.Get(name, options)
	create := false
	operation := "update"
	labels := map[string]string{
		kube.LabelCredentialsType: kube.ValueCredentialTypeUsernamePassword,
		kube.LabelCreatedBy:       kube.ValueCreatedByJX,
		kube.LabelKind:            kube.ValueKindIssue,
		kube.LabelServiceKind:     server.Kind,
	}
	annotations := map[string]string{
		kube.AnnotationCredentialsDescription: fmt.Sprintf("API Token for acccessing %s chat inside pipelines", server.URL),
		kube.AnnotationURL:                    server.URL,
		kube.AnnotationName:                   server.Name,
	}
	if err != nil {
		// lets create a new secret
		create = true
		operation = "create"
		secret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Annotations: annotations,
				Labels:      labels,
			},
			Data: map[string][]byte{},
		}
	} else {
		secret.Annotations = util.MergeMaps(secret.Annotations, annotations)
		secret.Labels = util.MergeMaps(secret.Labels, labels)
	}
	if userAuth.Username != "" {
		secret.Data["username"] = []byte(userAuth.Username)
	}
	if userAuth.ApiToken != "" {
		secret.Data["password"] = []byte(userAuth.ApiToken)
	}
	if create {
		_, err = secrets.Create(secret)
	} else {
		_, err = secrets.Update(secret)
	}
	if err != nil {
		return fmt.Errorf("Failed to %s secret %s due to %s", operation, secret.Name, err)
	}
	return nil
}
