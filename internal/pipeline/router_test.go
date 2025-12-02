package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"photonic/internal/tasks"
)

func TestRouterAlignmentSelectionAndConfig(t *testing.T) {
	alignStub := &stubAlignmentManager{}
	r := &router{
		log:      slog.Default(),
		alignMgr: alignStub,
		stackFn:  tasks.StackImages,
		astroFac: func() astroStacker { return tasks.NewAstroStacker() },
	}

	imgs := []string{"a.jpg", "b.jpg"}
	job := Job{
		ID:     "align-1",
		Type:   JobAlign,
		Output: t.TempDir(),
		Options: map[string]any{
			"images":        imgs,
			"atype":         "astro",
			"quality":       "ultra",
			"starThreshold": 0.72,
		},
	}

	res := r.handleAlign(context.Background(), job)
	if res.Error != nil {
		t.Fatalf("expected nil error, got %v", res.Error)
	}
	if alignStub.lastType != tasks.AlignmentAstro {
		t.Fatalf("expected astro alignment, got %v", alignStub.lastType)
	}
	if alignStub.lastConfig["starThreshold"] != 0.72 {
		t.Fatalf("expected starThreshold passed to alignment manager")
	}
	if alignStub.callCount != 1 {
		t.Fatalf("expected one AlignWithTypeAndConfig call, got %d", alignStub.callCount)
	}
}

func TestRouterAlignDetectsTypeWhenMissing(t *testing.T) {
	alignStub := &stubAlignmentManager{detected: tasks.AlignmentPanoramic, hasDetect: true}
	r := &router{
		log:      slog.Default(),
		alignMgr: alignStub,
		stackFn:  tasks.StackImages,
		astroFac: func() astroStacker { return tasks.NewAstroStacker() },
	}

	job := Job{
		ID:     "align-2",
		Type:   JobAlign,
		Output: t.TempDir(),
		Options: map[string]any{
			"images": []string{"one.jpg", "two.jpg", "three.jpg"},
		},
	}

	res := r.handleAlign(context.Background(), job)
	if res.Error != nil {
		t.Fatalf("expected nil error, got %v", res.Error)
	}
	if alignStub.lastType != tasks.AlignmentPanoramic {
		t.Fatalf("expected detected panoramic type, got %v", alignStub.lastType)
	}
}

func TestRouterStackUsesAstroStacker(t *testing.T) {
	alignStub := &stubAlignmentManager{}
	stackStub := &stubStacker{}
	r := &router{
		log:      slog.Default(),
		alignMgr: alignStub,
		rawMgr:   nil,
		stackFn: func(ctx context.Context, req tasks.StackRequest) (tasks.StackResult, error) {
			return tasks.StackResult{}, errors.New("should not be called")
		},
		astroFac: func() astroStacker { return stackStub },
	}

	job := Job{
		ID:        "stack-astro",
		Type:      JobStack,
		InputPath: t.TempDir(),
		Output:    filepath.Join(t.TempDir(), "out.tif"),
		Options: map[string]any{
			"astroMode":     true,
			"method":        "sigma-clip",
			"sigmaLow":      1.1,
			"sigmaHigh":     2.2,
			"iterations":    4,
			"kappa":         1.8,
			"winsorPercent": 6.0,
		},
	}

	res := r.handleStack(context.Background(), job)
	if res.Error != nil {
		t.Fatalf("expected nil error, got %v", res.Error)
	}
	if stackStub.calls != 1 {
		t.Fatalf("expected astro stacker to be invoked")
	}
	if res.Meta["output"] != "astro-out.tif" {
		t.Fatalf("unexpected meta output %v", res.Meta["output"])
	}
}

func TestRouterStackUsesStandardStacker(t *testing.T) {
	alignStub := &stubAlignmentManager{}
	stackCalled := 0
	r := &router{
		log:      slog.Default(),
		alignMgr: alignStub,
		rawMgr:   nil,
		stackFn: func(ctx context.Context, req tasks.StackRequest) (tasks.StackResult, error) {
			stackCalled++
			return tasks.StackResult{OutputFile: "stack-out.tif", Method: req.Method}, nil
		},
		astroFac: func() astroStacker { return &stubStacker{} },
	}

	job := Job{
		ID:        "stack-basic",
		Type:      JobStack,
		InputPath: t.TempDir(),
		Output:    filepath.Join(t.TempDir(), "stack.tif"),
		Options: map[string]any{
			"method": "average",
		},
	}

	res := r.handleStack(context.Background(), job)
	if res.Error != nil {
		t.Fatalf("expected nil error, got %v", res.Error)
	}
	if stackCalled != 1 {
		t.Fatalf("expected stack function called once, got %d", stackCalled)
	}
	if res.Meta["output"] != "stack-out.tif" {
		t.Fatalf("unexpected output meta: %v", res.Meta["output"])
	}
}

// Stubs
type stubAlignmentManager struct {
	detected   tasks.AlignmentType
	hasDetect  bool
	lastType   tasks.AlignmentType
	lastConfig map[string]any
	callCount  int
}

func (s *stubAlignmentManager) DetectAlignmentType(images []string) tasks.AlignmentType {
	if s.hasDetect {
		return s.detected
	}
	return tasks.AlignmentGeneral
}

func (s *stubAlignmentManager) AlignWithTypeAndConfig(ctx context.Context, images []string, outputDir string, quality string, alignType tasks.AlignmentType, config map[string]any) (tasks.AlignmentResult, error) {
	s.callCount++
	s.lastType = alignType
	s.lastConfig = config
	return tasks.AlignmentResult{Success: true, ToolUsed: "stub-aligner"}, nil
}

type stubStacker struct {
	calls int
}

func (s *stubStacker) StackImages(ctx context.Context, req tasks.AstroStackRequest) (tasks.AstroStackResult, error) {
	s.calls++
	return tasks.AstroStackResult{
		OutputFile: "astro-out.tif",
		Method:     req.Method,
		ImageCount: 3,
	}, nil
}
