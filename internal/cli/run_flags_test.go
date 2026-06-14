package cli

import (
	"flag"
	"testing"
)

func TestRunFlagsUseConfigParallelDefault(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"task"}); err != nil {
		t.Fatal(err)
	}

	options, err := flags.options()
	if err != nil {
		t.Fatal(err)
	}
	if options.MaxParallelToolCalls != 0 {
		t.Fatalf("max parallel = %d", options.MaxParallelToolCalls)
	}
}

func TestRunFlagsAcceptParallelOverride(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"--max-parallel-tool-calls", "3", "--max-parallel-subagents", "2", "task"}); err != nil {
		t.Fatal(err)
	}

	options, err := flags.options()
	if err != nil {
		t.Fatal(err)
	}
	if options.MaxParallelToolCalls != 3 {
		t.Fatalf("max parallel = %d", options.MaxParallelToolCalls)
	}
	if options.MaxParallelSubagents != 2 {
		t.Fatalf("max parallel subagents = %d", options.MaxParallelSubagents)
	}
}

func TestRunFlagsRejectInvalidParallelOverride(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"--max-parallel-tool-calls", "0", "task"}); err != nil {
		t.Fatal(err)
	}

	if _, err := flags.options(); err == nil {
		t.Fatal("expected max parallel validation error")
	}
}

func TestRunFlagsRejectInvalidSubagentParallelOverride(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"--max-parallel-subagents", "0", "task"}); err != nil {
		t.Fatal(err)
	}

	if _, err := flags.options(); err == nil {
		t.Fatal("expected max parallel subagents validation error")
	}
}

func TestRunFlagsAcceptApprovalAndYolo(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"--approval", "ask", "--yolo", "task"}); err != nil {
		t.Fatal(err)
	}

	options, err := flags.options()
	if err != nil {
		t.Fatal(err)
	}
	if options.ApprovalMode != "ask" || !options.YoloMode {
		t.Fatalf("options = %+v", options)
	}
}

func TestRunFlagsRejectInvalidApprovalMode(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"--approval", "maybe", "task"}); err != nil {
		t.Fatal(err)
	}

	if _, err := flags.options(); err == nil {
		t.Fatal("expected approval validation error")
	}
}

func TestRunFlagsRejectConflictingApprovalAndYolo(t *testing.T) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	flags := registerRunFlags(fs)
	if err := fs.Parse([]string{"--approval", "deny", "--yolo", "task"}); err != nil {
		t.Fatal(err)
	}

	if _, err := flags.options(); err == nil {
		t.Fatal("expected approval/yolo conflict")
	}
}
