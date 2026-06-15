package hooks

import "github.com/MADTeacher/madharness-mini-go/internal/redaction"

const (
	defaultStringLimit = 2000
	defaultListLimit   = 20
	defaultDepthLimit  = 5
)

// CompactPayload обрезает большие структуры и прячет очевидные секреты.
func CompactPayload(value any) any {
	return redaction.CompactPayload(value, redaction.CompactOptions{
		StringLimit: defaultStringLimit,
		ListLimit:   defaultListLimit,
		DepthLimit:  defaultDepthLimit,
	})
}
