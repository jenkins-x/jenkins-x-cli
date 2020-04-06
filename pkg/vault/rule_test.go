// +build unit

package vault_test

import (
	"testing"

	"github.com/hashicorp/hcl"
	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/stretchr/testify/assert"
)

func TestEncodeVaultPathRule(t *testing.T) {
	tests := map[string]struct {
		rule *vault.PathRule
		err  bool
	}{
		"marshal policy": {
			rule: &vault.PathRule{
				Path: []vault.PathPolicy{{
					Prefix:       "secrets/*",
					Capabilities: []string{vault.CreateCapability, vault.ReadCapability},
				}},
			},
			err: false,
		},
		"marshal empty policy": {
			rule: &vault.PathRule{
				Path: []vault.PathPolicy{{
					Prefix:       "",
					Capabilities: []string{},
				}},
			},
			err: false,
		},
		"marshal nil policy with error": {
			rule: nil,
			err:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			output, err := tc.rule.String()
			if tc.err {
				assert.Error(t, err, "should encode policy with an error")
			} else {
				assert.NoError(t, err, "should encode policy without error")
				var rule vault.PathRule
				err = hcl.Decode(&rule, output)
				assert.NoError(t, err, "should decode policy without error")
				assert.Equal(t, *tc.rule, rule)
			}
		})
	}
}
