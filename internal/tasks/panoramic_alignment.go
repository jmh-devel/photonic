package tasks

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"photonic/internal/config"
)

// PanoramicAlignmentProcessor aligns panoramic sets.
type PanoramicAlignmentProcessor struct {
	config config.PanoramicAlignmentConfig
}

func (p *PanoramicAlignmentProcessor) Name() string { return "panoramic" }
func (p *PanoramicAlignmentProcessor) SupportsType(alignType AlignmentType) bool {
	return alignType == AlignmentPanoramic
}
func (p *PanoramicAlignmentProcessor) IsAvailable() bool {
	if !p.config.Enabled {
		return false
	}
	return commandExists("align_image_stack")
}

func (p *PanoramicAlignmentProcessor) Align(ctx context.Context, req AlignmentRequest) (AlignmentResult, error) {
	start := time.Now()
	if !p.IsAvailable() {
		return AlignmentResult{}, fmt.Errorf("no panoramic alignment tool available")
	}
	if p.config.UseHuginAlignment && commandExists("align_image_stack") {
		return p.alignWithHugin(ctx, req, start)
	}
	return p.alignWithOpenCV(ctx, req, start)
}

func (p *PanoramicAlignmentProcessor) alignWithHugin(ctx context.Context, req AlignmentRequest, start time.Time) (AlignmentResult, error) {
	args := []string{"-a", filepath.Join(req.OutputDir, "pano_aligned_")}
	switch req.Quality {
	case "ultra":
		args = append(args, "-g", "10", "-s", "2")
	case "high":
		args = append(args, "-g", "8", "-s", "1")
	case "fast":
		args = append(args, "-g", "4")
	}
	args = append(args, req.Images...)
	cmd := exec.CommandContext(ctx, "align_image_stack", args...)
	out, err := cmd.CombinedOutput()
	return AlignmentResult{ToolUsed: "align_image_stack", ProcessingTime: time.Since(start), Success: err == nil, Error: err, Warnings: []string{string(out)}}, err
}

func (p *PanoramicAlignmentProcessor) alignWithOpenCV(ctx context.Context, req AlignmentRequest, start time.Time) (AlignmentResult, error) {
	// placeholder for OpenCV-based alignment
	warn := "opencv panoramic alignment not yet implemented"
	return AlignmentResult{ToolUsed: "opencv-placeholder", ProcessingTime: time.Since(start), Success: true, Warnings: []string{warn}}, nil
}

func (p *PanoramicAlignmentProcessor) EstimateQuality(images []string) (float64, error) {
	if len(images) == 0 {
		return 0, fmt.Errorf("no images")
	}
	return 0.6, nil
}
