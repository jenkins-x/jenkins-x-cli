package terraform

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"

	"io"

	"github.com/jenkins-x/jx/pkg/util"
)

// MinTerraformVersion defines the minimum terraform version we support
var MinTerraformVersion = "0.12.0"

func Init(terraformDir string, serviceAccountPath string) error {
	log.Logger().Infof("Initialising Terraform")

	if _, err := os.Stat(".terraform"); !os.IsNotExist(err) {
		log.Logger().Infof("Discovered local .terraform directory, removing...")
		err = os.RemoveAll(".terraform")
		if err != nil {
			return errors.Wrap(err, "unable to remove local .terraform directory")
		}
	}

	os.Setenv("GOOGLE_CREDENTIALS", serviceAccountPath)
	cmd := util.Command{
		Name: "terraform",
		Args: []string{"init", terraformDir},
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}

func Plan(terraformDir string, terraformVars string, serviceAccountPath string) (string, error) {
	log.Logger().Infof("Showing Terraform Plan")
	cmd := util.Command{
		Name: "terraform",
		Args: []string{"plan",
			fmt.Sprintf("-var-file=%s", terraformVars),
			"-var",
			fmt.Sprintf("credentials=%s", serviceAccountPath),
			terraformDir},
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return out, err
	}
	return out, nil
}

// Output displays a terraform output from the local terraform.tfstate
func Output(output string) (string, error) {
	log.Logger().Debugf("Extracting terraform output %s", output)
	cmd := util.Command{
		Name: "terraform",
		Args: []string{"output",
			output},
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		return out, err
	}
	return out, nil
}

func Apply(terraformDir string, terraformVars string, serviceAccountPath string, stdout io.Writer, stderr io.Writer) error {
	log.Logger().Infof("Applying Terraform")
	cmd := util.Command{
		Name: "terraform",
		Args: []string{"apply", "-auto-approve",
			fmt.Sprintf("-var-file=%s", terraformVars),
			"-var",
			fmt.Sprintf("credentials=%s", serviceAccountPath),
			terraformDir},
		Out: stdout,
		Err: stderr,
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}

func WriteKeyValueToFileIfNotExists(path string, key string, value string) error {
	// file exists
	if _, err := os.Stat(path); err == nil {
		buffer, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		contents := string(buffer)

		if strings.Contains(contents, key) {
			return nil
		}
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	line := fmt.Sprintf("%s = \"%s\"\n", key, value)

	_, err = file.WriteString(line)
	if err != nil {
		return err
	}

	return nil
}

func ReadValueFromFile(path string, key string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		buffer, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}
		contents := string(buffer)
		lines := strings.Split(contents, "\n")
		for _, line := range lines {
			if strings.Contains(line, key) {
				tokens := strings.Split(line, "=")
				trimmedValue := strings.Trim(strings.TrimSpace(tokens[1]), "\"")
				return trimmedValue, nil
			}
		}

	}
	return "", nil
}

// CheckVersion checks the installed version of terraform to sure it is greater than 0.12.0
func CheckVersion() error {
	log.Logger().Infof("Checking Terraform Version...")
	cmd := util.Command{
		Name: "terraform",
		Args: []string{"-version"},
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}

	version, err := extractVersionFromTerraformOutput(output)

	log.Logger().Infof("Determined terraform version as %s", util.ColorInfo(version))

	if err != nil {
		return err
	}

	v, err := semver.Make(version)
	versionClause := fmt.Sprintf(">= %s", MinTerraformVersion)

	r, err := semver.ParseRange(versionClause)
	if !r(v) {
		return errors.Errorf("terraform version appears to be too old, please install a newer version '%s'", versionClause)
	}

	log.Logger().Infof("Terraform version appears to be valid")

	return nil
}

func extractVersionFromTerraformOutput(output string) (string, error) {

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Terraform") {
			versionTokens := strings.Split(line, " ")
			return strings.TrimPrefix(versionTokens[1], "v"), nil
		}
	}

	return "", errors.Errorf("unable to extract version from output '%s'", output)

}
