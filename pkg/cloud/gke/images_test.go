// +build unit

package gke

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sampleOutput = `[
  {
    "digest": "sha256:2d36cccfd865cc4e958a5fb4ae6e039669f96c1ada3f6b4e4340c530e517bed1",
    "tags": [
      "0.0.0-SNAPSHOT-PR-308-3"
    ],
    "timestamp": {
      "datetime": "2019-03-09 21:06:09+00:00",
      "day": 9,
      "hour": 21,
      "microsecond": 0,
      "minute": 6,
      "month": 3,
      "second": 9,
      "year": 2019
    }
  },
  {
    "digest": "sha256:6e5c2f6591b5843e645d49f920149b0e8a9de387845521e59bec6f113f8dd058",
    "tags": [
      "0.1.279"
    ],
    "timestamp": {
      "datetime": "2019-03-09 22:34:47+00:00",
      "day": 9,
      "hour": 22,
      "microsecond": 0,
      "minute": 34,
      "month": 3,
      "second": 47,
      "year": 2019
    }
  },
  {
    "digest": "sha256:1b6cbdf1cac7071936be650d94777039b521cf9ccc0774b383cd79c6ee56f27c",
    "tags": [
      "0.0.0-SNAPSHOT-PR-293-2"
    ],
    "timestamp": {
      "datetime": "2019-03-09 19:47:37+00:00",
      "day": 9,
      "hour": 19,
      "microsecond": 0,
      "minute": 47,
      "month": 3,
      "second": 37,
      "year": 2019
    }
  },
  {
    "digest": "sha256:ae45eb93df37b4ac9fe9ebe42af044e89f74c25f0ceac8592885d61df590f730",
    "tags": [
      "0.0.0-SNAPSHOT-tekton-pipelines-2"
    ],
    "timestamp": {
      "datetime": "2019-03-09 19:33:03+00:00",
      "day": 9,
      "hour": 19,
      "microsecond": 0,
      "minute": 33,
      "month": 3,
      "second": 3,
      "year": 2019
    }
  },
  {
    "digest": "sha256:aff95ef03f6dc1472a3a79bdae42f9831d1e1b6a64f0a2ffb62d9b91d3913613",
    "tags": [
      "0.1.278"
    ],
    "timestamp": {
      "datetime": "2019-03-09 22:04:53+00:00",
      "day": 9,
      "hour": 22,
      "microsecond": 0,
      "minute": 4,
      "month": 3,
      "second": 53,
      "year": 2019
    }
  },
  {
    "digest": "sha256:9863c2227f0106002b94a7d67fe3b6ff809dd72a74877be466208299bb805a97",
    "tags": [
      "0.0.0-SNAPSHOT-PR-293-1"
    ],
    "timestamp": {
      "datetime": "2019-03-09 15:38:46+00:00",
      "day": 9,
      "hour": 15,
      "microsecond": 0,
      "minute": 38,
      "month": 3,
      "second": 46,
      "year": 2019
    }
  }
]`

func TestFindLatestImageTag(t *testing.T) {
	t.Parallel()

	version, err := FindLatestImageTag(sampleOutput)
	require.NoError(t, err, "finding latest image from input")

	t.Logf("found latest image version: %s\n", version)

	assert.Equal(t, "0.1.279", version, "finding latest image version")

}
