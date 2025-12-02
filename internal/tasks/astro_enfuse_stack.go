package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"photonic/internal/fsutil"
)

// AstroEnfuseStacker implements astronomical stacking using proven enfuse tool
type AstroEnfuseStacker struct{}

func NewAstroEnfuseStacker() *AstroEnfuseStacker {
	return &AstroEnfuseStacker{}
}

// IsAvailable checks if enfuse is available
func (s *AstroEnfuseStacker) IsAvailable() bool {
	_, err := exec.LookPath("enfuse")
	return err == nil
}

// StackImages performs astronomical stacking using enfuse
func (s *AstroEnfuseStacker) StackImages(ctx context.Context, req AstroStackRequest) (AstroStackResult, error) {
	start := time.Now()

	if !s.IsAvailable() {
		return AstroStackResult{}, fmt.Errorf("enfuse not available - install Hugin tools")
	}

	fmt.Printf("Starting astronomical stacking with enfuse (proven tool)...\n")

	// Get input images
	images, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to list images: %v", err)
	}

	if len(images) < 2 {
		return AstroStackResult{}, fmt.Errorf("need at least 2 images for stacking, got %d", len(images))
	}

	fmt.Printf("Stacking %d images with enfuse...\n", len(images))

	// Prepare output path
	outputPath := req.Output
	if outputPath == "" || outputPath[len(outputPath)-1] == filepath.Separator {
		outputPath = filepath.Join(outputPath, "astro_stack.tif")
	} else if filepath.Ext(outputPath) == "" {
		// Add .tif extension if no extension provided
		outputPath = outputPath + ".tif"
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Build enfuse command based on stacking method
	args := s.buildEnfuseArgs(req, outputPath, images)

	fmt.Printf("Running enfuse with args: %s\n", args)
	cmd := exec.CommandContext(ctx, "enfuse", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return AstroStackResult{}, fmt.Errorf("enfuse failed: %v\nOutput: %s", err, string(output))
	}

	duration := time.Since(start)
	fmt.Printf("Enfuse astronomical stacking complete in %v\n", duration)
	fmt.Printf("Enfuse output:\n%s\n", string(output))

	// Check if output file was created
	if _, err := os.Stat(outputPath); err != nil {
		return AstroStackResult{}, fmt.Errorf("enfuse output file not created: %v", err)
	}

	return AstroStackResult{
		OutputFile:     outputPath,
		Method:         fmt.Sprintf("enfuse-%s", req.Method),
		ImageCount:     len(images),
		ProcessingTime: duration,
		RejectedPixels: 0, // enfuse doesn't report rejected pixels
		SignalToNoise:  0, // would need to calculate
		CosmicRayCount: 0, // enfuse doesn't detect cosmic rays specifically
	}, nil
}

// buildEnfuseArgs constructs enfuse command line arguments based on stacking method
func (s *AstroEnfuseStacker) buildEnfuseArgs(req AstroStackRequest, outputPath string, images []string) []string {
	args := []string{
		"--output=" + outputPath,
		"--depth=16",         // 16-bit output for astronomical data
		"--compression=none", // No compression for max quality
		"--verbose",          // Verbose output for debugging
	}

	// Configure enfuse based on stacking method
	switch req.Method {
	case "star-trails":
		// Special star trails method - combines aligned + unaligned for trail effect
		args = append(args,
			"--exposure-weight=1.0",   // Full exposure for base stars
			"--saturation-weight=0.2", // Slight saturation for trail color
			"--contrast-weight=0.0",   // No contrast bias (preserve both aligned & trails)
			"--entropy-weight=0.0",    // No entropy bias
			"--soft-mask",             // Smooth blending for natural trails
		)

	case "average", "mean":
		// Use star-trails-like approach but exposure-only for clean averaging
		args = append(args,
			"--exposure-weight=1.0",   // Full exposure weighting like star-trails
			"--saturation-weight=0.0", // No saturation bias
			"--contrast-weight=0.0",   // No contrast bias like star-trails
			"--entropy-weight=0.0",    // No entropy bias like star-trails
			"--soft-mask",             // Soft mask like star-trails (maybe this is key!)
			// No levels specified - use enfuse defaults
		)

	case "median":
		// Minimal blending for median-like behavior
		args = append(args,
			"--exposure-weight=0.5", // Reduced exposure weighting
			"--saturation-weight=0.0",
			"--contrast-weight=0.0", // No contrast bias to avoid star warping
			"--entropy-weight=0.0",
			"--hard-mask", // Sharp masking for crisp results
			"--levels=1",  // Minimal blend levels
		)

	case "sigma-clip":
		// Use contrast weighting to reject outliers (approximate sigma clipping)
		args = append(args,
			"--exposure-weight=0.8",
			"--saturation-weight=0.0",
			"--contrast-weight=0.3", // Higher contrast weight for outlier detection
			"--entropy-weight=0.1",  // Some entropy for cosmic ray rejection
			"--hard-mask",           // Hard masks for better outlier rejection
		)

	case "maximum":
		// Maximum value stacking (for star trails and bright object enhancement)
		args = append(args,
			"--exposure-weight=1.0",
			"--saturation-weight=0.5", // Favor bright pixels
			"--contrast-weight=0.5",   // Favor high contrast (stars)
			"--entropy-weight=0.2",
			"--hard-mask",
			"--levels=4",
		)

	case "focus", "detail-enhancement":
		// Special mode for enhancing faint details in astronomical objects
		args = append(args,
			"--exposure-weight=0.8",   // Slight bias toward well-exposed areas
			"--saturation-weight=0.3", // Preserve color information
			"--contrast-weight=0.5",   // High contrast weight for detail enhancement
			"--entropy-weight=0.2",    // Some entropy for structure preservation
			"--hard-mask",             // Sharp boundaries
			"--levels=7",              // Maximum blend levels for finest detail
		)

	default:
		// Default: pure astronomical stacking without multi-scale artifacts
		args = append(args,
			"--exposure-weight=1.0",
			"--saturation-weight=0.0", // No saturation bias
			"--contrast-weight=0.0",   // No contrast bias to prevent star warping
			"--entropy-weight=0.0",
			"--hard-mask", // Sharp masking
			"--levels=1",  // Single level to minimize warping
		)
	}

	// Add all input images
	args = append(args, images...)

	return args
}
