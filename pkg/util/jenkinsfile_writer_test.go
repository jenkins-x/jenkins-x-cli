// +build unit

package util_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestJenkinsfileWriter(t *testing.T) {
	expectedValue := `container('maven') {
  dir('/foo/bar') {
    sh "ls -al"
    sh "mvn deploy"
  }
}
`
	writer := util.NewWriter(0)

	statements := []*util.Statement{
		{
			Function:  "container",
			Arguments: []string{"maven"},
			Children: []*util.Statement{
				{
					Function:  "dir",
					Arguments: []string{"/foo/bar"},
					Children: []*util.Statement{
						{
							Statement: "sh \"ls -al\"",
						},
					},
				},
			},
		},
		{
			Function:  "container",
			Arguments: []string{"maven"},
			Children: []*util.Statement{
				{
					Function:  "dir",
					Arguments: []string{"/foo/bar"},
					Children: []*util.Statement{
						{
							Statement: "sh \"mvn deploy\"",
						},
					},
				},
			},
		},
	}
	writer.Write(statements)
	text := writer.String()

	assert.Equal(t, expectedValue, text, "for statements %#v", statements)
}
