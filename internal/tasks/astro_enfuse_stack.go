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

	// Special handling for star-trails method
	if req.Method == "star-trails" {
		return s.stackStarTrailsHybrid(ctx, req, start)
	}

	// Get input images for other methods
	images, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to list images: %v", err)
	}

	if len(images) < 2 {
		return AstroStackResult{}, fmt.Errorf("need at least 2 images for stacking, got %d", len(images))
	}

	fmt.Printf("Stacking %d images with enfuse...\n", len(images))

	// Continue with normal processing...
	return s.processWithEnfuse(ctx, req, images, start)
}

// stackStarTrailsHybrid implements the hybrid star-trails method from STAR_TRAILS_DISCOVERY.md
// Combines aligned images (sharp stars) + unaligned images (motion trails)
func (s *AstroEnfuseStacker) stackStarTrailsHybrid(ctx context.Context, req AstroStackRequest, start time.Time) (AstroStackResult, error) {
	fmt.Printf("Using hybrid star-trails method: aligned + unaligned images\n")

	// Get unaligned images (motion trails)
	unalignedImages, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to list unaligned images: %v", err)
	}

	// Look for aligned images - try pipeline-created directory first, then fallback to output/aligned-only
	possibleAlignedDirs := []string{
		filepath.Join(filepath.Dir(req.Output), "aligned"), // Pipeline-created directory
		"output/aligned-only",                              // Manual aligned directory
	}

	var alignedImages []string
	var alignedDir string
	for _, dir := range possibleAlignedDirs {
		if images, err := fsutil.ListImages(dir); err == nil && len(images) > 0 {
			alignedImages = images
			alignedDir = dir
			break
		}
	}

	if len(alignedImages) == 0 {
		fmt.Printf("No aligned images found, using unaligned images only for motion trails\n")
		// Fallback to unaligned images only
		return s.processWithEnfuse(ctx, req, unalignedImages, start)
	}

	// Combine aligned + unaligned for the hybrid effect
	allImages := make([]string, 0, len(alignedImages)+len(unalignedImages))
	allImages = append(allImages, alignedImages...)   // Sharp stars first
	allImages = append(allImages, unalignedImages...) // Motion trails second

	fmt.Printf("Star-trails hybrid: %d aligned (from %s) + %d unaligned = %d total images\n",
		len(alignedImages), alignedDir, len(unalignedImages), len(allImages))

	return s.processWithEnfuse(ctx, req, allImages, start)
}

// processWithEnfuse handles the actual enfuse processing
func (s *AstroEnfuseStacker) processWithEnfuse(ctx context.Context, req AstroStackRequest, images []string, start time.Time) (AstroStackResult, error) {
	if len(images) < 2 {
		return AstroStackResult{}, fmt.Errorf("need at least 2 images for stacking, got %d", len(images))
	}

	fmt.Printf("Processing %d images with enfuse...\n", len(images))

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
		// For averaging, use equal exposure weighting and let enfuse handle the rest
		// DO NOT set all weights to 0 - that breaks enfuse blending
		args = append(args,
			"--exposure-weight=1.0",   // Equal exposure contribution
			"--saturation-weight=0.0", // No saturation bias for clean averaging
			"--contrast-weight=0.0",   // No contrast bias for averaging
			"--entropy-weight=0.0",    // No entropy bias for averaging
			// Use soft-mask for smoother averaging blending
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
		// Default: Test 5 parameters (best results from testing) - equal weights for clean stacking
		args = append(args,
			"--exposure-weight=1.0",   // Equal exposure contribution
			"--saturation-weight=1.0", // Equal saturation contribution
			"--contrast-weight=1.0",   // Equal contrast contribution
			"--entropy-weight=1.0",    // Equal entropy contribution
			"--soft-mask",             // Soft masking for smooth blending (no artifacts)
			"--levels=5",              // Multiple levels for quality without excessive processing
		)
	} // Add all input images
	args = append(args, images...)

	return args
}
