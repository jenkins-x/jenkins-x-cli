// +build unit

package syntax

import (
	"strings"
	"testing"
)

func TestFindDuplicates(t *testing.T) {

	tests := []struct {
		name   string
		input  []string
		errors []string
	}{
		{
			name:   "Two stage name duplicated",
			input:  []string{"Stage 1", "Stage 1", "Stage 2", "Stage 2"},
			errors: []string{"Stage 1", "Stage 2"},
		}, {
			name:   "One stage name duplicated",
			input:  []string{"Stage 1", "Stage 1"},
			errors: []string{"Stage 1"},
		}, {
			name:   "No stage name duplicated",
			input:  []string{"Stage 0", "Stage 1", "Stage 2", "Stage 3"},
			errors: nil,
		}, {
			name:   "Case-insensitive stage name duplicated",
			input:  []string{"Stage 1", "stage 1"},
			errors: []string{"Stage 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := findDuplicates(tt.input)

			if len(tt.errors) == 0 && err != nil {
				t.Fatal("Not all duplicates found A")
			}

			if len(tt.errors) > 0 && len(err.Details) > 0 {
				for _, expectedError := range tt.errors {
					if !strings.Contains(err.Details, expectedError) {
						t.Fatal("Not all duplicates found", expectedError)
					}
				}
			}

			if len(tt.errors) == 0 {
				if err != nil {
					t.Fatal("Unexpected error ", err.Details)
				}
			}

		})
	}
}
