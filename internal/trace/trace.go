// Package trace пишет JSONL-трассы запусков ask/run.
package trace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
)

// Trace представляет один файл событий текущей сессии.
type Trace struct {
	ID   string
	Path string
}

// New создаёт trace-файл и сразу пишет session_start.
func New(cfg *config.Config, kind string) (*Trace, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	id := time.Now().Format("20060102-150405") + "-" + randomSuffix()
	tr := &Trace{
		ID:   id,
		Path: filepath.Join(cfg.StateDir, "traces", id+".jsonl"),
	}
	if err := tr.Write("session_start", map[string]any{"kind": kind}); err != nil {
		return nil, err
	}
	return tr, nil
}

func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf)
}

// Write дописывает одно событие в JSONL.
func (t *Trace) Write(event string, fields map[string]any) error {
	record := map[string]any{
		"ts":    float64(time.Now().UnixNano()) / 1e9,
		"event": event,
	}
	for key, value := range fields {
		record[key] = value
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(t.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
