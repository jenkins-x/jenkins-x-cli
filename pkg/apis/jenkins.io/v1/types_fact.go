package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// Fact represents observed facts. Apps will generate Facts about the system.
// A naming schema is required since each Fact has a name that's unique for the whole system.
// Apps should prefix their generated Facts with the name of the App, like <app-name>-<fact>.
// This makes that different Apps can't possibly have conflicting Fact names.
//
// For an app generating facts on a pipeline, which will be have several different executions, we recommend <app>-<fact>-<pipeline>.
type Fact struct {
	metav1.TypeMeta `json:",inline"`
	// The Fact labels will be used to query the API for interesting Facts.
	// The Apps responsible for creating Facts need to add the relevant labels.
	// For example, creating Facts on a pipeline would create Facts with the following labels
	// {
	//   subjectkind: PipelineActivity
	//   pipelineName: my-org-my-app-master-23
	//   org: my-org
	//   repo: my-app
	//   branch: master
	//   buildNumber: 23
	// }
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   FactSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status FactStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// FactSpec is the specification of a Fact
type FactSpec struct {
	Name         string        `json:"name" protobuf:"bytes,1,opt,name=name"`
	FactType     string        `json:"factType" protobuf:"bytes,3,opt,name=factType"`
	Measurements []Measurement `json:"measurements,omitempty" protobuf:"bytes,4,opt,name=measurements"`
	// +optional
	Statements []Statement `json:"statements,omitempty" protobuf:"bytes,5,opt,name=statements"`
	// +optional
	Original Original `json:"original,omitempty" protobuf:"bytes,6,opt,name=original"`
	// +optional
	Tags             []string          `json:"tags,omitempty" protobuf:"bytes,7,opt,name=tags"`
	SubjectReference ResourceReference `json:"subject" protobuf:"bytes,8,opt,name=subject"`
}

// FactStatus is the status for an Fact resource
type FactStatus struct {
	Version string `json:"version,omitempty" protobuf:"bytes,1,opt,name=version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FactList is a list of Fact resources
type FactList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Fact `json:"items"`
}

// Measurement is a percentage or a count, something measured that the system will capture within a fact
type Measurement struct {
	Name             string   `json:"name" protobuf:"bytes,1,opt,name=name"`
	MeasurementType  string   `json:"measurementType" protobuf:"bytes,2,opt,name=measurementType"`
	MeasurementValue int      `json:"measurementValue" protobuf:"bytes,3,opt,name=measurementValue"`
	Tags             []string `json:"tags,omitempty" protobuf:"bytes,4,opt,name=tags"`
}

// Statement represents a decision that was made, for example that a promotion was approved or denied
type Statement struct {
	Name             string   `json:"name" protobuf:"bytes,1,opt,name=name"`
	StatementType    string   `json:"statementType" protobuf:"bytes,2,opt,name=statementType"`
	MeasurementValue bool     `json:"measurementValue" protobuf:"bytes,3,opt,name=measurementValue"`
	Tags             []string `json:"tags,omitempty" protobuf:"bytes,4,opt,name=tags"`
}

// Original contains the report
type Original struct {
	MimeType string   `json:"mimetype,omitempty" protobuf:"bytes,1,opt,name=mimetype"`
	URL      string   `json:"url,omitempty" protobuf:"bytes,1,opt,name=url"`
	Tags     []string `json:"tags,omitempty" protobuf:"bytes,8,opt,name=tags"`
}

// Recommended measurements for static program analysis
const (
	StaticProgramAnalysisTotalClasses   = "TotalClasses"
	StaticProgramAnalysisTotalBugs      = "TotalBugs"
	StaticProgramAnalysisHighPriority   = "High"
	StaticProgramAnalysisNormalPriority = "Normal"
	StaticProgramAnalysisLowPriority    = "Low"
	StaticProgramAnalysisIgnored        = "Ignored"
)

// Recommended measurements for code coverage
const (
	CodeCoverageMeasurementTotal    = "Total"
	CodeCoverageMeasurementMissed   = "Missed"
	CodeCoverageMeasurementCoverage = "Covered"
)

// Recommended types for code coverage count
const (
	CodeCoverageCountTypeInstructions = "Instructions"
	CodeCoverageCountTypeBranches     = "Branches"
	CodeCoverageCountTypeComplexity   = "Complexity"
	CodeCoverageCountTypeLines        = "Lines"
	CodeCoverageCountTypeMethods      = "Methods"
	CodeCoverageCountTypeClasses      = "Classes"
)

const (
	MeasurementPercent = "percent"
	MeasurementCount   = "count"
)

const (
	FactTypeCoverage              = "jx.coverage"
	FactTypeStaticProgramAnalysis = "jx.staticProgramAnalysis"
)
