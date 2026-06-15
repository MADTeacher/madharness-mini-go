package redaction

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type corpusCase struct {
	Name    string `json:"name"`
	Payload any    `json:"payload"`
	Leaks   []string
	Keeps   []string
}

func TestRedactPayloadUsesSharedSecretCorpus(t *testing.T) {
	for _, tc := range loadCorpus(t) {
		t.Run(tc.Name, func(t *testing.T) {
			rendered := renderJSON(t, RedactPayload(tc.Payload))
			assertRedactionCorpus(t, rendered, tc.Leaks, tc.Keeps)
		})
	}
}

func TestCompactPayloadUsesSharedSecretCorpus(t *testing.T) {
	for _, tc := range loadCorpus(t) {
		t.Run(tc.Name, func(t *testing.T) {
			rendered := renderJSON(t, CompactPayload(tc.Payload, CompactOptions{
				StringLimit: 2000,
				ListLimit:   20,
				DepthLimit:  5,
			}))
			assertRedactionCorpus(t, rendered, tc.Leaks, tc.Keeps)
		})
	}
}

func TestCompactPayloadClipsLargeValues(t *testing.T) {
	payload := map[string]any{
		"text":  strings.Repeat("x", 2100),
		"items": manyItems(25),
	}

	compact, ok := CompactPayload(payload, CompactOptions{
		StringLimit: 2000,
		ListLimit:   20,
		DepthLimit:  5,
	}).(map[string]any)
	if !ok {
		t.Fatalf("unexpected compact type %#v", compact)
	}
	if !strings.Contains(compact["text"].(string), "...[clipped") {
		t.Fatalf("text was not clipped: %q", compact["text"])
	}
	items := compact["items"].([]any)
	if len(items) != 21 || !strings.Contains(items[20].(string), "<clipped 5 items>") {
		t.Fatalf("items = %#v", items)
	}
}

func loadCorpus(t *testing.T) []corpusCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []corpusCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func renderJSON(t *testing.T, value any) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String())
}

func assertRedactionCorpus(t *testing.T, rendered string, leaks []string, keeps []string) {
	t.Helper()
	for _, leak := range leaks {
		if strings.Contains(rendered, leak) {
			t.Fatalf("payload leaked %q: %s", leak, rendered)
		}
	}
	for _, keep := range keeps {
		if !strings.Contains(rendered, keep) {
			t.Fatalf("payload lost useful context %q: %s", keep, rendered)
		}
	}
}

func manyItems(count int) []any {
	items := make([]any, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, i)
	}
	return items
}
