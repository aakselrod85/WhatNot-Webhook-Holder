package service

import (
	"encoding/json"
	"errors"
)

// validateJSONObject ensures raw is a JSON object (not an array, string, number, bool, or null).
// It does not validate anything about the object's inner shape.
func validateJSONObject(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("value must be a JSON object")
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return errors.New("value must be valid JSON")
	}
	if _, ok := v.(map[string]interface{}); !ok {
		return errors.New("value must be a JSON object")
	}
	return nil
}
