package imagetool

import (
	"encoding/base64"
	"fmt"

	"github.com/MADTeacher/madharness-mini-go/internal/config"
	"github.com/MADTeacher/madharness-mini-go/internal/tools"
)

func readImage(ctx *tools.Context, args map[string]any) tools.Observation {
	rawPath := tools.StringArg(args, "path", "")
	path, err := ctx.SafePathForTool("read_image", rawPath, "read_path")
	if err != nil {
		return tools.Fail("read_image", err.Error())
	}
	detail, err := normalizeDetail(tools.StringArg(args, "detail", ctx.Config.Data.ImageDetail))
	if err != nil {
		return tools.Fail("read_image", err.Error())
	}
	release := ctx.LockWorkspaceRead("read_image", path)
	defer release()
	data, info, truncated, err := tools.ReadRegularFilePrefix(path, int64(ctx.Config.Data.MaxImageBytes))
	if err != nil {
		return tools.Fail("read_image", "not a file: "+rawPath)
	}
	size := info.Size()
	if truncated || size > int64(ctx.Config.Data.MaxImageBytes) {
		return tools.Fail("read_image", fmt.Sprintf("image is too large: %d bytes > %d", size, ctx.Config.Data.MaxImageBytes), map[string]any{
			"path":            rawPath,
			"bytes":           size,
			"max_image_bytes": ctx.Config.Data.MaxImageBytes,
		})
	}
	mimeType, err := detectMime(path, data)
	if err != nil {
		return tools.Fail("read_image", err.Error(), map[string]any{"path": rawPath, "bytes": size})
	}
	attached := ctx.Config.Data.SupportsImageInput
	reason := "supports_image_input is false"
	extra := map[string]any{}
	if attached {
		reason = "image will be attached to the next model request"
		extra["_followup_messages"] = []map[string]any{imageMessage(rawPath, data, mimeType, detail)}
	}
	extra["path"] = rawPath
	extra["mime_type"] = mimeType
	extra["bytes"] = size
	extra["attached"] = attached
	extra["detail"] = detail
	extra["reason"] = reason
	return tools.OK("read_image", "read image metadata for "+rawPath, extra)
}

func normalizeDetail(value string) (string, error) {
	if value == "" {
		value = "auto"
	}
	if !config.ImageDetailValues[value] {
		return "", fmt.Errorf("invalid image detail: %s; allowed: auto, high, low, original", value)
	}
	return value, nil
}

func imageMessage(path string, data []byte, mimeType string, detail string) map[string]any {
	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return map[string]any{
		"role": "user",
		"content": []map[string]any{
			{"type": "text", "text": "Image from read_image is attached: " + path},
			{"type": "image_url", "image_url": map[string]any{"url": dataURL, "detail": detail}},
		},
	}
}
