package tasks

import (
	"context"
	"fmt"
	"time"

	"photonic/internal/config"
)

// GeneralAlignmentProcessor handles generic alignment.
type GeneralAlignmentProcessor struct {
	config config.GeneralAlignmentConfig
}

func (p *GeneralAlignmentProcessor) Name() string { return "general" }
func (p *GeneralAlignmentProcessor) SupportsType(alignType AlignmentType) bool {
	return alignType == AlignmentGeneral || alignType == AlignmentTimelapse
}
func (p *GeneralAlignmentProcessor) IsAvailable() bool { return p.config.Enabled }

func (p *GeneralAlignmentProcessor) Align(ctx context.Context, req AlignmentRequest) (AlignmentResult, error) {
	_ = ctx
	start := time.Now()
	warn := fmt.Sprintf("general alignment placeholder method=%s", p.config.Method)
	return AlignmentResult{ToolUsed: "placeholder", ProcessingTime: time.Since(start), Success: true, Warnings: []string{warn}}, nil
}

func (p *GeneralAlignmentProcessor) EstimateQuality(images []string) (float64, error) {
	if len(images) == 0 {
		return 0, fmt.Errorf("no images")
	}
	return 0.4, nil
}
