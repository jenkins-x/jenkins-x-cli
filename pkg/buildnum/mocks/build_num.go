// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/jenkins-x/jx/pkg/buildnum (interfaces: BuildNumberIssuer)

package buildnum_test

import (
	kube "github.com/jenkins-x/jx/pkg/kube"
	pegomock "github.com/petergtz/pegomock"
	"reflect"
)

type MockBuildNumberIssuer struct {
	fail func(message string, callerSkip ...int)
}

func NewMockBuildNumberIssuer() *MockBuildNumberIssuer {
	return &MockBuildNumberIssuer{fail: pegomock.GlobalFailHandler}
}

func (mock *MockBuildNumberIssuer) NextBuildNumber(_param0 kube.PipelineID) (string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockBuildNumberIssuer().")
	}
	params := []pegomock.Param{_param0}
	result := pegomock.GetGenericMockFrom(mock).Invoke("NextBuildNumber", params, []reflect.Type{reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 string
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(string)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockBuildNumberIssuer) VerifyWasCalledOnce() *VerifierBuildNumberIssuer {
	return &VerifierBuildNumberIssuer{mock, pegomock.Times(1), nil}
}

func (mock *MockBuildNumberIssuer) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierBuildNumberIssuer {
	return &VerifierBuildNumberIssuer{mock, invocationCountMatcher, nil}
}

func (mock *MockBuildNumberIssuer) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierBuildNumberIssuer {
	return &VerifierBuildNumberIssuer{mock, invocationCountMatcher, inOrderContext}
}

type VerifierBuildNumberIssuer struct {
	mock                   *MockBuildNumberIssuer
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
}

func (verifier *VerifierBuildNumberIssuer) NextBuildNumber(_param0 kube.PipelineID) *BuildNumberIssuer_NextBuildNumber_OngoingVerification {
	params := []pegomock.Param{_param0}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "NextBuildNumber", params)
	return &BuildNumberIssuer_NextBuildNumber_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type BuildNumberIssuer_NextBuildNumber_OngoingVerification struct {
	mock              *MockBuildNumberIssuer
	methodInvocations []pegomock.MethodInvocation
}

func (c *BuildNumberIssuer_NextBuildNumber_OngoingVerification) GetCapturedArguments() kube.PipelineID {
	_param0 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1]
}

func (c *BuildNumberIssuer_NextBuildNumber_OngoingVerification) GetAllCapturedArguments() (_param0 []kube.PipelineID) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]kube.PipelineID, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.(kube.PipelineID)
		}
	}
	return
}
