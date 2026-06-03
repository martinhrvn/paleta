package main

import (
	"encoding/json"
	"testing"

	"github.com/martinhrvn/go-pm/internal/commands"
)

func TestVersionString(t *testing.T) {
	old := version
	defer func() { version = old }()

	version = "1.2.3"
	got := versionString()
	want := "gopm version 1.2.3"
	if got != want {
		t.Errorf("versionString() = %q, want %q", got, want)
	}
}

// decode unmarshals JSON output into a generic value for assertions.
func decode(t *testing.T, data []byte, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, data)
	}
}

func TestMarshalSelectionSingle(t *testing.T) {
	results := []commands.SelectionResult{
		{Directory: "/tmp/app", Command: "npm test", DisplayName: "app"},
	}

	data, err := marshalSelection(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A single selection must marshal to an object (not an array) for
	// backward compatibility with the shell wrapper.
	var obj map[string]interface{}
	decode(t, data, &obj)

	if obj["directory"] != "/tmp/app" {
		t.Errorf("directory = %v, want /tmp/app", obj["directory"])
	}
	if obj["command"] != "npm test" {
		t.Errorf("command = %v, want 'npm test'", obj["command"])
	}
	if obj["display_name"] != "app" {
		t.Errorf("display_name = %v, want app", obj["display_name"])
	}
	// action must be omitted when not "edit"
	if _, ok := obj["action"]; ok {
		t.Errorf("action should be omitted for execute, got %v", obj["action"])
	}
}

func TestMarshalSelectionEditAction(t *testing.T) {
	results := []commands.SelectionResult{
		{Directory: ".", Command: "edit", DisplayName: "config", Action: "edit"},
	}

	data, err := marshalSelection(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]interface{}
	decode(t, data, &obj)

	if obj["action"] != "edit" {
		t.Errorf("action = %v, want edit", obj["action"])
	}
}

func TestMarshalSelectionMultiple(t *testing.T) {
	results := []commands.SelectionResult{
		{Directory: "/a", Command: "make build", DisplayName: "a"},
		{Directory: "/b", Command: "make test", DisplayName: "b"},
	}

	data, err := marshalSelection(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multiple selections must marshal to an array.
	var arr []map[string]interface{}
	decode(t, data, &arr)

	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
	if arr[0]["command"] != "make build" || arr[1]["command"] != "make test" {
		t.Errorf("unexpected commands: %v, %v", arr[0]["command"], arr[1]["command"])
	}
}

func TestMarshalSelectionWithEnv(t *testing.T) {
	results := []commands.SelectionResult{
		{
			Directory:   "/app",
			Command:     "npm run dev",
			DisplayName: "app",
			Env:         map[string]string{"PORT": "3001", "DEBUG": "1"},
		},
	}

	data, err := marshalSelection(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]interface{}
	decode(t, data, &obj)

	// command stays raw — env must NOT be baked into it (keeps history key stable)
	if obj["command"] != "npm run dev" {
		t.Errorf("command = %v, want raw 'npm run dev'", obj["command"])
	}

	env, ok := obj["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("env missing or wrong type: %v", obj["env"])
	}
	if env["PORT"] != "3001" || env["DEBUG"] != "1" {
		t.Errorf("env = %v, want PORT=3001 DEBUG=1", env)
	}
}

func TestMarshalSelectionOmitsEmptyEnv(t *testing.T) {
	results := []commands.SelectionResult{
		{Directory: "/app", Command: "npm test", DisplayName: "app"},
	}

	data, err := marshalSelection(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var obj map[string]interface{}
	decode(t, data, &obj)

	if _, ok := obj["env"]; ok {
		t.Errorf("env should be omitted when empty, got %v", obj["env"])
	}
}

// TestMarshalSelectionSpecialChars is the regression test for the old
// hand-rolled escaper, which only handled \\ " \n \t and produced invalid
// JSON for other control characters and unicode.
func TestMarshalSelectionSpecialChars(t *testing.T) {
	tricky := "say \"hi\"\tand\\or\r\nnew\bline \x01 ☃"
	results := []commands.SelectionResult{
		{Directory: "/p", Command: tricky, DisplayName: "weird"},
	}

	data, err := marshalSelection(results)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Round-trip: the decoded command must exactly equal the original,
	// proving the escaping is correct and reversible.
	var obj map[string]interface{}
	decode(t, data, &obj)

	if obj["command"] != tricky {
		t.Errorf("round-trip mismatch:\n got: %q\nwant: %q", obj["command"], tricky)
	}
}
