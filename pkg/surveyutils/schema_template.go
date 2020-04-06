package surveyutils

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/helm/pkg/chartutil"

	"github.com/pkg/errors"
)

// TemplateSchemaFile if there is a template for the schema file then evaluate it and write the schema file
func TemplateSchemaFile(schemaFileName string, requirements *config.RequirementsConfig) error {
	templateFile := strings.TrimSuffix(schemaFileName, ".schema.json") + ".tmpl.schema.json"
	exists, err := util.FileExists(templateFile)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	data, err := ReadSchemaTemplate(templateFile, requirements)
	if err != nil {
		return errors.Wrapf(err, "failed to render schema template %s", templateFile)
	}
	err = ioutil.WriteFile(schemaFileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save schema file %s generated from template %s", schemaFileName, templateFile)
	}
	log.Logger().Infof("generated schema file %s from template %s\n", util.ColorInfo(schemaFileName), util.ColorInfo(templateFile))
	return nil
}

func validateRequirements(requirements *config.RequirementsConfig) {
	if requirements.Cluster.GitKind == "" {
		requirements.Cluster.GitKind = "github"
	}
	if requirements.Cluster.GitServer == "" {
		switch requirements.Cluster.GitKind {
		case "bitbucketcloud":
			requirements.Cluster.GitServer = "https://bitbucket.org"
		case "github":
			requirements.Cluster.GitServer = "https://github.com"
		case "gitlab":
			requirements.Cluster.GitServer = "https://gitlab.com"
		}
	}
}

// readSchemaTemplate evaluates the given go template file and returns the output data
func ReadSchemaTemplate(templateFile string, requirements *config.RequirementsConfig) ([]byte, error) {
	_, name := filepath.Split(templateFile)
	funcMap := helm.NewFunctionMap()
	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap).ParseFiles(templateFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse schema template: %s", templateFile)
	}

	validateRequirements(requirements)

	requirementsMap, err := requirements.ToMap()
	if err != nil {
		return nil, errors.Wrapf(err, "failed turn requirements into a map: %v", requirements)
	}

	templateData := map[string]interface{}{
		"GitKind":      requirements.Cluster.GitKind,
		"GitServer":    requirements.Cluster.GitServer,
		"GithubApp":    requirements.GithubApp != nil && requirements.GithubApp.Enabled,
		"Requirements": chartutil.Values(requirementsMap),
		"Environments": chartutil.Values(requirements.EnvironmentMap()),
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute schema template: %s", templateFile)
	}
	data := buf.Bytes()
	return data, nil
}
