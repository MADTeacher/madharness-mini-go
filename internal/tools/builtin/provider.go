// Package builtin перечисляет встроенные инструменты первой ветки.
package builtin

import (
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/filetools"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/patchtool"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/searchtool"
	"github.com/MADTeacher/madharness-mini-go/internal/tools/shelltool"
)

// Provider отдаёт стандартный набор инструментов в учебном порядке.
type Provider struct{}

// Specs возвращает инструменты, доступные run-agent в минимальной ветке.
func (Provider) Specs(ctx *tools.Context) []tools.Spec {
	_ = ctx
	specs := []tools.Spec{}
	specs = append(specs, filetools.Specs()...)
	specs = append(specs, patchtool.Spec())
	specs = append(specs, searchtool.Spec())
	specs = append(specs, shelltool.Spec())
	return specs
}
