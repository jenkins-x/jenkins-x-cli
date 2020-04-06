// +build unit

package velero

import (
	"reflect"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextentions_mocks "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
)

func TestDoesVeleroBackupScheduleExist(t *testing.T) {
	apiextensionsInterface := apiextentions_mocks.NewSimpleClientset()
	type args struct {
		apiClient clientset.Interface
		namespace string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "no-error-test",
			args: args{
				apiClient: apiextensionsInterface,
				namespace: "namespace",
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DoesVeleroBackupScheduleExist(tt.args.apiClient, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("DoesVeleroBackupScheduleExist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DoesVeleroBackupScheduleExist() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBackupsFromBackupResource(t *testing.T) {
	apiextensionsInterface := apiextentions_mocks.NewSimpleClientset()
	type args struct {
		apiClient clientset.Interface
		namespace string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "no-error-test",
			args: args{
				apiClient: apiextensionsInterface,
				namespace: "namespace",
			},
			want:    []string{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetBackupsFromBackupResource(tt.args.apiClient, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBackupsFromBackupResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBackupsFromBackupResource() got = %v, want %v", got, tt.want)
			}
		})
	}
}
