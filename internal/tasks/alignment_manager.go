package tasks

import (
	"context"
	"fmt"

	"photonic/internal/config"
)

// AlignmentManager selects and executes processors.
type AlignmentManager struct {
	processors map[string]AlignmentProcessor
	order      []string
	config     *config.AlignmentConfig
}

// NewAlignmentManager registers processors based on config.
func NewAlignmentManager(cfg *config.AlignmentConfig) *AlignmentManager {
	m := &AlignmentManager{processors: make(map[string]AlignmentProcessor), config: cfg}

	if cfg == nil {
		// Always register native processor as fallback
		m.Register(&NativeAstroProcessor{})
		return m
	}

	// Register proven astronomical alignment processors first (highest priority)
	if cfg.Darktable.Enabled {
		// Register the hybrid processor that does darktable + proven tools
		m.Register(NewAstroDarktableProvenProcessor(cfg.Darktable))
	}

	if cfg.Astro.Enabled {
		m.Register(&AstroAlignmentProcessor{config: cfg.Astro})
	}

	// Register other processors (temporarily disabled for testing)
	if cfg.Darktable.Enabled {
		// m.Register(NewStarAlignmentProcessor(cfg.Darktable)) // Temporarily disabled - star detection not working
		// m.Register(NewAstroDarktableNativeProcessor(cfg.Darktable)) // Temporarily disabled - uses fake alignment
	} else {
		// Fallback to pure native processor
		m.Register(&NativeAstroProcessor{})
	}
	if cfg.Panoramic.Enabled {
		m.Register(&PanoramicAlignmentProcessor{config: cfg.Panoramic})
	}
	if cfg.General.Enabled {
		m.Register(&GeneralAlignmentProcessor{config: cfg.General})
	}
	return m
}

// Register a processor.
func (m *AlignmentManager) Register(p AlignmentProcessor) {
	if p == nil {
		return
	}
	if _, exists := m.processors[p.Name()]; !exists {
		m.order = append(m.order, p.Name())
	}
	m.processors[p.Name()] = p
}

// Processors exposes registry.
func (m *AlignmentManager) Processors() map[string]AlignmentProcessor {
	return m.processors
}

// AutoAlign picks best processor based on type and availability.
func (m *AlignmentManager) AutoAlign(ctx context.Context, images []string, outputDir string, quality string) (AlignmentResult, error) {
	alignType := m.DetectAlignmentType(images)
	proc := m.selectProcessor(alignType, images)
	if proc == nil {
		return AlignmentResult{}, fmt.Errorf("no alignment processor available")
	}
	req := AlignmentRequest{Images: images, AlignType: alignType, OutputDir: outputDir, Quality: quality}
	return proc.Align(ctx, req)
}

// AlignWithType performs alignment with an explicit type instead of auto-detection
func (m *AlignmentManager) AlignWithType(ctx context.Context, images []string, outputDir string, quality string, alignType AlignmentType) (AlignmentResult, error) {
	proc := m.selectProcessor(alignType, images)
	if proc == nil {
		return AlignmentResult{}, fmt.Errorf("no alignment processor available for type %v", alignType)
	}
	req := AlignmentRequest{Images: images, AlignType: alignType, OutputDir: outputDir, Quality: quality}
	return proc.Align(ctx, req)
}

// AlignWithTypeAndConfig performs alignment with an explicit type and config
func (m *AlignmentManager) AlignWithTypeAndConfig(ctx context.Context, images []string, outputDir string, quality string, alignType AlignmentType, config map[string]any) (AlignmentResult, error) {
	proc := m.selectProcessor(alignType, images)
	if proc == nil {
		return AlignmentResult{}, fmt.Errorf("no alignment processor available for type %v", alignType)
	}
	req := AlignmentRequest{Images: images, AlignType: alignType, OutputDir: outputDir, Quality: quality, Config: config}
	return proc.Align(ctx, req)
}

// DetectAlignmentType exposes the detection logic
func (m *AlignmentManager) DetectAlignmentType(images []string) AlignmentType {
	return m.detectAlignmentType(images)
}

func (m *AlignmentManager) detectAlignmentType(images []string) AlignmentType {
	if len(images) == 0 {
		return AlignmentGeneral
	}
	// simple heuristic placeholder
	if len(images) > 20 {
		return AlignmentTimelapse
	}
	return AlignmentPanoramic
}

func (m *AlignmentManager) selectProcessor(t AlignmentType, images []string) AlignmentProcessor {
	// try default
	if m.config != nil && m.config.DefaultProcessor != "" {
		if p, ok := m.processors[m.config.DefaultProcessor]; ok && p.IsAvailable() && p.SupportsType(t) {
			return p
		}
	}

	var (
		best      AlignmentProcessor
		bestScore float64
	)

	for _, name := range m.order {
		p := m.processors[name]
		if !p.IsAvailable() || !p.SupportsType(t) {
			continue
		}

		score, err := p.EstimateQuality(images)
		if err != nil {
			continue
		}
		if best == nil || score > bestScore {
			best = p
			bestScore = score
		}
	}

	return best
}
