package tasks

import (
	"context"
	"testing"

	"photonic/internal/config"
)

func TestAlignmentManagerProcessorSelection(t *testing.T) {
	cfg := &config.AlignmentConfig{DefaultProcessor: ""}
	mgr := &AlignmentManager{
		processors: make(map[string]AlignmentProcessor),
		config:     cfg,
	}

	hybrid := newStubAlignmentProcessor("hybrid", AlignmentAstro, 0.9)
	native := newStubAlignmentProcessor("native", AlignmentAstro, 0.4)

	mgr.Register(hybrid)
	mgr.Register(native)

	outDir := t.TempDir()
	res, err := mgr.AlignWithType(context.Background(), []string{"a.cr2", "b.cr2"}, outDir, "high", AlignmentAstro)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.ToolUsed != hybrid.Name() {
		t.Fatalf("expected hybrid processor, got %s", res.ToolUsed)
	}
	if hybrid.alignCalls != 1 || native.alignCalls != 0 {
		t.Fatalf("unexpected call counts hybrid=%d native=%d", hybrid.alignCalls, native.alignCalls)
	}
}

func TestAlignmentManagerHonorsDefaultProcessor(t *testing.T) {
	cfg := &config.AlignmentConfig{DefaultProcessor: "native-default"}
	mgr := &AlignmentManager{
		processors: make(map[string]AlignmentProcessor),
		config:     cfg,
	}

	defaultProc := newStubAlignmentProcessor("native-default", AlignmentAstro, 0.1)
	better := newStubAlignmentProcessor("better", AlignmentAstro, 1.0)

	mgr.Register(defaultProc)
	mgr.Register(better)

	_, err := mgr.AlignWithType(context.Background(), []string{"a.cr2", "b.cr2"}, t.TempDir(), "normal", AlignmentAstro)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if defaultProc.alignCalls != 1 {
		t.Fatalf("expected default processor to be used")
	}
	if better.alignCalls != 0 {
		t.Fatalf("expected better processor to be skipped due to default override")
	}
}

func TestAlignmentManagerEstimateQuality(t *testing.T) {
	cfg := &config.AlignmentConfig{}
	mgr := &AlignmentManager{
		processors: make(map[string]AlignmentProcessor),
		config:     cfg,
	}

	low := newStubAlignmentProcessor("low", AlignmentPanoramic, 0.2)
	high := newStubAlignmentProcessor("high", AlignmentPanoramic, 0.8)
	mgr.Register(low)
	mgr.Register(high)

	_, err := mgr.AlignWithType(context.Background(), []string{"img1.jpg", "img2.jpg"}, t.TempDir(), "fast", AlignmentPanoramic)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if high.alignCalls != 1 {
		t.Fatalf("expected high quality processor to run, got calls high=%d low=%d", high.alignCalls, low.alignCalls)
	}
}

type stubAlignmentProcessor struct {
	name       string
	alignTypes []AlignmentType
	quality    float64
	alignCalls int
}

func newStubAlignmentProcessor(name string, t AlignmentType, quality float64) *stubAlignmentProcessor {
	return &stubAlignmentProcessor{name: name, alignTypes: []AlignmentType{t}, quality: quality}
}

func (p *stubAlignmentProcessor) Name() string { return p.name }

func (p *stubAlignmentProcessor) IsAvailable() bool { return true }

func (p *stubAlignmentProcessor) SupportsType(t AlignmentType) bool {
	for _, at := range p.alignTypes {
		if at == t {
			return true
		}
	}
	return false
}

func (p *stubAlignmentProcessor) Align(ctx context.Context, req AlignmentRequest) (AlignmentResult, error) {
	p.alignCalls++
	return AlignmentResult{Success: true, ToolUsed: p.name}, nil
}

func (p *stubAlignmentProcessor) EstimateQuality(images []string) (float64, error) {
	return p.quality, nil
}
