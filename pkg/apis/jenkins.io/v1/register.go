package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: jenkinsio.GroupName, Version: jenkinsio.Version}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	// SchemeBuilder for building the schema :)
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme helper
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	// We only register manually written functions here. The registration of the
	// generated functions takes place in the generated files. The separation
	// makes the code compile even when the generated files are missing.
	SchemeBuilder.Register(addKnownTypes)
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&BuildPack{},
		&BuildPackList{},
		&App{},
		&AppList{},
		&CommitStatus{},
		&CommitStatusList{},
		&Environment{},
		&EnvironmentList{},
		&EnvironmentRoleBinding{},
		&EnvironmentRoleBindingList{},
		&Extension{},
		&ExtensionList{},
		&Fact{},
		&FactList{},
		&GitService{},
		&GitServiceList{},
		&PluginList{},
		&Plugin{},
		&PipelineActivity{},
		&PipelineActivityList{},
		&Scheduler{},
		&SchedulerList{},
		&PipelineStructure{},
		&PipelineStructureList{},
		&Release{},
		&ReleaseList{},
		&SourceRepository{},
		&SourceRepositoryList{},
		&SourceRepositoryGroup{},
		&SourceRepositoryGroupList{},
		&Team{},
		&TeamList{},
		&User{},
		&UserList{},
		&Workflow{},
		&WorkflowList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

type ResourceReference struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,5,opt,name=apiVersion"`
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Name of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
	// UID of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#uids
	UID types.UID `json:"uid,omitempty" protobuf:"bytes,4,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
}
