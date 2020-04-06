// +build unit

package gke_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
)

func TestParseContext(t *testing.T) {
	type args struct {
		context string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		want2   string
		wantErr bool
	}{
		{
			name: "facelie",
			args: args{
				context: "gke_apps-dev-229310_europe-west1-b_facelie",
			},
			want:  "apps-dev-229310",
			want1: "europe-west1-b",
			want2: "facelie",
		},
		{
			name: "tekton-mole",
			args: args{
				context: "gke_jenkins-x-infra_europe-west1-c_tekton-mole",
			},
			want:  "jenkins-x-infra",
			want1: "europe-west1-c",
			want2: "tekton-mole",
		},
		{
			name: "malformed",
			args: args{
				context: "gke_jenkins_x_infra_europe-west1-c_tekton-mole",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := gke.ParseContext(tt.args.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseContext() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseContext() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("ParseContext() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
