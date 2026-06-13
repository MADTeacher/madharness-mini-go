// Package imagetool проверяет локальные изображения перед vision-запросом.
package imagetool

import (
	"sort"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

// Spec описывает read_image для агентского режима run.
func Spec() tools.Spec {
	return tools.Spec{
		Name:        "read_image",
		Description: description,
		Parameters: tools.Obj(map[string]any{
			"path": tools.StrParam("", "Workspace-relative image path to inspect.", true),
			"detail": map[string]any{
				"type":        "string",
				"enum":        imageDetailValues(),
				"default":     "auto",
				"description": "Vision detail level to send when image input is enabled.",
			},
		}, []string{"path"}),
		Handler: readImage,
	}
}

func imageDetailValues() []string {
	values := make([]string, 0, len(config.ImageDetailValues))
	for value := range config.ImageDetailValues {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

const description = `Read an image file from the workspace for visual inspection.

Use this for screenshots and other local images. The tool validates a supported
PNG, JPEG, WEBP, or non-animated GIF and returns metadata only. If
supports_image_input is true, the harness attaches the image to the next model
request; if false, you must not claim you visually inspected the pixels.`
