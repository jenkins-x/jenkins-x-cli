// Code generated by pegomock. DO NOT EDIT.
// Source: github.com/jenkins-x/jx/pkg/jxfactory (interfaces: Factory)

package jxfactory_test

import (
	versioned "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	jxfactory "github.com/jenkins-x/jx/pkg/jxfactory"
	pegomock "github.com/petergtz/pegomock"
	versioned0 "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	kubernetes "k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"reflect"
	"time"
)

type MockFactory struct {
	fail func(message string, callerSkip ...int)
}

func NewMockFactory(options ...pegomock.Option) *MockFactory {
	mock := &MockFactory{}
	for _, option := range options {
		option.Apply(mock)
	}
	return mock
}

func (mock *MockFactory) SetFailHandler(fh pegomock.FailHandler) { mock.fail = fh }
func (mock *MockFactory) FailHandler() pegomock.FailHandler      { return mock.fail }

func (mock *MockFactory) CreateJXClient() (versioned.Interface, string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockFactory().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("CreateJXClient", params, []reflect.Type{reflect.TypeOf((*versioned.Interface)(nil)).Elem(), reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 versioned.Interface
	var ret1 string
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(versioned.Interface)
		}
		if result[1] != nil {
			ret1 = result[1].(string)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockFactory) CreateKubeClient() (kubernetes.Interface, string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockFactory().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("CreateKubeClient", params, []reflect.Type{reflect.TypeOf((*kubernetes.Interface)(nil)).Elem(), reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 kubernetes.Interface
	var ret1 string
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(kubernetes.Interface)
		}
		if result[1] != nil {
			ret1 = result[1].(string)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockFactory) CreateKubeConfig() (*rest.Config, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockFactory().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("CreateKubeConfig", params, []reflect.Type{reflect.TypeOf((**rest.Config)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 *rest.Config
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(*rest.Config)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockFactory) CreateTektonClient() (versioned0.Interface, string, error) {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockFactory().")
	}
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("CreateTektonClient", params, []reflect.Type{reflect.TypeOf((*versioned0.Interface)(nil)).Elem(), reflect.TypeOf((*string)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 versioned0.Interface
	var ret1 string
	var ret2 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(versioned0.Interface)
		}
		if result[1] != nil {
			ret1 = result[1].(string)
		}
		if result[2] != nil {
			ret2 = result[2].(error)
		}
	}
	return ret0, ret1, ret2
}

func (mock *MockFactory) ImpersonateUser(_param0 string) jxfactory.Factory {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockFactory().")
	}
	params := []pegomock.Param{_param0}
	result := pegomock.GetGenericMockFrom(mock).Invoke("ImpersonateUser", params, []reflect.Type{reflect.TypeOf((*jxfactory.Factory)(nil)).Elem()})
	var ret0 jxfactory.Factory
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(jxfactory.Factory)
		}
	}
	return ret0
}

func (mock *MockFactory) WithBearerToken(_param0 string) jxfactory.Factory {
	if mock == nil {
		panic("mock must not be nil. Use myMock := NewMockFactory().")
	}
	params := []pegomock.Param{_param0}
	result := pegomock.GetGenericMockFrom(mock).Invoke("WithBearerToken", params, []reflect.Type{reflect.TypeOf((*jxfactory.Factory)(nil)).Elem()})
	var ret0 jxfactory.Factory
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(jxfactory.Factory)
		}
	}
	return ret0
}

func (mock *MockFactory) VerifyWasCalledOnce() *VerifierMockFactory {
	return &VerifierMockFactory{
		mock:                   mock,
		invocationCountMatcher: pegomock.Times(1),
	}
}

func (mock *MockFactory) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierMockFactory {
	return &VerifierMockFactory{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
	}
}

func (mock *MockFactory) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierMockFactory {
	return &VerifierMockFactory{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		inOrderContext:         inOrderContext,
	}
}

func (mock *MockFactory) VerifyWasCalledEventually(invocationCountMatcher pegomock.Matcher, timeout time.Duration) *VerifierMockFactory {
	return &VerifierMockFactory{
		mock:                   mock,
		invocationCountMatcher: invocationCountMatcher,
		timeout:                timeout,
	}
}

type VerifierMockFactory struct {
	mock                   *MockFactory
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
	timeout                time.Duration
}

func (verifier *VerifierMockFactory) CreateJXClient() *MockFactory_CreateJXClient_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CreateJXClient", params, verifier.timeout)
	return &MockFactory_CreateJXClient_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockFactory_CreateJXClient_OngoingVerification struct {
	mock              *MockFactory
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockFactory_CreateJXClient_OngoingVerification) GetCapturedArguments() {
}

func (c *MockFactory_CreateJXClient_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockFactory) CreateKubeClient() *MockFactory_CreateKubeClient_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CreateKubeClient", params, verifier.timeout)
	return &MockFactory_CreateKubeClient_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockFactory_CreateKubeClient_OngoingVerification struct {
	mock              *MockFactory
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockFactory_CreateKubeClient_OngoingVerification) GetCapturedArguments() {
}

func (c *MockFactory_CreateKubeClient_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockFactory) CreateKubeConfig() *MockFactory_CreateKubeConfig_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CreateKubeConfig", params, verifier.timeout)
	return &MockFactory_CreateKubeConfig_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockFactory_CreateKubeConfig_OngoingVerification struct {
	mock              *MockFactory
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockFactory_CreateKubeConfig_OngoingVerification) GetCapturedArguments() {
}

func (c *MockFactory_CreateKubeConfig_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockFactory) CreateTektonClient() *MockFactory_CreateTektonClient_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "CreateTektonClient", params, verifier.timeout)
	return &MockFactory_CreateTektonClient_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockFactory_CreateTektonClient_OngoingVerification struct {
	mock              *MockFactory
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockFactory_CreateTektonClient_OngoingVerification) GetCapturedArguments() {
}

func (c *MockFactory_CreateTektonClient_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierMockFactory) ImpersonateUser(_param0 string) *MockFactory_ImpersonateUser_OngoingVerification {
	params := []pegomock.Param{_param0}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "ImpersonateUser", params, verifier.timeout)
	return &MockFactory_ImpersonateUser_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockFactory_ImpersonateUser_OngoingVerification struct {
	mock              *MockFactory
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockFactory_ImpersonateUser_OngoingVerification) GetCapturedArguments() string {
	_param0 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1]
}

func (c *MockFactory_ImpersonateUser_OngoingVerification) GetAllCapturedArguments() (_param0 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]string, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.(string)
		}
	}
	return
}

func (verifier *VerifierMockFactory) WithBearerToken(_param0 string) *MockFactory_WithBearerToken_OngoingVerification {
	params := []pegomock.Param{_param0}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "WithBearerToken", params, verifier.timeout)
	return &MockFactory_WithBearerToken_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type MockFactory_WithBearerToken_OngoingVerification struct {
	mock              *MockFactory
	methodInvocations []pegomock.MethodInvocation
}

func (c *MockFactory_WithBearerToken_OngoingVerification) GetCapturedArguments() string {
	_param0 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1]
}

func (c *MockFactory_WithBearerToken_OngoingVerification) GetAllCapturedArguments() (_param0 []string) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]string, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.(string)
		}
	}
	return
}
