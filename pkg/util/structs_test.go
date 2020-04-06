// +build unit

package util_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestToStringMapStringFromStruct(t *testing.T) {
	t.Parallel()

	name := "Charls"
	age := int(30)
	height := int32(6)
	eyes := int64(2)
	awesome := true
	// This is one digit greater than the float32 size limit
	hiccups := float32(101.555569)
	// This is one digit greater than the float64 size limit
	excite := 101.555555123456789
	pop := uint(500)
	cola := uint8(12)
	pizza := uint16(7)
	salad := uint32(5)
	cake := uint64(9000)
	shirts := []uint8{0x66, 0x67, 0x5a, 0x45}
	jeans := []byte{0x49, 0x43, 0x41, 0x54}

	s := struct {
		Name    string
		Age     int
		Height  int32
		Eyes    int64
		Awesome bool
		Hiccups float32
		Excite  float64
		Pop     uint
		Cola    uint8
		Pizza   uint16
		Salad   uint32
		Cake    uint64
		Shirts  []uint8
		Jeans   []byte
	}{
		name,
		age,
		height,
		eyes,
		awesome,
		hiccups,
		excite,
		pop,
		cola,
		pizza,
		salad,
		cake,
		shirts,
		jeans,
	}

	m := util.ToStringMapStringFromStruct(s)

	assert.Equal(t, 14, len(m))
	assert.Equal(t, "Charls", m["Name"])
	assert.Equal(t, "30", m["Age"])
	assert.Equal(t, "6", m["Height"])
	assert.Equal(t, "2", m["Eyes"])
	assert.Equal(t, "true", m["Awesome"])
	assert.Equal(t, "101.55557", m["Hiccups"])
	assert.Equal(t, "101.55555512345678", m["Excite"])
	assert.Equal(t, "500", m["Pop"])
	assert.Equal(t, "12", m["Cola"])
	assert.Equal(t, "7", m["Pizza"])
	assert.Equal(t, "5", m["Salad"])
	assert.Equal(t, "9000", m["Cake"])
	assert.Equal(t, "fgZE", m["Shirts"])
	assert.Equal(t, "ICAT", m["Jeans"])
}

func TestToStringMapStringFromStructDiscardUnknownTypes(t *testing.T) {
	t.Parallel()

	type yoyo struct {
		Bingo bool
	}

	woot := struct{}{}
	yo := yoyo{
		Bingo: true,
	}
	name := "Derek"

	s := struct {
		Woot struct{}
		Yo   yoyo
		Name string
	}{
		woot,
		yo,
		name,
	}

	m := util.ToStringMapStringFromStruct(s)

	assert.Equal(t, 1, len(m))
	assert.Equal(t, "Derek", m["Name"])
}

func TestToStringMapStringFromStructWithTags(t *testing.T) {
	t.Parallel()

	name := "Charls"
	age := 30
	awesome := true

	s := struct {
		Name    string `structs:"name"`
		Age     int
		Awesome bool `structs:"awesomeness_is_maxed"`
	}{
		name,
		age,
		awesome,
	}

	m := util.ToStringMapStringFromStruct(s)

	assert.Equal(t, 3, len(m))
	assert.Equal(t, "Charls", m["name"])
	assert.Equal(t, "30", m["Age"])
	assert.Equal(t, "true", m["awesomeness_is_maxed"])
}

func TestToGenericMapAndBack(t *testing.T) {
	t.Parallel()

	s := TestStruct{
		C: "Sea",
		InnerStruct: struct {
			A string
			B string
		}{
			A: "Aye",
			B: "Bee",
		},
	}

	marshalled, err := util.ToMapStringInterfaceFromStruct(s)
	unmarshalled := TestStruct{}
	util.ToStructFromMapStringInterface(marshalled, &unmarshalled)

	assert.NoError(t, err)
	assert.Equal(t, s, unmarshalled)
}

type TestStruct struct {
	C           string
	InnerStruct struct {
		A string
		B string
	}
}
