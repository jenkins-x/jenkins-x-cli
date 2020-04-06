package apps

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/secreturl"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/environments"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/surveyutils"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	//ValuesAnnotation is the name of the annotation used to stash values
	ValuesAnnotation = "jenkins.io/values.yaml"
)

// StashValues takes the values used to configure an app and annotates the APP CRD with them allowing them to be used
// at a later date e.g. when the app is upgraded
func StashValues(values []byte, name string, jxClient versioned.Interface, ns string, chartDir string, repository string) (bool, *jenkinsv1.App, error) {
	// locate the app CRD
	create := false
	app, err := jxClient.JenkinsV1().Apps(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		create = true
		app = &jenkinsv1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: jenkinsv1.AppSpec{},
		}
	}
	// base64 encode the values.yaml
	encoded := base64.StdEncoding.EncodeToString(values)
	if app.Annotations == nil {
		app.Annotations = make(map[string]string)
	}
	app.Annotations[ValuesAnnotation] = encoded

	err = environments.AddAppMetaData(chartDir, app, repository)
	if err != nil {
		return false, nil, errors.Wrapf(err, "adding metadata to app %s", app.Name)
	}

	return create, app, nil
}

// AddValuesToChart adds a values file to the chart rooted at dir
func AddValuesToChart(name string, values []byte, verbose bool) (string, func(), error) {
	valuesYaml, err := yaml.JSONToYAML(values)
	if err != nil {
		return "", func() {}, errors.Wrapf(err, "error converting values from json to yaml\n\n%v", values)
	}
	if verbose {
		log.Logger().Infof("Generated values.yaml:\n\n%v", util.ColorInfo(string(valuesYaml)))
	}

	valuesFile, err := ioutil.TempFile("", fmt.Sprintf("%s-values.yaml", util.ToValidFileSystemName(name)))
	cleanup := func() {
		err = valuesFile.Close()
		if err != nil {
			log.Logger().Warnf("Error closing %s because %v", valuesFile.Name(), err)
		}
		err = util.DeleteFile(valuesFile.Name())
		if err != nil {
			log.Logger().Warnf("Error deleting %s because %v", valuesFile.Name(), err)
		}
	}
	if err != nil {
		return "", func() {}, errors.Wrapf(err, "creating tempfile to write values for %s", name)
	}
	_, err = valuesFile.Write(valuesYaml)
	if err != nil {
		return "", func() {}, errors.Wrapf(err, "writing values to %s for %s", valuesFile.Name(), name)
	}
	return valuesFile.Name(), cleanup, nil
}

//GenerateQuestions asks questions based on the schema
func GenerateQuestions(schema []byte, batchMode bool, askExisting bool, basePath string, secretURLClient secreturl.Client,
	existing map[string]interface{}, vaultScheme string, handles util.IOFileHandles) ([]byte, error) {
	schemaOptions := surveyutils.JSONSchemaOptions{
		VaultClient:         secretURLClient,
		VaultScheme:         vaultScheme,
		VaultBasePath:       basePath,
		Out:                 handles.Out,
		In:                  handles.In,
		OutErr:              handles.Err,
		IgnoreMissingValues: false,
		NoAsk:               batchMode,
		AutoAcceptDefaults:  batchMode,
		AskExisting:         askExisting,
	}
	// For adding an app there are by definition no existing values,
	// and whether we auto-accept defaults is determined by batch mode
	values, err := schemaOptions.GenerateValues(schema, existing)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return values, nil
}

func addApp(create bool, jxClient versioned.Interface, app *jenkinsv1.App) error {
	if create {
		_, err := jxClient.JenkinsV1().Apps(app.Namespace).Create(app)
		if err != nil {
			return errors.Wrapf(err, "creating App %s to annotate with values.yaml", app.Name)
		}
	} else {
		_, err := jxClient.JenkinsV1().Apps(app.Namespace).PatchUpdate(app)
		if err != nil {
			return errors.Wrapf(err, "updating App %s to annotate with values.yaml", app.Name)
		}
	}
	return nil
}

// ProcessValues is responsible for taking a schema, asking questions of the user (using in, out and outErr), and generating
// a yaml file that contains the answers. The path to the yaml file is returned, along with a function to cleanup temporary
// resources, including the yaml file. The gitOpsURL, if specified, is used to determine the path to store the secrets in the
// vault, otherwise the team name is used. If batchMode is true, it alters the way questions are asked, trying to use existing answers or defaults
// where possible. If askExisting is true then all questions, even those with existing answers are asked. The vault client is
// used to store secrets, and the secretsScheme is used as the scheme part of the url to the secret.
func ProcessValues(
	schema []byte,
	name string,
	gitOpsURL string,
	teamName string,
	basepath string,
	batchMode bool,
	askExisting bool,
	secretURLClient secreturl.Client,
	existing map[string]interface{},
	vaultScheme string,
	handles util.IOFileHandles,
	verbose bool) (string, func(), error) {
	var values []byte
	var err error
	if basepath == "" {
		if gitOpsURL != "" {
			gitInfo, err := gits.ParseGitURL(gitOpsURL)
			if err != nil {
				return "", func() {}, err
			}
			basepath = strings.Join([]string{"gitOps", gitInfo.Organisation, gitInfo.Name}, "/")
		} else {
			basepath = strings.Join([]string{"teams", teamName}, "/")
		}
	}
	values, err = GenerateQuestions(schema, batchMode, askExisting, basepath, secretURLClient, existing, vaultScheme, handles)
	if err != nil {
		return "", func() {}, errors.Wrapf(err, "asking questions for schema")
	}
	valuesFileName, cleanupValues, err := AddValuesToChart(name, values, verbose)
	cleanup := func() {
		cleanupValues()
	}
	if err != nil {
		return "", cleanup, err
	}
	return valuesFileName, cleanup, nil
}
