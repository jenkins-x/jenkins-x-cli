package vault

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/rodaine/hclencoder"
)

const (
	DenyCapability   = "deny"
	CreateCapability = "create"
	ReadCapability   = "read"
	UpdateCapability = "update"
	DeleteCapability = "delete"
	ListCapability   = "list"
	SudoCapability   = "sudo"
	RootCapability   = "root"

	PathRulesName            = "allow_secrets"
	DefaultSecretsPathPrefix = "secret/*"
	PoliciesName             = "policies"
	DefaultSecretsPath       = "secret"
)

var (
	DefaultSecretsCapabiltities = []string{CreateCapability, ReadCapability, UpdateCapability, DeleteCapability, ListCapability}
)

// PathRule defines a path rule
type PathRule struct {
	Path []PathPolicy `hcl:"path" hcle:"omitempty"`
}

// PathPolicy defiens a vault path policy
type PathPolicy struct {
	Prefix       string   `hcl:",key"`
	Capabilities []string `hcl:"capabilities" hcle:"omitempty"`
}

// String  encodes a Vault path rule to a string
func (r *PathRule) String() (string, error) {
	output, err := hclencoder.Encode(r)
	if err != nil {
		return "", errors.Wrap(err, "encodeing the path policy")
	}
	return strings.Replace(string(output), "\n", "", -1), nil
}
