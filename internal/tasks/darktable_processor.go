package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"photonic/internal/config"
)

// DarktableProcessor wraps darktable-cli execution.
type DarktableProcessor struct {
	config config.DarktableConfig
}

func (p *DarktableProcessor) Name() string { return "darktable" }

func (p *DarktableProcessor) IsAvailable() bool {
	return p.config.Enabled && commandExists("darktable-cli")
}

func (p *DarktableProcessor) Convert(ctx context.Context, req RawConvertRequest) (RawConvertResult, error) {
	outputDir := filepath.Dir(req.OutputFile)

	// Check if something exists at the output directory path
	if stat, err := os.Stat(outputDir); err == nil && !stat.IsDir() {
		err := fmt.Errorf("cannot create output directory %s: file exists with same name (try using a different --output path)", outputDir)
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "darktable-cli",
			Success:    false,
			Error:      err,
		}, err
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "darktable-cli",
			Success:    false,
			Error:      fmt.Errorf("failed to create output directory: %v", err),
		}, err
	}

	args := []string{req.InputFile}

	// Check for custom .xmp file first (user's custom processing)
	// Try both .xmp and .CR2.xmp naming conventions
	xmpFile := req.InputFile + ".xmp" // Try .CR2.xmp first
	var hasCustomXMP bool

	if fileExists(xmpFile) {
		fmt.Printf("DEBUG: Found custom .xmp file: %s\n", xmpFile)
		args = append(args, xmpFile)
		hasCustomXMP = true
	} else {
		// Try alternative naming: replace extension with .xmp
		altXmpFile := strings.TrimSuffix(req.InputFile, filepath.Ext(req.InputFile)) + ".xmp"
		if fileExists(altXmpFile) {
			fmt.Printf("DEBUG: Found custom .xmp file: %s\n", altXmpFile)
			args = append(args, altXmpFile)
			hasCustomXMP = true
		} else if req.XMPFile != "" && fileExists(req.XMPFile) {
			fmt.Printf("DEBUG: Using provided .xmp file: %s\n", req.XMPFile)
			args = append(args, req.XMPFile)
			hasCustomXMP = true
		} else {
			fmt.Printf("DEBUG: No custom .xmp file found, using automatic processing\n")
		}
	}

	args = append(args, outputDir)

	if !p.config.ApplyPresets {
		args = append(args, "--apply-custom-presets", "false")
	}
	if p.config.HighQuality {
		args = append(args, "--hq", "true")
	}
	if p.config.Width > 0 {
		args = append(args, "--width", fmt.Sprint(p.config.Width))
	}
	if p.config.Height > 0 {
		args = append(args, "--height", fmt.Sprint(p.config.Height))
	}
	if p.config.Style != "" {
		args = append(args, "--style", p.config.Style)
		if p.config.StyleOverwrite {
			args = append(args, "--style-overwrite")
		}
	}
	if p.config.ExportMasks {
		args = append(args, "--export_masks", "true")
	}

	// Only apply automatic enhancements if no custom .xmp file exists
	// (respect user's custom processing when .xmp is present)
	if !hasCustomXMP {
		fmt.Printf("DEBUG: Applying automatic enhancements (no custom .xmp found)\n")
		// For darktable-cli, we'll rely on styles and presets rather than individual --iop params
		// which can be unreliable. Users should create .xmp files for custom processing.
		if p.config.AutoExposure || p.config.AutoWhiteBalance {
			fmt.Printf("DEBUG: For auto exposure/white balance, recommend creating .xmp files in darktable GUI\n")
		}
	} else {
		fmt.Printf("DEBUG: Using custom .xmp processing - your darktable settings will be applied\n")
	}

	ext := strings.TrimPrefix(filepath.Ext(req.OutputFile), ".")
	if ext == "" {
		ext = "jpg"
	}
	args = append(args, "--out-ext", ext)
	args = append(args, p.config.ExtraArgs...)

	cmd := exec.CommandContext(ctx, "darktable-cli", args...)
	cmd.Env = []string{
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
		"LD_LIBRARY_PATH=" + os.Getenv("LD_LIBRARY_PATH"),
		"DISPLAY=",
		"WAYLAND_DISPLAY=",
		"XDG_RUNTIME_DIR=",
		"QT_QPA_PLATFORM=offscreen",
		"GDK_BACKEND=x11",
	}
	tempConfigDir := filepath.Join(req.TempDir, "darktable-config")
	if err := os.MkdirAll(tempConfigDir, 0o755); err == nil {
		cmd.Env = append(cmd.Env, "XDG_CONFIG_HOME="+tempConfigDir)
	}

	output, err := cmd.CombinedOutput()
	res := RawConvertResult{InputFile: req.InputFile, OutputFile: req.OutputFile, ToolUsed: "darktable-cli", ProcessingLog: string(output), Success: err == nil, Error: err}
	if res.Success && !fileExists(req.OutputFile) {
		expectedName := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(req.InputFile), filepath.Ext(req.InputFile))+"."+ext)
		if fileExists(expectedName) {
			res.OutputFile = expectedName
		} else {
			res.Success = false
			res.Error = fmt.Errorf("darktable-cli completed but output file not found")
		}
	}
	return res, err
}

func (p *DarktableProcessor) BatchConvert(ctx context.Context, files []string, outputDir string) ([]string, error) {
	var outs []string
	for _, f := range files {
		out := filepath.Join(outputDir, trimExt(filepath.Base(f))+".jpg")
		_, _ = p.Convert(ctx, RawConvertRequest{InputFile: f, OutputFile: out})
		outs = append(outs, out)
	}
	return outs, nil
}
