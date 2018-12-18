package config

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/sethvargo/go-password/password"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
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
          <id>local-nexus</id>
          <username>admin</username>
          <password>%s</password>
          </server>
      </servers>
      <profiles>
          <profile>
              <id>nexus</id>
              <properties>
                  <altDeploymentRepository>local-nexus::default::http://nexus/repository/maven-snapshots/</altDeploymentRepository>
                  <altReleaseDeploymentRepository>local-nexus::default::http://nexus/repository/maven-releases/</altReleaseDeploymentRepository>
                  <altSnapshotDeploymentRepository>local-nexus::default::http://nexus/repository/maven-snapshots/</altSnapshotDeploymentRepository>
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

const allowedSymbols = "~!#%^*_+-=?,."

type ChartMuseum struct {
	ChartMuseumEnv ChartMuseumEnv `yaml:"env"`
}

type ChartMuseumEnv struct {
	ChartMuseumSecret ChartMuseumSecret `yaml:"secret"`
}

type ChartMuseumSecret struct {
	User     string `yaml:"BASIC_AUTH_USER"`
	Password string `yaml:"BASIC_AUTH_PASS"`
}

type Grafana struct {
	GrafanaSecret GrafanaSecret `yaml:"server"`
}

type GrafanaSecret struct {
	User     string `yaml:"adminUser"`
	Password string `yaml:"adminPassword"`
}

type Jenkins struct {
	JenkinsSecret JenkinsAdminSecret `yaml:"Master"`
}

type JenkinsAdminSecret struct {
	Password string `yaml:"AdminPassword"`
}

type PipelineSecrets struct {
	MavenSettingsXML string `yaml:"MavenSettingsXML,omitempty"`
}

type AdminSecretsConfig struct {
	IngressBasicAuth string           `yaml:"JXBasicAuth,omitempty"`
	ChartMuseum      *ChartMuseum     `yaml:"chartmuseum,omitempty"`
	Grafana          *Grafana         `yaml:"grafana,omitempty"`
	Jenkins          *Jenkins         `yaml:"jenkins,omitempty"`
	Nexus            *Nexus           `yaml:"nexus,omitempty"`
	PipelineSecrets  *PipelineSecrets `yaml:"PipelineSecrets,omitempty"`
}

type Nexus struct {
	DefaultAdminPassword string `yaml:"defaultAdminPassword,omitempty"`
}

type AdminSecretsService struct {
	FileName    string
	Secrets     AdminSecretsConfig
	Flags       AdminSecretsFlags
	ingressAuth BasicAuth
}

type AdminSecretsFlags struct {
	DefaultAdminPassword string
}

func (s *AdminSecretsService) AddAdminSecretsValues(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&s.Flags.DefaultAdminPassword, "default-admin-password", "", "", "the default admin password to access Jenkins, Kubernetes Dashboard, Chartmuseum and Nexus")
}

func (s *AdminSecretsService) NewAdminSecretsConfig() error {
	s.Secrets = AdminSecretsConfig{
		ChartMuseum:     &ChartMuseum{},
		Grafana:         &Grafana{},
		Jenkins:         &Jenkins{},
		PipelineSecrets: &PipelineSecrets{},
		Nexus:           &Nexus{},
	}

	if s.Flags.DefaultAdminPassword == "" {
		log.Infof("No default password set, generating a random one\n")

		input := password.GeneratorInput{
			Symbols: allowedSymbols,
		}

		generator, err := password.NewGenerator(&input)
		if err != nil {
			return errors.Wrap(err, "unable to create password generator")
		}

		s.Flags.DefaultAdminPassword, _ = generator.Generate(20, 4, 2, false, true)
	}

	s.Secrets.Jenkins.JenkinsSecret.Password = s.Flags.DefaultAdminPassword
	s.Secrets.ChartMuseum.ChartMuseumEnv.ChartMuseumSecret.User = "admin"
	s.Secrets.ChartMuseum.ChartMuseumEnv.ChartMuseumSecret.Password = s.Flags.DefaultAdminPassword
	s.Secrets.Grafana.GrafanaSecret.User = "admin"
	s.Secrets.Grafana.GrafanaSecret.Password = s.Flags.DefaultAdminPassword
	s.Secrets.Nexus.DefaultAdminPassword = s.Flags.DefaultAdminPassword
	s.Secrets.PipelineSecrets.MavenSettingsXML = fmt.Sprintf(defaultMavenSettings, s.Flags.DefaultAdminPassword)

	s.newIngressBasicAuth()

	return nil
}

func (s *AdminSecretsService) NewAdminSecretsConfigFromSecret(decryptedSecrets string) error {
	a := AdminSecretsConfig{}

	data, err := ioutil.ReadFile(decryptedSecrets)
	if err != nil {
		return errors.Wrap(err, "unable to read file")
	}

	err = yaml.Unmarshal([]byte(data), &a)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshall secrets")
	}

	s.Secrets = a
	s.Flags.DefaultAdminPassword = s.Secrets.Jenkins.JenkinsSecret.Password
	s.updateIngressBasicAuth()
	return nil
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
