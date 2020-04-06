package config

import (
	"fmt"

	"github.com/ghodss/yaml"

	"io/ioutil"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"

	"github.com/spf13/cobra"
)

const defaultMavenSettings = `<settings>
      <!-- sets the local maven repository outside of the ~/.m2 folder for easier mounting of secrets and repo -->
      <localRepository>${user.home}/.mvnrepository</localRepository>
      <!-- lets disable the download progress indicator that fills up logs -->
      <interactiveMode>false</interactiveMode>
      <mirrors>
          <mirror>
              <id>nexus</id>
              <mirrorOf>external:*</mirrorOf>
              <url>http://nexus/repository/maven-group/</url>
          </mirror>
      </mirrors>
      <servers>
          <server>
              <id>nexus</id>
              <username>admin</username>
              <password>%s</password>
          </server>
      </servers>
      <profiles>
          <profile>
              <id>nexus</id>
              <properties>
                  <altDeploymentRepository>nexus::default::http://nexus/repository/maven-snapshots/</altDeploymentRepository>
                  <altReleaseDeploymentRepository>nexus::default::http://nexus/repository/maven-releases/</altReleaseDeploymentRepository>
                  <altSnapshotDeploymentRepository>nexus::default::http://nexus/repository/maven-snapshots/</altSnapshotDeploymentRepository>
              </properties>
          </profile>
          <profile>
              <id>release</id>
              <properties>
                  <gpg.executable>gpg</gpg.executable>
                  <gpg.passphrase>mysecretpassphrase</gpg.passphrase>
              </properties>
          </profile>
      </profiles>
      <activeProfiles>
          <!--make the profile active all the time -->
          <activeProfile>nexus</activeProfile>
      </activeProfiles>
  </settings>
`

const allowedSymbols = "~!#%^_+-=?,."

type ChartMuseum struct {
	ChartMuseumEnv ChartMuseumEnv `json:"env"`
}

type ChartMuseumEnv struct {
	ChartMuseumSecret ChartMuseumSecret `json:"secret"`
}

type ChartMuseumSecret struct {
	User     string `json:"BASIC_AUTH_USER"`
	Password string `json:"BASIC_AUTH_PASS"`
}

type Grafana struct {
	GrafanaSecret GrafanaSecret `json:"server"`
}

type GrafanaSecret struct {
	User     string `json:"adminUser"`
	Password string `json:"adminPassword"`
}

type Jenkins struct {
	JenkinsSecret JenkinsAdminSecret `json:"Master"`
}

type JenkinsAdminSecret struct {
	Password string `json:"AdminPassword"`
}

type PipelineSecrets struct {
	MavenSettingsXML string `json:"MavenSettingsXML,omitempty"`
}

// KanikoSecret store the kaniko service account
type KanikoSecret struct {
	Data string `json:"Data,omitempty"`
}

type AdminSecretsConfig struct {
	IngressBasicAuth string           `json:"JXBasicAuth,omitempty"`
	ChartMuseum      *ChartMuseum     `json:"chartmuseum,omitempty"`
	Grafana          *Grafana         `json:"grafana,omitempty"`
	Jenkins          *Jenkins         `json:"jenkins,omitempty"`
	Nexus            *Nexus           `json:"nexus,omitempty"`
	PipelineSecrets  *PipelineSecrets `json:"PipelineSecrets,omitempty"`
	KanikoSecret     *KanikoSecret    `json:"KanikoSecret,omitempty"`
}

type Nexus struct {
	DefaultAdminPassword string `json:"defaultAdminPassword,omitempty"`
}

type AdminSecretsService struct {
	FileName    string
	Secrets     AdminSecretsConfig
	Flags       AdminSecretsFlags
	ingressAuth BasicAuth
}

type AdminSecretsFlags struct {
	DefaultAdminUsername string
	DefaultAdminPassword string
	KanikoSecret         string
}

func (s *AdminSecretsService) AddAdminSecretsValues(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&s.Flags.DefaultAdminPassword, "default-admin-password", "", "", "the default admin password to access Jenkins, Kubernetes Dashboard, ChartMuseum and Nexus")
	cmd.Flags().StringVarP(&s.Flags.DefaultAdminUsername, "default-admin-username", "", "admin", "the default admin username to access Jenkins, Kubernetes Dashboard, ChartMuseum and Nexus")
}

