package kube

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	openapi "github.com/jenkins-x/jx/pkg/client/openapi/all"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-openapi/pkg/common"

	"github.com/go-openapi/jsonreference"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/go-openapi/spec"
	"github.com/pkg/errors"

	"github.com/cenkalti/backoff"
	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

// RegisterAllCRDs ensures that all Jenkins-X CRDs are registered
func RegisterAllCRDs(apiClient apiextensionsclientset.Interface) error {
	err := RegisterCommitStatusCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Commit Status CRD")
	}
	err = RegisterExtensionCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Extension CRD")
	}
	err = RegisterAppCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the App CRD")
	}
	err = RegisterPluginCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Plugin CRD")
	}
	err = RegisterEnvironmentRoleBindingCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Environment Role Binding CRD")
	}
	err = RegisterGitServiceCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Git Service CRD")
	}
	err = RegisterFactCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Fact CRD")
	}
	err = RegisterTeamCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Team CRD")
	}
	err = RegisterUserCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the User CRD")
	}
	err = RegisterWorkflowCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Workflow CRD")
	}

	return RegisterPipelineCRDs(apiClient)
}

// RegisterPipelineCRDs ensures that all Jenkins X Pipeline related CRDs are registered
func RegisterPipelineCRDs(apiClient apiextensionsclientset.Interface) error {
	err := RegisterBuildPackCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Build Pack CRD")
	}
	err = RegisterEnvironmentCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Environment CRD")
	}
	err = RegisterReleaseCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Release CRD")
	}
	err = RegisterPipelineActivityCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Pipeline Activity CRD")
	}
	err = RegisterPipelineStructureCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Pipeline Structure CRD")
	}
	err = RegisterPluginCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Plugin CRD")
	}
	err = RegisterSourceRepositoryCRD(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the SourceRepository CRD")
	}
	err = RegisterPipelineScheduler(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the PipelineScheduler CRD")
	}
	err = RegisterSourceRepositoryGroup(apiClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the RegisterSourceRepositoryGroup CRD")
	}
	return nil
}

