package workflow

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetWorkflow returns the workflow for the given name. If the name is blank it defaults to `DefaultWorkflowName`.
// If the workflow does not exist yet then its defaulted from the auto promotion workflows in order.
func GetWorkflow(name string, jxClient versioned.Interface, ns string) (*v1.Workflow, error) {
	if name == "" {
		name = DefaultWorkflowName
	}
	workflow, err := jxClient.JenkinsV1().Workflows(ns).Get(name, metav1.GetOptions{})
	if err == nil || name != DefaultWorkflowName {
		return workflow, err
	}
	return CreateDefaultWorkflow(jxClient, ns)
}

// CreateDefaultWorkflow creates the default workflow if none is provided by just chaining the Auto environments together
// sequentially
func CreateDefaultWorkflow(jxClient versioned.Interface, ns string) (*v1.Workflow, error) {
	m, names, err := kube.GetOrderedEnvironments(jxClient, ns)
	if err != nil {
		return nil, err
	}

	// lets create a default workflow
	workflow := &v1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DefaultWorkflowName,
			Namespace: ns,
		},
		Spec: v1.WorkflowSpec{},
	}
	spec := &workflow.Spec
	previousEnv := ""
	for _, name := range names {
		env := m[name]
		if env != nil && env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic && env.Spec.Kind == v1.EnvironmentKindTypePermanent {
			step := CreateWorkflowPromoteStep(name)
			if previousEnv != "" {
				step.Preconditions.Environments = []string{previousEnv}
			}
			spec.Steps = append(spec.Steps, step)
			previousEnv = name
		}
	}
	return workflow, nil
}

// CreateWorkflow creates a default Workflow instance
func CreateWorkflow(ns string, name string, steps ...v1.WorkflowStep) *v1.Workflow {
	return &v1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      naming.ToValidName(name),
			Namespace: ns,
		},
		Spec: v1.WorkflowSpec{
			Steps: steps,
		},
	}
}

// CreateWorkflowPromoteStep creates a default Workflow promote step
func CreateWorkflowPromoteStep(envName string, preconditionSteps ...v1.WorkflowStep) v1.WorkflowStep {
	answer := v1.WorkflowStep{
		Kind: v1.WorkflowStepKindTypePromote,
		Promote: &v1.PromoteWorkflowStep{
			Environment: envName,
		},
	}
	for _, preconditionStep := range preconditionSteps {
		promote := preconditionStep.Promote
		if promote != nil {
			envName := promote.Environment
			if envName != "" {
				answer.Preconditions.Environments = append(answer.Preconditions.Environments, envName)
			}
		}
	}
	return answer
}