func (s *AdminSecretsService) NewAdminSecretsConfig() error {
	s.Secrets = AdminSecretsConfig{
		ChartMuseum:     &ChartMuseum{},
		Grafana:         &Grafana{},
		Jenkins:         &Jenkins{},
		PipelineSecrets: &PipelineSecrets{},
		Nexus:           &Nexus{},
		KanikoSecret:    &KanikoSecret{},
	}

	if s.Flags.DefaultAdminPassword == "" {
		log.Logger().Debugf("No default password set, generating a random one")

		input := password.GeneratorInput{
			Symbols: allowedSymbols,
		}

		generator, err := password.NewGenerator(&input)
		if err != nil {
			return errors.Wrap(err, "unable to create password generator")
		}

		s.Flags.DefaultAdminPassword, _ = generator.Generate(20, 4, 2, false, true)
	}

	s.setDefaultSecrets()
	s.NewMavenSettingsXML()
	s.newIngressBasicAuth()
	s.newKanikoSecret()

	return nil
}

func (s *AdminSecretsService) setDefaultSecrets() error {
	s.Secrets.Jenkins.JenkinsSecret.Password = s.Flags.DefaultAdminPassword
	s.Secrets.ChartMuseum.ChartMuseumEnv.ChartMuseumSecret.User = "admin"
	s.Secrets.ChartMuseum.ChartMuseumEnv.ChartMuseumSecret.Password = s.Flags.DefaultAdminPassword
	s.Secrets.Grafana.GrafanaSecret.User = "admin"
	s.Secrets.Grafana.GrafanaSecret.Password = s.Flags.DefaultAdminPassword
	s.Secrets.Nexus.DefaultAdminPassword = s.Flags.DefaultAdminPassword
	return nil
}

// NewMavenSettingsXML generates the maven settings
func (s *AdminSecretsService) NewMavenSettingsXML() error {
	s.Secrets.PipelineSecrets.MavenSettingsXML = fmt.Sprintf(defaultMavenSettings, s.Flags.DefaultAdminPassword)
	return nil
}

func (s *AdminSecretsService) NewAdminSecretsConfigFromSecret(decryptedSecretsFile string) error {
	a := AdminSecretsConfig{}

	data, err := ioutil.ReadFile(decryptedSecretsFile)
	if err != nil {
		return errors.Wrap(err, "unable to read file")
	}

	err = yaml.Unmarshal(data, &a)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshall secrets")
	}

	s.Secrets = a
	s.Flags.DefaultAdminPassword = s.Secrets.Jenkins.JenkinsSecret.Password

	s.setDefaultSecrets()
	s.updateIngressBasicAuth()

	return nil
}

func (s *AdminSecretsService) newKanikoSecret() {
	s.Secrets.KanikoSecret.Data = s.Flags.KanikoSecret
}

func (s *AdminSecretsService) newIngressBasicAuth() {
	password := s.Flags.DefaultAdminPassword
	username := "admin"
	s.ingressAuth = BasicAuth{
		Username: "admin",
		Password: password,
	}
	hash := util.HashPassword(password)
	s.Secrets.IngressBasicAuth = fmt.Sprintf("%s:{SHA}%s", username, hash)
}

func (s *AdminSecretsService) updateIngressBasicAuth() {
	password := s.Flags.DefaultAdminPassword
	parts := strings.Split(s.Secrets.IngressBasicAuth, ":")
	username := parts[0]
	s.ingressAuth = BasicAuth{
		Username: username,
		Password: password,
	}
}

// JenkinsAuth returns the current basic auth credentials for Jenkins
func (s *AdminSecretsService) JenkinsAuth() BasicAuth {
	return BasicAuth{
		Username: "admin",
		Password: s.Secrets.Jenkins.JenkinsSecret.Password,
	}
}

// IngressAuth returns the current basic auth credentials for Ingress
func (s *AdminSecretsService) IngressAuth() BasicAuth {
	return s.ingressAuth
}

// ChartMuseumAuth returns the current credentials for ChartMuseum
func (s *AdminSecretsService) ChartMuseumAuth() BasicAuth {
	return BasicAuth{
		Username: s.Secrets.ChartMuseum.ChartMuseumEnv.ChartMuseumSecret.User,
		Password: s.Secrets.ChartMuseum.ChartMuseumEnv.ChartMuseumSecret.Password,
	}
}

// GrafanaAuth returns the current credentials for Grafana
func (s *AdminSecretsService) GrafanaAuth() BasicAuth {
	return BasicAuth{
		Username: s.Secrets.Grafana.GrafanaSecret.User,
		Password: s.Secrets.Grafana.GrafanaSecret.Password,
	}
}

// NexusAuth returns the current credentials for Nexus
func (s *AdminSecretsService) NexusAuth() BasicAuth {
	return BasicAuth{
		Username: "admin",
		Password: s.Secrets.Nexus.DefaultAdminPassword,
	}
}