// RegisterEnvironmentCRD ensures that the CRD is registered for Environments
func RegisterEnvironmentCRD(apiClient apiextensionsclientset.Interface) error {
	name := "environments." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Environment",
		ListKind:   "EnvironmentList",
		Plural:     "environments",
		Singular:   "environment",
		ShortNames: []string{"env"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Namespace",
			Type:        "string",
			Description: "The namespace used for the environment",
			JSONPath:    ".spec.namespace",
		},
		{
			Name:        "Kind",
			Type:        "string",
			Description: "The kind of environment",
			JSONPath:    ".spec.kind",
		},
		{
			Name:        "Promotion",
			Type:        "string",
			Description: "The strategy used for promoting to this environment",
			JSONPath:    ".spec.promotionStrategy",
		},
		{
			Name:        "Order",
			Type:        "integer",
			Description: "The order in which environments are automatically promoted",
			JSONPath:    ".spec.order",
		},
		{
			Name:        "Git URL",
			Type:        "string",
			Description: "The Git repository URL for the source of the environment configuration",
			JSONPath:    ".spec.source.url",
		},
		{
			Name:        "Git Branch",
			Type:        "string",
			Description: "The git branch for the source of the environment configuration",
			JSONPath:    ".spec.source.ref",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterEnvironmentRoleBindingCRD ensures that the CRD is registered for Environments
func RegisterEnvironmentRoleBindingCRD(apiClient apiextensionsclientset.Interface) error {
	name := "environmentrolebindings." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "EnvironmentRoleBinding",
		ListKind:   "EnvironmentRoleBindingList",
		Plural:     "environmentrolebindings",
		Singular:   "environmentrolebinding",
		ShortNames: []string{"envrolebindings", "envrolebinding", "envrb", "erb"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterGitServiceCRD ensures that the CRD is registered for GitServices
func RegisterGitServiceCRD(apiClient apiextensionsclientset.Interface) error {
	name := "gitservices." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "GitService",
		ListKind:   "GitServiceList",
		Plural:     "gitservices",
		Singular:   "gitservice",
		ShortNames: []string{"gits", "gs"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Git URL",
			Type:        "string",
			Description: "The URL of the Git repository",
			JSONPath:    ".spec.url",
		},
		{
			Name:        "Kind",
			Type:        "string",
			Description: "The kind of the Git provider",
			JSONPath:    ".spec.gitKind",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterPipelineActivityCRD ensures that the CRD is registered for PipelineActivity
func RegisterPipelineActivityCRD(apiClient apiextensionsclientset.Interface) error {
	name := "pipelineactivities." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "PipelineActivity",
		ListKind:   "PipelineActivityList",
		Plural:     "pipelineactivities",
		Singular:   "pipelineactivity",
		ShortNames: []string{"activity", "act", "pa"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Git URL",
			Type:        "string",
			Description: "The URL of the Git repository",
			JSONPath:    ".spec.gitUrl",
		},
		{
			Name:        "Status",
			Type:        "string",
			Description: "The status of the pipeline",
			JSONPath:    ".spec.status",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterPipelineStructureCRD ensures that the CRD is registered for PipelineStructure
func RegisterPipelineStructureCRD(apiClient apiextensionsclientset.Interface) error {
	name := "pipelinestructures." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "PipelineStructure",
		ListKind:   "PipelineStructureList",
		Plural:     "pipelinestructures",
		Singular:   "pipelinestructure",
		ShortNames: []string{"structure", "ps"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterFactCRD ensures that the CRD is registered for Fact
func RegisterFactCRD(apiClient apiextensionsclientset.Interface) error {
	name := "facts." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Fact",
		ListKind:   "FactList",
		Plural:     "facts",
		Singular:   "fact",
		ShortNames: []string{"fact"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Name",
			Type:        "string",
			Description: "The name of the fact",
			JSONPath:    ".spec.name",
		},
		{
			Name:        "Type",
			Type:        "string",
			Description: "The type of the fact",
			JSONPath:    ".spec.factType",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterExtensionCRD ensures that the CRD is registered for Extension
func RegisterExtensionCRD(apiClient apiextensionsclientset.Interface) error {
	name := "extensions." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Extension",
		ListKind:   "ExtensionList",
		Plural:     "extensions",
		Singular:   "extensions",
		ShortNames: []string{"extension", "ext"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Name",
			Type:        "string",
			Description: "The name of the extension",
			JSONPath:    ".spec.name",
		},
		{
			Name:        "Description",
			Type:        "string",
			Description: "A description of the extension",
			JSONPath:    ".spec.description",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterBuildPackCRD ensures that the CRD is registered for BuildPack
func RegisterBuildPackCRD(apiClient apiextensionsclientset.Interface) error {
	name := "buildpacks." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "BuildPack",
		ListKind:   "BuildPackList",
		Plural:     "buildpacks",
		Singular:   "buildpack",
		ShortNames: []string{"bp"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "LABEL",
			Type:        "string",
			Description: "The label of the BuildPack",
			JSONPath:    ".spec.Label",
		},
		{
			Name:        "GIT URL",
			Type:        "string",
			Description: "The Git URL of the BuildPack",
			JSONPath:    ".spec.gitUrl",
		},
		{
			Name:        "Git Ref",
			Type:        "string",
			Description: "The Git REf of the BuildPack",
			JSONPath:    ".spec.gitRef",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterAppCRD ensures that the CRD is registered for App
func RegisterAppCRD(apiClient apiextensionsclientset.Interface) error {
	name := "apps." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "App",
		ListKind:   "AppList",
		Plural:     "apps",
		Singular:   "app",
		ShortNames: []string{"app"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterPipelineScheduler ensures that the CRD is registered for App
func RegisterPipelineScheduler(apiClient apiextensionsclientset.Interface) error {
	name := "schedulers." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Scheduler",
		ListKind:   "SchedulerList",
		Plural:     "schedulers",
		Singular:   "scheduler",
		ShortNames: []string{"scheduler"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterSourceRepositoryGroup ensures that the CRD is registered for App
func RegisterSourceRepositoryGroup(apiClient apiextensionsclientset.Interface) error {
	name := "sourcerepositorygroups." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "SourceRepositoryGroup",
		ListKind:   "SourceRepositoryGroupList",
		Plural:     "sourcerepositorygroups",
		Singular:   "sourcerepositorygroup",
		ShortNames: []string{"srg"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Scheduler",
			Type:        "string",
			Description: "The pipeline scheduler used by the source repository group",
			JSONPath:    ".spec.scheduler.name",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterSourceRepositoryCRD ensures that the CRD is registered for Applications
func RegisterSourceRepositoryCRD(apiClient apiextensionsclientset.Interface) error {
	name := "sourcerepositories." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "SourceRepository",
		ListKind:   "SourceRepositoryList",
		Plural:     "sourcerepositories",
		Singular:   "sourcerepository",
		ShortNames: []string{"sourcerepo", "srcrepo", "sr"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "URL",
			Type:        "string",
			Description: "The URL of the git repository",
			JSONPath:    ".spec.url",
		},
		{
			Name:        "Description",
			Type:        "string",
			Description: "A description of the source code repository - non-functional user-data",
			JSONPath:    ".spec.description",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterPluginCRD ensures that the CRD is registered for Plugin
func RegisterPluginCRD(apiClient apiextensionsclientset.Interface) error {
	name := "plugins." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Plugin",
		ListKind:   "PluginList",
		Plural:     "plugins",
		Singular:   "plugin",
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Name",
			Type:        "string",
			Description: "The name of the plugin",
			JSONPath:    ".spec.name",
		},
		{
			Name:        "Description",
			Type:        "string",
			Description: "A description of the plugin",
			JSONPath:    ".spec.description",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterCommitStatusCRD ensures that the CRD is registered for CommitStatus
func RegisterCommitStatusCRD(apiClient apiextensionsclientset.Interface) error {
	name := "commitstatuses." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "CommitStatus",
		ListKind:   "CommitStatusList",
		Plural:     "commitstatuses",
		Singular:   "commitstatus",
		ShortNames: []string{"commitstatus"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterReleaseCRD ensures that the CRD is registered for Release
func RegisterReleaseCRD(apiClient apiextensionsclientset.Interface) error {
	name := "releases." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Release",
		ListKind:   "ReleaseList",
		Plural:     "releases",
		Singular:   "release",
		ShortNames: []string{"rel"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Name",
			Type:        "string",
			Description: "The name of the Release",
			JSONPath:    ".spec.name",
		},
		{
			Name:        "Version",
			Type:        "string",
			Description: "The version number of the Release",
			JSONPath:    ".spec.version",
		},
		{
			Name:        "Git URL",
			Type:        "string",
			Description: "The URL of the Git repository",
			JSONPath:    ".spec.gitHttpUrl",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterUserCRD ensures that the CRD is registered for User
func RegisterUserCRD(apiClient apiextensionsclientset.Interface) error {
	name := "users." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "User",
		ListKind:   "UserList",
		Plural:     "users",
		Singular:   "user",
		ShortNames: []string{"usr"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Name",
			Type:        "string",
			Description: "The name of the user",
			JSONPath:    ".spec.name",
		},
		{
			Name:        "Email",
			Type:        "string",
			Description: "The email address of the user",
			JSONPath:    ".spec.email",
		},
	}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterTeamCRD ensures that the CRD is registered for Team
func RegisterTeamCRD(apiClient apiextensionsclientset.Interface) error {
	name := "teams." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Team",
		ListKind:   "TeamList",
		Plural:     "teams",
		Singular:   "team",
		ShortNames: []string{"tm"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{
		{
			Name:        "Kind",
			Type:        "string",
			Description: "The kind of Team",
			JSONPath:    ".spec.kind",
		},
		{
			Name:        "Status",
			Type:        "string",
			Description: "The provision status of the Team",
			JSONPath:    ".status.provisionStatus",
		},
	}

	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterWorkflowCRD ensures that the CRD is registered for Environments
func RegisterWorkflowCRD(apiClient apiextensionsclientset.Interface) error {
	name := "workflows." + jenkinsio.GroupName
	names := &v1beta1.CustomResourceDefinitionNames{
		Kind:       "Workflow",
		ListKind:   "WorkflowList",
		Plural:     "workflows",
		Singular:   "workflow",
		ShortNames: []string{"flow"},
		Categories: []string{"all"},
	}
	columns := []v1beta1.CustomResourceColumnDefinition{}
	return RegisterCRD(apiClient, name, names, columns, jenkinsio.GroupName, jenkinsio.Package, jenkinsio.Version)
}

// RegisterCRD allows new custom resources to be registered using apiClient under a particular name.
// Various forms of the name are provided using names. In Kubernetes 1.11
// and later a custom display format for kubectl is used, which is specified using columns.
func RegisterCRD(apiClient apiextensionsclientset.Interface, name string,
	names *v1beta1.CustomResourceDefinitionNames, columns []v1beta1.CustomResourceColumnDefinition, groupName string,
	pkg string, version string) error {
	//"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1.PipelineActivity":
	schemaPath := fmt.Sprintf("%s/%s/%s.%s", pkg, groupName, version, names.Kind)

	schema, err := getOpenAPISchema(schemaPath)
	if err != nil {
		return errors.Wrapf(err, "error generating OpenAPI Schema for %s", schemaPath)
	}
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:                    groupName,
			Version:                  version,
			Scope:                    v1beta1.NamespaceScoped,
			Names:                    *names,
			AdditionalPrinterColumns: columns,
			Validation: &v1beta1.CustomResourceValidation{
				OpenAPIV3Schema: schema,
			},
		},
	}

	return register(apiClient, name, crd)
}

func getOpenAPIDefinitions(name string, ref common.ReferenceCallback) *common.OpenAPIDefinition {
	if def, ok := openapi.GetOpenAPIDefinitions(ref)[name]; ok {
		return &def
	}
	return nil
}

func getOpenAPISchema(defName string) (*v1beta1.JSONSchemaProps, error) {
	refCallBack := func(path string) spec.Ref {
		ref, err := jsonreference.New(path)
		if err != nil {
			log.Logger().Warnf("Error resolving ref %s %v", path, err)
		}
		return spec.Ref{
			Ref: ref,
		}
	}
	if def := getOpenAPIDefinitions(defName, refCallBack); def != nil {
		// resolve references
		schema, err := FixSchema(def.Schema, refCallBack)
		if err != nil {
			return nil, err
		}
		// Unfortunately the schema is generated into one type, and the validation takes another type.
		// However both define the OpenAPI v3 data structures and are compatible, so we i
		bytes, err := json.Marshal(schema)
		if err != nil {
			return nil, err
		}
		def1 := v1beta1.JSONSchemaProps{}
		err = json.Unmarshal(bytes, &def1)
		if err != nil {
			return nil, err
		}
		return &def1, nil
	}
	return nil, nil
}

// FixSchema walks the schema and automatically fixes it up to be better supported by Kubernetes.
// Current automatic fixes are:
// * resolving $ref
// * remove unresolved $ref
// * clear additionalProperties (this is unsupported in older kubernetes, when we drop them,
// we can investigate adding support, for now use patternProperties)
//
// as these are all unsupported
func FixSchema(schema spec.Schema, ref common.ReferenceCallback) (spec.Schema, error) {
	if schema.Type.Contains("object") {
		for k, v := range schema.Properties {
			resolved, err := FixSchema(v, ref)
			if err != nil {
				return schema, err
			}
			schema.Properties[k] = resolved
		}
		schema.AdditionalProperties = nil
	} else if schema.Type.Contains("array") {
		if schema.Items.Len() == 1 {
			resolved, err := FixSchema(*schema.Items.Schema, ref)
			if err != nil {
				return schema, err
			}
			schema.Items.Schema = &resolved
		} else {
			result := make([]spec.Schema, 0)
			for _, v := range schema.Items.Schemas {
				resolved, err := FixSchema(v, ref)
				if err != nil {
					return schema, err
				}
				result = append(result, resolved)
			}
			schema.Items.Schemas = result
		}

	} else if path := schema.Ref.String(); path != "" {
		def := getOpenAPIDefinitions(path, ref)
		if def != nil {
			return FixSchema(def.Schema, ref)
		}
		// return an empty schema if we can't resolve
		return spec.Schema{}, nil
	}
	return schema, nil
}

func register(apiClient apiextensionsclientset.Interface, name string, crd *v1beta1.CustomResourceDefinition) error {
	crdResources := apiClient.ApiextensionsV1beta1().CustomResourceDefinitions()

	f := func() error {
		old, err := crdResources.Get(name, metav1.GetOptions{})
		if err == nil {
			if !reflect.DeepEqual(&crd.Spec, old.Spec) {
				old.Spec = crd.Spec
				_, err = crdResources.Update(old)
				if err != nil {
					log.Logger().Infof("Error doing update to %s %v\n%v", old.Name, err, old.Spec)
				}
				return err
			}
			return nil
		}

		_, err = crdResources.Create(crd)
		if err != nil {
			log.Logger().Infof("Error creating %s: %v", crd.Name, err)
		}
		return err
	}

	exponentialBackOff := backoff.NewExponentialBackOff()
	timeout := 60 * time.Second
	exponentialBackOff.MaxElapsedTime = timeout
	exponentialBackOff.Reset()
	return backoff.Retry(f, exponentialBackOff)
}
