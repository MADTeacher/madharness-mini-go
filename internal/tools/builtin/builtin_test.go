package builtin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/processes"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/builtin"
)

func TestBuiltinSchemasIncludeApplyPatch(t *testing.T) {
	registry := testRegistry(t)
	names := []string{}
	for _, schema := range registry.Schemas() {
		fn := schema["function"].(map[string]any)
		names = append(names, fn["name"].(string))
	}
	if !contains(names, "apply_patch") {
		t.Fatalf("schemas = %v", names)
	}
}

func TestBuiltinSchemasOrderIncludesReadImage(t *testing.T) {
	registry := testRegistry(t)
	names := []string{}
	for _, schema := range registry.Schemas() {
		fn := schema["function"].(map[string]any)
		names = append(names, fn["name"].(string))
	}
	want := []string{"list_files", "read_file", "read_image", "write_file", "apply_patch", "search_code", "run_shell", "start_shell", "shell_status", "stop_shell"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %v", names)
	}
}

func TestReadAndWriteFileTools(t *testing.T) {
	registry := testRegistry(t)
	obs := registry.Call("write_file", map[string]any{"path": "example/hello.txt", "content": "hello\n"})
	if obs["ok"] != true {
		t.Fatalf("write obs = %+v", obs)
	}
	obs = registry.Call("read_file", map[string]any{"path": "example/hello.txt", "start": 1, "end": 1})
	if obs["ok"] != true || !strings.Contains(obs["content"].(string), "1: hello") {
		t.Fatalf("read obs = %+v", obs)
	}
}

