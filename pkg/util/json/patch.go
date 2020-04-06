package json

import (
	"encoding/json"
	"errors"

	"github.com/mattbaird/jsonpatch"
)

// CreatePatch creates a patch as specified in http://jsonpatch.com/.
//
// 'before' is original, 'after' is the modified struct.
// The function will return the patch as byte array.
//
// An error will be returned if any of the two structs is nil.
func CreatePatch(before, after interface{}) ([]byte, error) {
	if before == nil {
		return nil, errors.New("'before' cannot be nil when creating a JSON patch")
	}

	if after == nil {
		return nil, errors.New("'after' cannot be nil when creating a JSON patch")
	}

	rawBefore, rawAfter, err := marshallBeforeAfter(before, after)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreatePatch(rawBefore, rawAfter)
	if err != nil {
		return nil, err
	}
	jsonPatch, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	return jsonPatch, nil
}

func marshallBeforeAfter(before, after interface{}) ([]byte, []byte, error) {
	rawBefore, err := json.Marshal(before)
	if err != nil {
		return nil, nil, err
	}

	rawAfter, err := json.Marshal(after)
	if err != nil {
		return rawBefore, nil, err
	}

	return rawBefore, rawAfter, nil
}

// Patch is a slice of JsonPatchOperations
type Patch []jsonpatch.JsonPatchOperation

// MarshalJSON converts the Patch into a byte array
func (p Patch) MarshalJSON() ([]byte, error) {
	return json.Marshal([]jsonpatch.JsonPatchOperation(p))
}
