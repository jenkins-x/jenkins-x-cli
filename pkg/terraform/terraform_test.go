// +build unit

package terraform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanExtractVersion(t *testing.T) {
	t.Parallel()

	v0_7_13 := `Terraform v0.7.13

	Your version of Terraform is out of date! The latest version
	is 0.11.10. You can update by downloading from www.terraform.io`

	v0_8_8 := `Terraform v0.8.8

	Your version of Terraform is out of date! The latest version
	is 0.11.10. You can update by downloading from www.terraform.io`

	v0_9_11 := `Terraform v0.9.11

	Your version of Terraform is out of date! The latest version
	is 0.11.10. You can update by downloading from www.terraform.io`

	v0_10_8 := `Terraform v0.10.8

	Your version of Terraform is out of date! The latest version
	is 0.11.10. You can update by downloading from www.terraform.io/downloads.html`

	v0_11_10 := `Terraform v0.11.10
+ provider.google v1.19.1`

	version, err := extractVersionFromTerraformOutput(v0_7_13)
	assert.Nil(t, err, "Failed to extract version")
	assert.Equal(t, "0.7.13", version)

	version, err = extractVersionFromTerraformOutput(v0_8_8)
	assert.Nil(t, err, "Failed to extract version")
	assert.Equal(t, "0.8.8", version)

	version, err = extractVersionFromTerraformOutput(v0_9_11)
	assert.Nil(t, err, "Failed to extract version")
	assert.Equal(t, "0.9.11", version)

	version, err = extractVersionFromTerraformOutput(v0_10_8)
	assert.Nil(t, err, "Failed to extract version")
	assert.Equal(t, "0.10.8", version)

	version, err = extractVersionFromTerraformOutput(v0_11_10)
	assert.Nil(t, err, "Failed to extract version")
	assert.Equal(t, "0.11.10", version)

}
