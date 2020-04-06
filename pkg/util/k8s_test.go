// +build unit

package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeKubernetesNameNoEncoding(t *testing.T) {
	t.Parallel()

	original := "abcdefghijklmnopqrstuvwxyz-.0123456789"
	assert.Equal(t, original, original, "Didn't expect _any_ encoding to happen.")
}

func TestEncodeKubernetesNameUpper(t *testing.T) {
	t.Parallel()

	original := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	expected := strings.ToLower(".a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z")
	assert.Equal(t, expected, EncodeKubernetesName(original), "Expected upper->.lower conversion.")
}

func TestEncodeKubernetesPunctuationString(t *testing.T) {
	t.Parallel()

	//Note _valid_ - and . hidden in the string below.
	original := "!\"£$%^&*()-=_+¬`;'#:@~,./<>?[]{}\\|€"
	expected := ".21.22.A3.24.25.5E.26.2A.28.29-.3D.5F.2B.AC.60.3B.27.23.3A.40.7E.2C..2F.3C.3E.3F.5B.5D.7B.7D.5C.7C.20AC"
	assert.Equal(t, expected, EncodeKubernetesName(original), "Expected punctuation to be period-encoded.")
}

func TestEncodeKubernetesNameExtendedRunes(t *testing.T) {
	t.Parallel()

	original := "⌘日本語"
	expected := ".2318.65E5.672C.8A9E"
	assert.Equal(t, expected, EncodeKubernetesName(original),
		"Expected extended characters to be period-encoded.")
}
