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
	if err := fs.Parse([]string{"--max-parallel-tool-calls", "3", "task"}); err != nil {
		t.Fatal(err)
	}

	options, err := flags.options()
	if err != nil {
		t.Fatal(err)
	}
	if options.MaxParallelToolCalls != 3 {
		t.Fatalf("max parallel = %d", options.MaxParallelToolCalls)
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
