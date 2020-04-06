// +build unit

package gke

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRegionFromZone(t *testing.T) {
	t.Parallel()
	r := GetRegionFromZone("europe-west1-b")
	assert.Equal(t, r, "europe-west1")

	r = GetRegionFromZone("us-west1-d")
	assert.Equal(t, r, "us-west1")

	r = GetRegionFromZone("us-west1")
	assert.Equal(t, r, "us-west1")
}

func TestGetManagedZoneName(t *testing.T) {
	t.Parallel()
	d := generateManagedZoneName("wine.cheese.co.uk")
	assert.Equal(t, d, "wine-cheese-co-uk-zone")

	d = generateManagedZoneName("planes.n.trains.com")
	assert.Equal(t, d, "planes-n-trains-com-zone")
}
