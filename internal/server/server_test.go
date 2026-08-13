package server

import (
	"encoding/json"
	"testing"
)

func TestListResponseEncodesNilAsEmptyArray(t *testing.T) {
	payload, err := json.Marshal(listResponse[string]("items", nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != `{"items":[]}` {
		t.Fatalf("expected stable empty-list contract, got %s", payload)
	}
}
