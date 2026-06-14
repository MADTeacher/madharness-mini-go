package processes

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManagerStartMatchesReadyPattern(t *testing.T) {
	manager := NewManager()
	status, err := manager.Start(helperStartOptions(t, "ready", StartOptions{
		Command:      "helper ready",
		Name:         "api",
		ReadyPattern: "server ready",
		ReadyTimeout: time.Second,
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.CloseAll(nil)
	if status.ProcessID != "proc-1" || status.Name != "api" || !status.Running || !status.Ready {
		t.Fatalf("status = %+v", status)
	}
	if !strings.Contains(status.Stdout, "server ready") {
		t.Fatalf("stdout = %q", status.Stdout)
	}
}

func TestManagerStatusListsProcesses(t *testing.T) {
	manager := NewManager()
	if _, err := manager.Start(helperStartOptions(t, "ready", StartOptions{Name: "one"})); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(helperStartOptions(t, "ready", StartOptions{Name: "two"})); err != nil {
		t.Fatal(err)
	}
	defer manager.CloseAll(nil)
	statuses, err := manager.Status("", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestManagerRejectsDuplicateLiveName(t *testing.T) {
	manager := NewManager()
	if _, err := manager.Start(helperStartOptions(t, "ready", StartOptions{Name: "api"})); err != nil {
		t.Fatal(err)
	}
	defer manager.CloseAll(nil)
	if _, err := manager.Start(helperStartOptions(t, "ready", StartOptions{Name: "api"})); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestManagerReportsEarlyExitBeforeReady(t *testing.T) {
	manager := NewManager()
	status, err := manager.Start(helperStartOptions(t, "exit", StartOptions{
		ReadyPattern: "never",
		ReadyTimeout: time.Second,
	}))
	if err != ErrExitedBeforeReady {
		t.Fatalf("err = %v", err)
	}
	if status.Running || status.ExitCode == nil || *status.ExitCode != 7 {
		t.Fatalf("status = %+v", status)
	}
}

func TestManagerReadyTimeoutStopsLiveProcess(t *testing.T) {
	manager := NewManager()
	status, err := manager.Start(helperStartOptions(t, "ready", StartOptions{
		Name:         "slow",
		ReadyPattern: "never",
		ReadyTimeout: 50 * time.Millisecond,
	}))
	if err != ErrReadyTimeout {
		t.Fatalf("err = %v", err)
	}
	if status.Running || status.Ready {
		t.Fatalf("status = %+v", status)
	}
	if _, err := manager.Start(helperStartOptions(t, "ready", StartOptions{Name: "slow"})); err != nil {
		t.Fatalf("name was not released after timeout: %v", err)
	}
	manager.CloseAll(nil)
}

func TestManagerBuffersOutput(t *testing.T) {
	manager := NewManager()
	status, err := manager.Start(helperStartOptions(t, "big-output", StartOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	waitForExit(t, manager, status.ProcessID)
	statuses, err := manager.Status(status.ProcessID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses[0].Stdout) > bufferLimit {
		t.Fatalf("stdout len = %d", len(statuses[0].Stdout))
	}
	if !strings.Contains(statuses[0].Stdout, "tail-marker") {
		t.Fatalf("stdout tail missing")
	}
}

func TestManagerStopKillsInterruptIgnoringProcess(t *testing.T) {
	manager := NewManager()
	status, err := manager.Start(helperStartOptions(t, "ignore-interrupt", StartOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := manager.Stop(StopOptions{ProcessID: status.ProcessID, Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Running {
		t.Fatalf("process still running: %+v", stopped)
	}
}

func TestManagerCloseAllStopsRunningProcesses(t *testing.T) {
	manager := NewManager()
	marker := filepath.Join(t.TempDir(), "closed.txt")
	status, err := manager.Start(helperStartOptions(t, "mark-on-interrupt", StartOptions{
		Command:      marker,
		ReadyPattern: "mark helper ready",
		ReadyTimeout: time.Second,
	}))
	if err != nil {
		t.Fatal(err)
	}
	manager.CloseAll(nil)
	waitForExit(t, manager, status.ProcessID)
	if text, err := os.ReadFile(marker); err != nil || string(text) != "closed" {
		t.Fatalf("marker text = %q err=%v", text, err)
	}
}

func TestHelperProcessManager(t *testing.T) {
	if os.Getenv("GO_WANT_PROCESS_MANAGER_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "ready":
		fmt.Println("server ready")
		time.Sleep(30 * time.Second)
	case "exit":
		fmt.Println("starting")
		os.Exit(7)
	case "big-output":
		fmt.Print(strings.Repeat("x", bufferLimit+1000))
		fmt.Print("tail-marker")
	case "ignore-interrupt":
		ignoreInterrupt()
		time.Sleep(30 * time.Second)
	case "mark-on-interrupt":
		marker := os.Args[len(os.Args)-2]
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		fmt.Println("mark helper ready")
		<-ch
		_ = os.WriteFile(marker, []byte("closed"), 0o644)
	case "spawn-descendants":
		marker := os.Args[len(os.Args)-2]
		cmd := helperCommandForMode(marker, "child-spawn-grandchild")
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		_ = os.WriteFile(marker+".child", []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
		waitForHelperFile(marker+".grandchild", 2*time.Second)
		fmt.Println("descendants ready")
		time.Sleep(30 * time.Second)
	case "child-spawn-grandchild":
		marker := os.Args[len(os.Args)-2]
		cmd := helperCommandForMode(marker, "grandchild-ignore-interrupt")
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		_ = os.WriteFile(marker+".grandchild", []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
		time.Sleep(30 * time.Second)
	case "grandchild-ignore-interrupt":
		ignoreInterrupt()
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func helperStartOptions(t *testing.T, mode string, options StartOptions) StartOptions {
	t.Helper()
	t.Setenv("GO_WANT_PROCESS_MANAGER_HELPER", "1")
	if options.Command == "" {
		options.Command = "helper " + mode
	}
	args := []string{"-test.run=TestHelperProcessManager", "--"}
	if mode == "mark-on-interrupt" || mode == "spawn-descendants" {
		args = append(args, options.Command)
	}
	args = append(args, mode)
	options.Argv = append([]string{os.Args[0]}, args...)
	if options.CWD == "" {
		options.CWD = t.TempDir()
	}
	if options.CWDDisplay == "" {
		options.CWDDisplay = "."
	}
	return options
}

func waitForExit(t *testing.T, manager *Manager, processID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statuses, err := manager.Status(processID, "")
		if err != nil {
			t.Fatal(err)
		}
		if !statuses[0].Running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process did not exit: %s", processID)
}

func ignoreInterrupt() {
	signal.Ignore(os.Interrupt)
}

func waitInterrupt() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
}

func helperCommandForMode(marker string, mode string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessManager", "--", marker, mode)
	cmd.Env = append(os.Environ(), "GO_WANT_PROCESS_MANAGER_HELPER=1")
	return cmd
}

func waitForHelperFile(path string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
