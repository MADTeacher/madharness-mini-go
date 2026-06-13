package tools

import "testing"

func TestObservationFormat(t *testing.T) {
	if OK("x", "done", nil)["ok"] != true {
		t.Fatal("OK should set ok=true")
	}
	if Fail("x", "bad")["ok"] != false {
		t.Fatal("Fail should set ok=false")
	}
}

func TestParseToolArgs(t *testing.T) {
	name, args, err := ParseToolArgs(map[string]any{
		"function": map[string]any{"name": "read_file", "arguments": `{"path":"hello.txt"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "read_file" || args["path"] != "hello.txt" {
		t.Fatalf("name=%s args=%v", name, args)
	}
}

func TestIgnoredDetectsHiddenDirsInAbsolutePath(t *testing.T) {
	if !Ignored("/tmp/project/.git/config") {
		t.Fatal(".git path should be ignored")
	}
}
