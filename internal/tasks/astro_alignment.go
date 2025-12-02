package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"photonic/internal/config"
)

// AstroAlignmentProcessor handles star-field alignment.
type AstroAlignmentProcessor struct {
	config config.AstroAlignmentConfig
}

func (p *AstroAlignmentProcessor) Name() string { return "astro" }
func (p *AstroAlignmentProcessor) SupportsType(alignType AlignmentType) bool {
	return alignType == AlignmentAstro
}
func (p *AstroAlignmentProcessor) IsAvailable() bool {
	if !p.config.Enabled {
		return false
	}
	// prefer siril, fall back to astroalign stub
	return commandExists("siril") || commandExists("siril-cli") || commandExists("astroalign") || commandExists("align_image_stack")
}

func (p *AstroAlignmentProcessor) Align(ctx context.Context, req AlignmentRequest) (AlignmentResult, error) {
	start := time.Now()
	if commandExists("siril") || commandExists("siril-cli") {
		res, err := p.alignWithSiril(ctx, req, start)
		if err == nil {
			return res, nil
		}
	}
	if commandExists("astroalign") {
		res, err := p.alignWithAstroAlign(ctx, req, start)
		if err == nil {
			return res, nil
		}
	}
	if commandExists("align_image_stack") {
		// Create output directory
		if err := os.MkdirAll(req.OutputDir, 0755); err != nil {
			return AlignmentResult{ToolUsed: "align_image_stack", ProcessingTime: time.Since(start), Success: false, Error: err}, err
		}

		args := []string{"-a", filepath.Join(req.OutputDir, "astro_aligned_")}
		args = append(args, req.Images...)
		cmd := exec.CommandContext(ctx, "align_image_stack", args...)
		out, err := cmd.CombinedOutput()

		// Find the generated files and rename them to our expected format
		if err == nil {
			// align_image_stack creates files like "astro_aligned_0000.tif", "astro_aligned_0001.tif", etc.
			// We want to rename them to "aligned_000_originalname.tif" format
			for i, imgPath := range req.Images {
				srcFile := filepath.Join(req.OutputDir, fmt.Sprintf("astro_aligned_%04d.tif", i))
				baseName := filepath.Base(imgPath)
				nameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
				dstFile := filepath.Join(req.OutputDir, fmt.Sprintf("aligned_%03d_%s.tif", i, nameWithoutExt))

				if _, err := os.Stat(srcFile); err == nil {
					os.Rename(srcFile, dstFile)
				}
			}
		}

		return AlignmentResult{ToolUsed: "align_image_stack", ProcessingTime: time.Since(start), Success: err == nil, Error: err, Warnings: []string{string(out)}}, err
	}
	warn := "astro alignment fallback; no tool succeeded"
	err := fmt.Errorf("%s", warn)
	return AlignmentResult{ToolUsed: "placeholder", ProcessingTime: time.Since(start), Success: false, Warnings: []string{warn}, Error: err}, err
}

func (p *AstroAlignmentProcessor) EstimateQuality(images []string) (float64, error) {
	if len(images) == 0 {
		return 0, fmt.Errorf("no images")
	}
	return 0.5, nil
}

func (p *AstroAlignmentProcessor) alignWithSiril(ctx context.Context, req AlignmentRequest, start time.Time) (AlignmentResult, error) {
	sirilBin := "siril"
	if commandExists("siril-cli") {
		sirilBin = "siril-cli"
	}
	script := fmt.Sprintf("cd %s\nrequires 0.99.10\nconvert %s -out=%s\nregister %s\n", req.OutputDir, filepath.Dir(req.Images[0]), req.OutputDir, strings.Join(req.Images, " "))
	cmd := exec.CommandContext(ctx, sirilBin, "-s", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	return AlignmentResult{ToolUsed: sirilBin, ProcessingTime: time.Since(start), Success: err == nil, Error: err, Warnings: []string{string(out)}}, err
}

func (p *AstroAlignmentProcessor) alignWithAstroAlign(ctx context.Context, req AlignmentRequest, start time.Time) (AlignmentResult, error) {
	args := append([]string{"-o", filepath.Join(req.OutputDir, "astroalign")}, req.Images...)
	cmd := exec.CommandContext(ctx, "astroalign", args...)
	out, err := cmd.CombinedOutput()
	return AlignmentResult{ToolUsed: "astroalign", ProcessingTime: time.Since(start), Success: err == nil, Error: err, Warnings: []string{string(out)}}, err
}
