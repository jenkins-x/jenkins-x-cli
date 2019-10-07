// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/jenkins-x/jx/pkg/reportingtools (interfaces: XUnitClient)

package reportingtools_test

import (
	"reflect"
	"time"

	opts "github.com/jenkins-x/jx/pkg/cmd/opts"
	pegomock "github.com/petergtz/pegomock"
)

type MockXUnitClient struct {
	fail func(message string, callerSkip ...int)
}

func NewMockXUnitClient(options ...pegomock.Option) *MockXUnitClient {
	mock := &MockXUnitClient{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockXUnitClient) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockXUnitClient) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockXUnitClient) CreateHTMLReport(_param0 string, _param1 string, _param2 string) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockXUnitClient().")
	}
	params := []pegomock.Param{_param0, _param1, _param2}
	result := pegomock.GetGenericMockFrom(mock).Invoke("CreateHTMLReport", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockXUnitClient) EnsureNPMIsInstalled() error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockXUnitClient().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("EnsureNPMIsInstalled", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockXUnitClient) EnsureXUnitViewer(_param0 *opts.CommonOptions) error {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockXUnitClient().")
	}
	params := []pegomock.Param{_param0}
	result := pegomock.GetGenericMockFrom(mock).Invoke("EnsureXUnitViewer", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockXUnitClient) VerifyWasCalledOnce() *VerifierMockXUnitClient {
	return &VerifierMockXUnitClient{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockXUnitClient) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierMockXUnitClient {
	return &VerifierMockXUnitClient{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockXUnitClient) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierMockXUnitClient {
	return &VerifierMockXUnitClient{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockXUnitClient) VerifyWasCalledEventually(invocationCountMatcher pegomock.Matcher, timeout time.Duration) *VerifierMockXUnitClient {
	return &VerifierMockXUnitClient{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockXUnitClient struct {
	mock                   *MockXUnitClient
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockXUnitClient) CreateHTMLReport(_param0 string, _param1 string, _param2 string) *MockXUnitClient_CreateHTMLReport_OngoingVerification {
	params := []pegomock.Param{_param0, _param1, _param2}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CreateHTMLReport", params, verifier.timeout)
	return &MockXUnitClient_CreateHTMLReport_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockXUnitClient_CreateHTMLReport_OngoingVerification struct {
	mock              *MockXUnitClient
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockXUnitClient_CreateHTMLReport_OngoingVerification) GetCapturedArguments() (string, string, string) {
	_param0, _param1, _param2 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1], _param1[len(_param1)-1], _param2[len(_param2)-1]
}

func (c *MockXUnitClient_CreateHTMLReport_OngoingVerification) GetAllCapturedArguments() (_param0 []string, _param1 []string, _param2 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]string, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.(string)
		}
		_param1 = make([]string, len(params[1]))
		for u, param := range params[1] {
			_param1[u] = param.(string)
		}
		_param2 = make([]string, len(params[2]))
		for u, param := range params[2] {
			_param2[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockXUnitClient) EnsureNPMIsInstalled() *MockXUnitClient_EnsureNPMIsInstalled_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "EnsureNPMIsInstalled", params, verifier.timeout)
	return &MockXUnitClient_EnsureNPMIsInstalled_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockXUnitClient_EnsureNPMIsInstalled_OngoingVerification struct {
	mock              *MockXUnitClient
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockXUnitClient_EnsureNPMIsInstalled_OngoingVerification) GetCapturedArguments() {
}

func (c *MockXUnitClient_EnsureNPMIsInstalled_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockXUnitClient) EnsureXUnitViewer(_param0 *opts.CommonOptions) *MockXUnitClient_EnsureXUnitViewer_OngoingVerification {
	params := []pegomock.Param{_param0}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "EnsureXUnitViewer", params, verifier.timeout)
	return &MockXUnitClient_EnsureXUnitViewer_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockXUnitClient_EnsureXUnitViewer_OngoingVerification struct {
	mock              *MockXUnitClient
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockXUnitClient_EnsureXUnitViewer_OngoingVerification) GetCapturedArguments() *opts.CommonOptions {
	_param0 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1]
}

func (c *MockXUnitClient_EnsureXUnitViewer_OngoingVerification) GetAllCapturedArguments() (_param0 []*opts.CommonOptions) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]*opts.CommonOptions, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.(*opts.CommonOptions)
		}
	}
	return
}
