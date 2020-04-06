package iks

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	gohttp "net/http"
	"net/url"
	"strconv"

	"github.com/jenkins-x/jx/pkg/log"

	ibmcloud "github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/authentication"
	"github.com/IBM-Cloud/bluemix-go/bmxerror"
	"github.com/IBM-Cloud/bluemix-go/client"
	"github.com/IBM-Cloud/bluemix-go/http"
	"github.com/IBM-Cloud/bluemix-go/rest"
	"github.com/IBM-Cloud/bluemix-go/session"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Config struct {
	Auths map[string]*Auth `json:"auths,omitempty"`
}

type Auth struct {
	Auth string `json:"auth,omitempty"`
}

type RegistryServiceAPI interface {
	Registry() Registry
}

//MccpService holds the client
type registry struct {
	*client.Client
}

type AddTokenResponse struct {
	Token string `json:"token"`
}

type Registry interface {
	AddToken(account string, description string, permanent bool, write bool) (string, error)
}

var regionToEndpoint = map[string]string{
	"us-south": "https://registry.ng.bluemix.net",
	"us-east":  "https://registry.ng.bluemix.net",
	"global":   "https://registry.bluemix.net",
	"eu-gb":    "https://registry.eu-gb.bluemix.net",
	"au-syd":   "https://registry.au-syd.bluemix.net",
	"eu-de":    "https://registry.eu-de.bluemix.net",
}

const (
	ErrCodeNotAuthorized        = "NotAuthorized"
	ErrCodeUnableToAuthenticate = "UnableToAuthenticate"
)

func (a *registry) Registry() Registry {
	return newRegistryAPI(a.Client)
}

func newRegistryAPI(c *client.Client) Registry {
	return &registry{
		Client: c,
	}
}

// Try to use the regional registry used by the cluster
// We will also set the corresponding secret for jenkins-x right away
func GetClusterRegistry(kubeClient kubernetes.Interface) string {

	registry := "registry.bluemix.net" // default to global
	secretFromConfig, err := kubeClient.CoreV1().Secrets("default").Get("bluemix-default-secret-regional", metav1.GetOptions{})
	if err != nil {
		return ""
	}
	dockerConfig := &Config{}
	err = json.Unmarshal(secretFromConfig.Data[".dockerconfigjson"], dockerConfig)
	if err == nil {
		for k := range dockerConfig.Auths {
			registry = k
		}
	}
	return registry
}

func GetRegistryConfigJSON(registry string) (string, error) {

	//token :=
	c := new(ibmcloud.Config)
	accountID, err := ConfigFromJSON(c)
	if err != nil {
		return "", err
	}

	s, err := session.New(c)
	if err != nil {
		return "", err
	}

	registryAPI, err := NewRegistryServiceAPI(s)
	registryIF := registryAPI.Registry()
	clusterName, _ := GetClusterName()
	token, err := registryIF.AddToken(accountID, fmt.Sprintf("%s Jenkins-X Token", clusterName), true, true)
	if err != nil {
		return "", err
	}

	newSecret := &Auth{}
	dockerConfig := &Config{}
	newSecret.Auth = b64.StdEncoding.EncodeToString([]byte("token:" + token))
	if dockerConfig.Auths == nil {
		dockerConfig.Auths = map[string]*Auth{}
	}
	dockerConfig.Auths[registry] = newSecret

	configBytes, err := json.Marshal(dockerConfig)
	if err != nil {
		return "", err
	}
	return string(configBytes), nil
}

//New ...
func NewRegistryServiceAPI(sess *session.Session) (RegistryServiceAPI, error) {
	config := sess.Config.Copy()
	if config.HTTPClient == nil {
		config.HTTPClient = http.NewHTTPClient(config)
	}
	tokenRefreher, err := authentication.NewIAMAuthRepository(config, &rest.Client{
		DefaultHeader: gohttp.Header{
			"User-Agent": []string{http.UserAgent()},
		},
		HTTPClient: config.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	if config.IAMAccessToken == "" {
		err := authentication.PopulateTokens(tokenRefreher, config)
		if err != nil {
			return nil, err
		}
	}
	if config.Endpoint == nil {
		endpoint := regionToEndpoint[config.Region]
		config.Endpoint = &endpoint
	}

	return &registry{
		Client: client.New(config, ibmcloud.IAMService, tokenRefreher),
	}, nil
}

func (a *registry) AddToken(accountID string, description string, permanent bool, write bool) (string, error) {
	queryResp := AddTokenResponse{}
	v := url.Values{}
	v.Set("description", description)
	v.Set("permanent", strconv.FormatBool(permanent))
	v.Set("write", strconv.FormatBool(write))
	m := make(map[string]string, 1)
	m["Account"] = accountID

	response, err := a.Client.Post(fmt.Sprintf("/api/v1/tokens?%s", v.Encode()), nil, &queryResp, m)
	if err != nil {
		log.Logger().Errorf("error - %s", err)
		if response.StatusCode == 401 {
			return "", bmxerror.New(ErrCodeNotAuthorized,
				"You are not authorized to view the requested resource, or your IBM Cloud bearer token is invalid. Correct the request and try again.")
		}
		if response.StatusCode == 503 {
			return "", bmxerror.New(ErrCodeUnableToAuthenticate,
				"Unable to authenticate with IBM Cloud. Try again later.")
		}
		return "", err

	}
	return queryResp.Token, nil

}
