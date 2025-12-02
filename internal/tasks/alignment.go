package tasks

import (
	"context"
	"time"
)

// AlignmentProcessor defines interface for image alignment tools.
type AlignmentProcessor interface {
	Name() string
	IsAvailable() bool
	SupportsType(alignType AlignmentType) bool
	Align(ctx context.Context, req AlignmentRequest) (AlignmentResult, error)
	EstimateQuality(images []string) (float64, error)
}

// AlignmentType enumerates supported alignment workflows.
type AlignmentType int

const (
	AlignmentAstro AlignmentType = iota
	AlignmentPanoramic
	AlignmentGeneral
	AlignmentTimelapse
)

// AlignmentRequest carries inputs for alignment.
type AlignmentRequest struct {
	Images       []string
	AlignType    AlignmentType
	ReferenceIdx int
	OutputDir    string
	Config       interface{}
	Quality      string
}

// AlignmentResult captures results and metrics.
type AlignmentResult struct {
	AlignedImages     []string
	TransformMatrices []TransformMatrix
	QualityMetrics    QualityMetrics
	ReferenceImage    string
	ProcessingTime    time.Duration
	ToolUsed          string
	Success           bool
	Warnings          []string
	Error             error
}

// TransformMatrix records per-image transforms.
type TransformMatrix struct {
	ImagePath   string
	Translation [2]float64
	Rotation    float64
	Scale       [2]float64
	Homography  [9]float64
}

// QualityMetrics tracks alignment quality stats.
type QualityMetrics struct {
	AlignmentAccuracy float64
	OverlapPercentage float64
	StarCount         int
	FeatureMatches    int
	RMSE              float64
	Sharpness         float64
}