func TestWriteFileRespectsPathPolicy(t *testing.T) {
	obs := testRegistry(t).Call("write_file", map[string]any{"path": "../nope.txt", "content": "bad"})
	if obs["ok"] != false {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestListFilesAndSearchCode(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "hello.go"), []byte("package hello\nfunc SafePath() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), []byte("SECRET_TOKEN=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := registry.Call("list_files", map[string]any{"path": ".", "glob": "*.go"})
	if files["ok"] != true || len(files["files"].([]string)) != 1 {
		t.Fatalf("files = %+v", files)
	}
	protected := registry.Call("list_files", map[string]any{"path": ".", "glob": ".env"})
	if protected["ok"] != true || len(protected["files"].([]string)) != 0 {
		t.Fatalf("protected list = %+v", protected)
	}
	search := registry.Call("search_code", map[string]any{"query": "SafePath", "glob": "*.go"})
	if search["ok"] != true || len(search["results"].([]map[string]any)) != 1 {
		t.Fatalf("search = %+v", search)
	}
	secret := registry.Call("search_code", map[string]any{"query": "SECRET_TOKEN", "glob": "*"})
	if secret["ok"] != true || len(secret["results"].([]map[string]any)) != 0 {
		t.Fatalf("secret search = %+v", secret)
	}
}

func TestApplyPatchCreatesAndUpdatesFile(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	add := "*** Begin Patch\n*** Add File: hello.txt\n+one\n+two\n*** End Patch"
	obs := registry.Call("apply_patch", map[string]any{"patch": add})
	if obs["ok"] != true {
		t.Fatalf("add obs = %+v", obs)
	}
	update := "*** Begin Patch\n*** Update File: hello.txt\n@@\n one\n-two\n+TWO\n*** End Patch"
	obs = registry.Call("apply_patch", map[string]any{"patch": update})
	if obs["ok"] != true {
		t.Fatalf("update obs = %+v", obs)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.Root, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "one\nTWO\n" {
		t.Fatalf("content = %q", raw)
	}
}

func TestApplyPatchReportsContextFailure(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "hello.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: hello.txt\n@@\n-missing\n+new\n*** End Patch"
	obs := registry.Call("apply_patch", map[string]any{"patch": patch})
	if obs["ok"] != false || obs["retryable"] != true {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestRunShellTool(t *testing.T) {
	obs := testRegistry(t).Call("run_shell", map[string]any{"command": "go version"})
	if obs["ok"] != true || obs["returncode"].(int) != 0 {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestRunShellAcceptsWorkspaceRelativeCWD(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.Mkdir(filepath.Join(cfg.Root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	obs := registry.Call("run_shell", map[string]any{"command": "pwd", "cwd": "sub"})

	if obs["ok"] != true || obs["cwd"] != "sub" || obs["returncode"].(int) != 0 {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestRunShellRejectsInvalidCWD(t *testing.T) {
	obs := testRegistry(t).Call("run_shell", map[string]any{"command": "pwd", "cwd": "../outside"})
	if obs["ok"] != false {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestManagedShellToolLifecycle(t *testing.T) {
	cfg, registry, manager := testRegistryWithProcesses(t)
	defer manager.CloseAll(nil)
	command := helperCommand(t, "ready")

	start := registry.Call("start_shell", map[string]any{
		"command":               command,
		"name":                  "client",
		"ready_pattern":         "managed ready",
		"ready_timeout_seconds": 1,
	})
	if start["ok"] != true || start["process_id"] != "proc-1" || start["ready"] != true {
		t.Fatalf("start = %+v", start)
	}
	if start["cwd"] != "." || start["pid"].(int) == 0 {
		t.Fatalf("start cwd/pid = %+v", start)
	}

	status := registry.Call("shell_status", map[string]any{"name": "client"})
	if status["ok"] != true || status["running"] != true || !strings.Contains(status["stdout"].(string), "managed ready") {
		t.Fatalf("status = %+v", status)
	}

	stop := registry.Call("stop_shell", map[string]any{"process_id": "proc-1", "timeout_seconds": 1})
	if stop["ok"] != true || stop["running"] != false {
		t.Fatalf("stop = %+v", stop)
	}

	statuses, err := manager.Status("proc-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].Running {
		t.Fatalf("manager status = %+v", statuses[0])
	}
	_ = cfg
}

func TestStartShellRejectsInvalidCWD(t *testing.T) {
	_, registry, manager := testRegistryWithProcesses(t)
	defer manager.CloseAll(nil)
	obs := registry.Call("start_shell", map[string]any{"command": helperCommand(t, "ready"), "cwd": "../outside"})
	if obs["ok"] != false {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestStartShellRejectsDeniedCommand(t *testing.T) {
	cfg := testConfigForRegistry(t)
	cfg.Data.AllowShell = false
	manager := processes.NewManager()
	registry, err := tools.NewRegistryWithOptions(cfg, tools.RegistryOptions{Processes: manager}, builtin.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.CloseAll(nil)
	obs := registry.Call("start_shell", map[string]any{"command": helperCommand(t, "ready")})
	if obs["ok"] != false || !strings.Contains(obs["summary"].(string), "shell disabled") {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestStartShellRejectsDuplicateName(t *testing.T) {
	_, registry, manager := testRegistryWithProcesses(t)
	defer manager.CloseAll(nil)
	command := helperCommand(t, "ready")
	first := registry.Call("start_shell", map[string]any{"command": command, "name": "api"})
	second := registry.Call("start_shell", map[string]any{"command": command, "name": "api"})
	if first["ok"] != true || second["ok"] != false {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestShellStatusRejectsUnknownProcess(t *testing.T) {
	_, registry, manager := testRegistryWithProcesses(t)
	defer manager.CloseAll(nil)
	obs := registry.Call("shell_status", map[string]any{"process_id": "proc-missing"})
	if obs["ok"] != false {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestToolsEmitSkillResourceEvents(t *testing.T) {
	cfg := testConfigForRegistry(t)
	if err := os.MkdirAll(filepath.Join(cfg.Root, "skill", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "skill", "data.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	trace := &fakeTrace{}
	tracker := fakeResourceTracker{root: filepath.Join(cfg.Root, "skill")}
	registry, err := tools.NewRegistryWithOptions(cfg, tools.RegistryOptions{
		Trace:           trace,
		ResourceTracker: tracker,
	}, builtin.Provider{})
	if err != nil {
		t.Fatal(err)
	}

	readObs := registry.Call("read_file", map[string]any{"path": "skill/data.txt"})
	shellObs := registry.Call("run_shell", map[string]any{"command": "pwd", "cwd": "skill"})

	if readObs["ok"] != true || shellObs["ok"] != true {
		t.Fatalf("read=%+v shell=%+v", readObs, shellObs)
	}
	if trace.count("skill_resource_used") != 2 {
		t.Fatalf("events = %+v", trace.events)
	}
}

func testRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	_, registry := testRegistryWithConfig(t)
	return registry
}

func testRegistryWithConfig(t *testing.T) (*config.Config, *tools.Registry) {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := tools.NewRegistry(cfg, builtin.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	return cfg, registry
}

func testRegistryWithProcesses(t *testing.T) (*config.Config, *tools.Registry, *processes.Manager) {
	t.Helper()
	cfg := testConfigForRegistry(t)
	manager := processes.NewManager()
	registry, err := tools.NewRegistryWithOptions(cfg, tools.RegistryOptions{Processes: manager}, builtin.Provider{})
	if err != nil {
		t.Fatal(err)
	}
	return cfg, registry, manager
}

func testConfigForRegistry(t *testing.T) *config.Config {
	t.Helper()
	t.Setenv("MADHARNESS_MINI_MODEL", "")
	t.Setenv("MADHARNESS_MINI_BASE_URL", "")
	t.Setenv("MADHARNESS_MINI_API_KEY", "")
	t.Setenv("MADHARNESS_MINI_MAX_PARALLEL_TOOL_CALLS", "")
	t.Setenv("MADHARNESS_MINI_SUPPORTS_IMAGE_INPUT", "")
	t.Setenv("MADHARNESS_MINI_MAX_IMAGE_BYTES", "")
	t.Setenv("MADHARNESS_MINI_IMAGE_DETAIL", "")
	cfg, err := config.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

type fakeTrace struct {
	events []map[string]any
}

func (f *fakeTrace) Write(event string, fields map[string]any) error {
	record := map[string]any{"event": event}
	for key, value := range fields {
		record[key] = value
	}
	f.events = append(f.events, record)
	return nil
}

func (f *fakeTrace) count(event string) int {
	total := 0
	for _, item := range f.events {
		if item["event"] == event {
			total++
		}
	}
	return total
}

type fakeResourceTracker struct {
	root string
}

func (f fakeResourceTracker) ResourceEvent(path string) map[string]any {
	rel, err := filepath.Rel(f.root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil
	}
	if rel == "." {
		rel = "."
	}
	return map[string]any{"name": "test-skill", "path": filepath.ToSlash(rel), "skill_root": "skill"}
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func TestHelperBuiltinManagedShell(t *testing.T) {
	if os.Getenv("GO_WANT_BUILTIN_MANAGED_SHELL_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if mode == "ready" {
		os.Stdout.WriteString("managed ready\n")
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

func helperCommand(t *testing.T, mode string) string {
	t.Helper()
	t.Setenv("GO_WANT_BUILTIN_MANAGED_SHELL_HELPER", "1")
	return shellQuote(os.Args[0]) + " -test.run=TestHelperBuiltinManagedShell -- " + mode
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
