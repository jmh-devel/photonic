package tasks

import (
	"bytes"
	"os/exec"
	"strings"
)

// identifyDimensions attempts to get WxH via ImageMagick identify; returns empty on failure.
func identifyDimensions(path string) string {
	cmd := exec.Command("identify", "-format", "%wx%h", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// commandExists checks presence of an executable in PATH.
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
