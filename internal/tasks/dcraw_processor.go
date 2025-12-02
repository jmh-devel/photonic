package tasks

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"photonic/internal/config"
)

// DCrawProcessor wraps dcraw conversions.
type DCrawProcessor struct {
	config config.DCrawConfig
}

func (p *DCrawProcessor) Name() string { return "dcraw" }
func (p *DCrawProcessor) IsAvailable() bool {
	return p.config.Enabled && commandExists("dcraw")
}

func (p *DCrawProcessor) Convert(ctx context.Context, req RawConvertRequest) (RawConvertResult, error) {
	args := []string{"-c"}
	switch p.config.WhiteBalance {
	case "auto":
		args = append(args, "-a")
	case "camera":
		args = append(args, "-w")
	}
	if p.config.ColorMatrix > 0 {
		args = append(args, "-o", fmt.Sprint(p.config.ColorMatrix))
	}
	if p.config.Gamma != "" {
		args = append(args, "-g", p.config.Gamma)
	}
	if p.config.Brightness != 0 {
		args = append(args, "-b", fmt.Sprintf("%0.2f", p.config.Brightness))
	}
	args = append(args, p.config.ExtraArgs...)
	args = append(args, req.InputFile)

	cmd := exec.CommandContext(ctx, "dcraw", args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		// pipe to convert for output format
		conv := exec.CommandContext(ctx, "convert", "-", req.OutputFile)
		conv.Stdin = strings.NewReader(string(out))
		_, err = conv.CombinedOutput()
	}
	res := RawConvertResult{InputFile: req.InputFile, OutputFile: req.OutputFile, ToolUsed: "dcraw", ProcessingLog: string(out), Success: err == nil, Error: err}
	return res, err
}

func (p *DCrawProcessor) BatchConvert(ctx context.Context, files []string, outputDir string) ([]string, error) {
	var outs []string
	for _, f := range files {
		out := filepath.Join(outputDir, trimExt(filepath.Base(f))+".jpg")
		_, _ = p.Convert(ctx, RawConvertRequest{InputFile: f, OutputFile: out})
		outs = append(outs, out)
	}
	return outs, nil
}
