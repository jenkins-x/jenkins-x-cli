// +build unit

package dependencymatrix_test

import (
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/dependencymatrix"
)

func TestVerifyDependencyMatrixHasConsistentVersions(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "two_degree_ok",
			args: args{
				dir: filepath.Join("testdata", "two_degree_matrix"),
			},
		},
		{
			name: "two_paths_ok",
			args: args{
				dir: filepath.Join("testdata", "two_paths_matrix"),
			},
		},
		{
			name: "two_paths_inconsistent",
			args: args{
				dir: filepath.Join("testdata", "two_paths_matrix_inconsistent"),
			},
			wantErr: true,
		},
		{
			name: "two_versions_two_paths_inconsistent",
			args: args{
				dir: filepath.Join("testdata", "two_versions_two_paths_matrix_inconsistent"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := dependencymatrix.VerifyDependencyMatrixHasConsistentVersions(tt.args.dir); (err != nil) != tt.wantErr {
				t.Errorf("VerifyDependencyMatrixHasConsistentVersions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
