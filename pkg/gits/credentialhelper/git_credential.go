package credentialhelper

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// GitCredential represents the different parts of a git credential URL
// See also https://git-scm.com/docs/git-credential
type GitCredential struct {
	Protocol string
	Host     string
	Path     string
	Username string
	Password string
}

// CreateGitCredential creates a CreateGitCredential instance from a slice of strings where each element is a key/value pair
// separated by '='.
func CreateGitCredential(lines []string) (GitCredential, error) {
	var credential GitCredential

	if lines == nil {
		return credential, errors.New("no data lines provided")
	}

	fieldMap, err := util.ExtractKeyValuePairs(lines, "=")
	if err != nil {
		return credential, errors.Wrap(err, "unable to extract git credential parameters")
	}

	data, err := json.Marshal(fieldMap)
	if err != nil {
		return GitCredential{}, errors.Wrapf(err, "unable to marshal git credential data")
	}

	err = json.Unmarshal(data, &credential)
	if err != nil {
		return GitCredential{}, errors.Wrapf(err, "unable unmarshal git credential data")
	}

	return credential, nil
}

// CreateGitCredentialFromURL creates a CreateGitCredential instance from a URL and optional username and password.
func CreateGitCredentialFromURL(gitURL string, username string, password string) (GitCredential, error) {
	var credential GitCredential

	if gitURL == "" {
		return credential, errors.New("url cannot be empty")
	}

	u, err := url.Parse(gitURL)
	if err != nil {
		return credential, errors.Wrapf(err, "unable to parse URL %s", gitURL)
	}

	credential.Protocol = u.Scheme
	credential.Host = u.Host
	credential.Path = u.Path
	if username != "" {
		credential.Username = username
	}

	if password != "" {
		credential.Password = password
	}

	return credential, nil
}

// String returns a string representation of this instance according to the expected format of git credential helpers.
// See also https://git-scm.com/docs/git-credential
func (g *GitCredential) String() string {
	answer := ""

	value := reflect.ValueOf(g).Elem()
	typeOfT := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		answer = answer + fmt.Sprintf("%s=%v\n", strings.ToLower(typeOfT.Field(i).Name), field.Interface())
	}

	answer = answer + "\n"

	return answer
}

// Clone clones this GitCredential instance
func (g *GitCredential) Clone() GitCredential {
	clone := GitCredential{}

	value := reflect.ValueOf(g).Elem()
	typeOfT := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		value := field.String()
		v := reflect.ValueOf(&clone).Elem().FieldByName(typeOfT.Field(i).Name)
		v.SetString(value)
	}

	return clone
}

// URL returns a URL from the data of this instance. If not enough information exist an error is returned
func (g *GitCredential) URL() (url.URL, error) {
	urlAsString := g.Protocol + "://" + g.Host
	if g.Path != "" {
		urlAsString = urlAsString + "/" + g.Path
	}
	u, err := url.Parse(urlAsString)
	if err != nil {
		return url.URL{}, errors.Wrap(err, "unable to construct URL")
	}

	u.User = url.UserPassword(g.Username, g.Password)
	return *u, nil
}
