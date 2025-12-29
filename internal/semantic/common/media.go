package common

import "strings"

// GetMediaType converts a file extension to its MIME type.
// Supports common image formats including HEIC/HEIF.
func GetMediaType(fileType string) string {
	switch strings.ToLower(fileType) {
	case ".png", "png":
		return "image/png"
	case ".jpg", ".jpeg", "jpg", "jpeg":
		return "image/jpeg"
	case ".gif", "gif":
		return "image/gif"
	case ".webp", "webp":
		return "image/webp"
	case ".heic", "heic":
		return "image/heic"
	case ".heif", "heif":
		return "image/heif"
	default:
		return "image/jpeg"
	}
}
