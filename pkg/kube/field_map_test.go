// +build unit

package kube

import (
	"testing"

	jenkinsio_v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestField(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipeline Activity Integration Test Suite")
}

var _ = Describe("fieldMap", func() {
	var pipelineActivity jenkinsio_v1.PipelineActivity
	var fieldMap fieldMap
	var err error

	BeforeEach(func() {
		pipelineActivity = jenkinsio_v1.PipelineActivity{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "test-pa",
				Labels: map[string]string{
					"lastCommitSHA": "5c25d20893323977de5606c10a377921fed99c1c",
				},
			},
			Spec: jenkinsio_v1.PipelineActivitySpec{
				LastCommitSHA: "5c25d20893323977de5606c10a377921fed99c1c",
			},
		}

		fieldMap, err = newFieldMap(pipelineActivity)
		Expect(err).Should(BeNil())
	})

	It("#Has returns true for existing fields", func() {
		Expect(fieldMap.Has("metadata.name")).Should(BeTrue())
		Expect(fieldMap.Has("spec.lastCommitSHA")).Should(BeTrue())
	})

	It("#Has returns false for non existing fields", func() {
		Expect(fieldMap.Has("metadata.foo")).Should(BeFalse())
		Expect(fieldMap.Has("foo")).Should(BeFalse())
	})

	It("#Has returns false for the empty string", func() {
		Expect(fieldMap.Has("")).Should(BeFalse())
	})

	It("#Has returns false for non existing fields", func() {
		Expect(fieldMap.Has("metadata.foo")).Should(BeFalse())
		Expect(fieldMap.Has("foo")).Should(BeFalse())
	})

	It("#Has returns false for the empty string", func() {
		Expect(fieldMap.Has("")).Should(BeFalse())
	})

	It("#Get returns the appropriate value for existing fields", func() {
		Expect(fieldMap.Get("metadata.name")).Should(Equal("test-pa"))
		Expect(fieldMap.Get("spec.lastCommitSHA")).Should(Equal("5c25d20893323977de5606c10a377921fed99c1c"))
	})

	It("#Get returns the empty string for non existing fields", func() {
		Expect(fieldMap.Get("foo")).Should(Equal(""))
	})
})
