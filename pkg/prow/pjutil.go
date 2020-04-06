package prow

import (
	uuid "github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	prowapi "k8s.io/test-infra/prow/apis/prowjobs/v1"
	"k8s.io/test-infra/prow/pod-utils/decorate"
)

// NewProwJob initializes a ProwJob out of a ProwJobSpec.
func NewProwJob(spec prowapi.ProwJobSpec, labels map[string]string) prowapi.ProwJob {
	return newProwJob(spec, labels, nil)
}

// TODO pmuir copied and pasted this from prow to avoid a direct dependence on the pjutil.go file,
//  and to allow us to upgrade go.uuid
func newProwJob(spec prowapi.ProwJobSpec, extraLabels, extraAnnotations map[string]string) prowapi.ProwJob {
	labels, annotations := decorate.LabelsAndAnnotationsForSpec(spec, extraLabels, extraAnnotations)
	name, _ := uuid.NewV1()

	return prowapi.ProwJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "prow.k8s.io/v1",
			Kind:       "ProwJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name.String(),
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: spec,
		Status: prowapi.ProwJobStatus{
			StartTime: metav1.Now(),
			State:     prowapi.TriggeredState,
		},
	}
}
