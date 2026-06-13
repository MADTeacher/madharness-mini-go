package tools

import (
	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/policy"
)

// Context хранит конфиг и policy для handlers одного запуска run.
type Context struct {
	Config *config.Config
	Policy *policy.Policy
}
