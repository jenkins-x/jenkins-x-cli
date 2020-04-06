// +build unit

package step_test

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/cmd/step"
)

func TestCollapseDependencyUpdates(t *testing.T) {
	type args struct {
		data string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "platform",
			args: args{
				data: "test_data/changelog/platform.yaml",
			},
			want: "test_data/changelog/platform.golden.yaml",
		},
		{
			// This one was a real regression seen in environment controller
			name: "environment-controller",
			args: args{
				data: "test_data/changelog2/platform.yaml",
			},
			want: "test_data/changelog2/platform.golden.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ups := make([]v1.DependencyUpdate, 0)
			data, err := ioutil.ReadFile(tt.args.data)
			assert.NoError(t, err)
			err = yaml.Unmarshal(data, &ups)
			assert.NoError(t, err)
			out := step.CollapseDependencyUpdates(ups)
			dataOut, err := yaml.Marshal(out)
			assert.NoError(t, err)
			golden, err := ioutil.ReadFile(tt.want)
			assert.NoError(t, err)
			assert.Equal(t, string(golden), string(dataOut))
		})
	}
}
