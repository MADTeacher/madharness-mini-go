package imagetool

import (
	"fmt"
	"path/filepath"
	"strings"
)

var imageMIMEByExt = map[string]string{
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

func detectMime(path string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	mimeType := imageMIMEByExt[ext]
	if mimeType == "" {
		if ext == "" {
			ext = "(no extension)"
		}
		return "", fmt.Errorf("unsupported image type: %s", ext)
	}
	switch mimeType {
	case "image/png":
		if strings.HasPrefix(string(data), "\x89PNG\r\n\x1a\n") {
			return mimeType, nil
		}
	case "image/jpeg":
		if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
			return mimeType, nil
		}
	case "image/webp":
		if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return mimeType, nil
		}
	case "image/gif":
		return detectGIF(data, mimeType)
	}
	return "", fmt.Errorf("file signature does not match %s", mimeType)
}

func detectGIF(data []byte, mimeType string) (string, error) {
	if !(strings.HasPrefix(string(data), "GIF87a") || strings.HasPrefix(string(data), "GIF89a")) {
		return "", fmt.Errorf("file signature does not match %s", mimeType)
	}
	animated, err := gifIsAnimated(data)
	if err != nil {
		return "", err
	}
	if animated {
		return "", fmt.Errorf("animated GIF is not supported")
	}
	return mimeType, nil
}
