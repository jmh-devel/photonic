package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strconv"

	"photonic/internal/storage"
)

// ExtractEXIF tries exiftool -json to obtain metadata fields.
func ExtractEXIF(ctx context.Context, path string) (storage.ImageMetadata, error) {
	var meta storage.ImageMetadata
	meta.FilePath = path
	if !commandExists("exiftool") {
		return meta, nil
	}
	cmd := exec.CommandContext(ctx, "exiftool", "-json", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return meta, nil
	}
	var parsed []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil || len(parsed) == 0 {
		return meta, nil
	}
	m := parsed[0]
	if v, ok := m["Make"].(string); ok {
		meta.CameraMake = v
	}
	if v, ok := m["Model"].(string); ok {
		meta.CameraModel = v
	}
	if v, ok := m["FocalLength"].(string); ok {
		meta.FocalLength = parseFloatSuffix(v)
	}
	if v, ok := m["Aperture"].(float64); ok {
		meta.Aperture = v
	}
	if v, ok := m["ISO"].(float64); ok {
		meta.ISO = int(v)
	}
	if v, ok := m["ExposureTime"].(string); ok {
		meta.ExposureTime = v
	}
	if v, ok := m["GPSLatitude"].(float64); ok {
		meta.GPSLat = v
	}
	if v, ok := m["GPSLongitude"].(float64); ok {
		meta.GPSLon = v
	}
	if v, ok := m["DateTimeOriginal"].(string); ok {
		meta.Timestamp = v
	}
	if v, ok := m["ImageWidth"].(float64); ok {
		meta.Width = int(v)
	}
	if v, ok := m["ImageHeight"].(float64); ok {
		meta.Height = int(v)
	}
	return meta, nil
}

func parseFloatSuffix(s string) float64 {
	for len(s) > 0 && (s[len(s)-1] < '0' || s[len(s)-1] > '9') {
		s = s[:len(s)-1]
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
