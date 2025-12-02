package tasks

import (
	"context"
	"os/exec"
	"path/filepath"

	"photonic/internal/config"
)

// RawTherapeeProcessor wraps rawtherapee-cli.
type RawTherapeeProcessor struct {
	config config.RawTherapeeConfig
}

func (p *RawTherapeeProcessor) Name() string { return "rawtherapee" }
func (p *RawTherapeeProcessor) IsAvailable() bool {
	return p.config.Enabled && commandExists("rawtherapee-cli")
}

func (p *RawTherapeeProcessor) Convert(ctx context.Context, req RawConvertRequest) (RawConvertResult, error) {
	args := []string{"-o", req.OutputFile, "-Y"}
	if p.config.ProcessingProfile != "" {
		args = append(args, "-p", p.config.ProcessingProfile)
	}
	if p.config.OutputProfile != "" {
		args = append(args, "-c", p.config.OutputProfile)
	}
	args = append(args, p.config.ExtraArgs...)
	args = append(args, req.InputFile)

	cmd := exec.CommandContext(ctx, "rawtherapee-cli", args...)
	out, err := cmd.CombinedOutput()
	res := RawConvertResult{InputFile: req.InputFile, OutputFile: req.OutputFile, ToolUsed: "rawtherapee-cli", ProcessingLog: string(out), Success: err == nil, Error: err}
	return res, err
}

func (p *RawTherapeeProcessor) BatchConvert(ctx context.Context, files []string, outputDir string) ([]string, error) {
	var outs []string
	for _, f := range files {
		out := filepath.Join(outputDir, trimExt(filepath.Base(f))+".jpg")
		_, _ = p.Convert(ctx, RawConvertRequest{InputFile: f, OutputFile: out})
		outs = append(outs, out)
	}
	return outs, nil
}
