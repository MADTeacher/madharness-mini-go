package builtin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x00\x00\x00\x00\x00")

func TestReadImageToolReturnsMetadataWithoutBase64(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "shot.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	obs := registry.Call("read_image", map[string]any{"path": "shot.png"})

	if obs["ok"] != true || obs["mime_type"] != "image/png" || obs["attached"] != false {
		t.Fatalf("obs = %+v", obs)
	}
	if obs["bytes"] != int64(len(pngBytes)) || obs["detail"] != "auto" {
		t.Fatalf("obs = %+v", obs)
	}
	encoded, _ := json.Marshal(obs)
	if strings.Contains(string(encoded), "data:image") || strings.Contains(string(encoded), "base64") {
		t.Fatalf("observation leaked image payload: %s", encoded)
	}
}

func TestReadImageToolReturnsFollowupWhenEnabled(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	cfg.Data.SupportsImageInput = true
	if err := os.WriteFile(filepath.Join(cfg.Root, "shot.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	obs, followups := registry.CallWithFollowups("read_image", map[string]any{"path": "shot.png", "detail": "high"})

	if obs["ok"] != true || obs["attached"] != true || obs["detail"] != "high" {
		t.Fatalf("obs = %+v", obs)
	}
	if len(followups) != 1 {
		t.Fatalf("followups = %+v", followups)
	}
	encodedObs, _ := json.Marshal(obs)
	encodedFollowups, _ := json.Marshal(followups)
	if strings.Contains(string(encodedObs), "data:image") || !strings.Contains(string(encodedFollowups), "data:image/png;base64,") {
		t.Fatalf("obs=%s followups=%s", encodedObs, encodedFollowups)
	}
}

func TestReadImageToolRejectsPolicyAndBadInputs(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, ".env"), pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "bad.txt"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "shot.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []map[string]any{
		{"path": ".env"},
		{"path": "../shot.png"},
		{"path": "bad.txt"},
		{"path": "shot.png", "detail": "microscope"},
	}
	for _, args := range cases {
		obs := registry.Call("read_image", args)
		if obs["ok"] != false {
			t.Fatalf("args=%v obs=%+v", args, obs)
		}
	}
}

func TestReadImageToolRejectsLargeAndAnimatedGIF(t *testing.T) {
	cfg, registry := testRegistryWithConfig(t)
	cfg.Data.MaxImageBytes = 4
	if err := os.WriteFile(filepath.Join(cfg.Root, "shot.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if obs := registry.Call("read_image", map[string]any{"path": "shot.png"}); obs["ok"] != false || !strings.Contains(obs["summary"].(string), "too large") {
		t.Fatalf("large obs = %+v", obs)
	}

	cfg.Data.MaxImageBytes = 5_000_000
	if err := os.WriteFile(filepath.Join(cfg.Root, "animated.gif"), animatedGIFBytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if obs := registry.Call("read_image", map[string]any{"path": "animated.gif"}); obs["ok"] != false || !strings.Contains(obs["summary"].(string), "animated GIF") {
		t.Fatalf("gif obs = %+v", obs)
	}
}

func animatedGIFBytes() []byte {
	header := []byte("GIF89a")
	screen := []byte{1, 0, 1, 0, 0, 0, 0}
	frame := []byte{0x2C, 0, 0, 0, 0, 1, 0, 1, 0, 0, 2, 0}
	out := append([]byte{}, header...)
	out = append(out, screen...)
	out = append(out, frame...)
	out = append(out, frame...)
	out = append(out, 0x3B)
	return out
}
