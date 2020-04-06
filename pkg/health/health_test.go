// +build unit

package health

import (
	"encoding/json"
	"testing"

	kh "github.com/Comcast/kuberhealthy/pkg/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckHealth(t *testing.T) {
	t.Parallel()

	ts := []struct {
		name     string
		data     string
		wantErr  bool
		errCount int
	}{
		{
			name: "KuberHealthStateNoErrors",
			data: `{
  "OK": true,
  "Errors": [],
  "CheckDetails": {
    "ComponentStatusChecker": {
      "OK": true,
      "Errors": [],
      "Namespace": "",
      "LastRun": "2019-09-26T12:56:40.691251944Z",
      "AuthorativePod": "kuberhealthy-849f8ccd74-7hjq8"
    }
  },
"CurrentMaster": "kuberhealthy-849f8ccd74-7hjq8"
}`,
			wantErr:  false,
			errCount: 0,
		},
		{
			name: "KuberHealthStateWithError",
			data: `{
  "OK": false,
  "Errors": [],
  "CheckDetails": {
    "ComponentStatusChecker": {
      "OK": false,
      "Errors": ["this is the error"],
      "Namespace": "",
      "LastRun": "2019-09-26T12:56:40.691251944Z",
      "AuthorativePod": "kuberhealthy-849f8ccd74-7hjq8"
    }
  },
"CurrentMaster": "kuberhealthy-849f8ccd74-7hjq8"
}`,
			wantErr:  true,
			errCount: 1,
		},
		{
			name: "KuberHealthStateMultipleErrors",
			data: `{
  "OK": false,
  "Errors": [],
  "CheckDetails": {
    "ComponentStatusChecker": {
      "OK": false,
      "Errors": ["this is the error"],
      "Namespace": "",
      "LastRun": "2019-09-26T12:56:40.691251944Z",
      "AuthorativePod": "kuberhealthy-849f8ccd74-7hjq8"    
    },
    "PodRestartChecker namespace jx": {
      "OK": false,
      "Errors": ["this is another error"],
      "Namespace": "jx",
      "LastRun": "2019-09-26T12:56:40.691251944Z",
      "AuthorativePod": "kuberhealthy-849f8ccd74-7hjq8"
    }
  },
  "CurrentMaster": "kuberhealthy-849f8ccd74-7hjq8"
}`,
			wantErr:  true,
			errCount: 2,
		},
	}

	for _, tt := range ts {
		t.Run(tt.name, func(t *testing.T) {
			state := kh.State{}
			err := json.Unmarshal([]byte(tt.data), &state)
			require.NoError(t, err, "could not get unmarshal data")
			err = checkHealth(state)
			if tt.wantErr {
				assert.Error(t, err)
				errors := make(map[string]kh.CheckDetails)
				err = json.Unmarshal([]byte(err.Error()), &errors)
				assert.Equal(t, len(errors), tt.errCount, "incorrect number of failures reported")
			} else {
				assert.NoError(t, err)
			}

		})
	}
}
