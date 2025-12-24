package testutil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAssertions(t *testing.T) {
	AssertEqual(t, 1, 1, "equal")
	AssertNotEqual(t, 1, 2, "not equal")
	AssertNil(t, nil, "nil")
	AssertNotNil(t, 1, "not nil")
	AssertTrue(t, true, "true")
	AssertFalse(t, false, "false")
	AssertNoError(t, nil, "no error")
	AssertContains(t, "abc", "b", "contains")
}

func TestJSONHelpers(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.WriteString(`{"ok":true}`)
	AssertStatusCode(t, rr, http.StatusOK)

	body := []byte(`{"foo":"bar"}`)
	AssertJSONContains(t, body, "foo", "bar")
	parsed := ParseJSONResponse(t, body)
	if parsed["foo"] != "bar" {
		t.Fatalf("expected parsed foo")
	}
}

func TestRequestBuilders(t *testing.T) {
	req := NewTestRequest(http.MethodGet, "/path", nil)
	if ct := req.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content type %s", ct)
	}

	data := struct {
		Name string `json:"name"`
	}{Name: "bob"}
	req2 := NewTestRequestWithJSON(t, http.MethodPost, "/p", data)
	buf := new(bytes.Buffer)
	buf.ReadFrom(req2.Body)
	if !bytes.Contains(buf.Bytes(), []byte("bob")) {
		t.Fatalf("expected json body")
	}
}

func TestRandomGenerators(t *testing.T) {
	if RandomUUID() == RandomUUID() {
		t.Fatalf("expected different uuids")
	}
	if RandomEmail() == RandomEmail() {
		t.Fatalf("expected different emails")
	}
}
