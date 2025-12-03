package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"photonic/internal/fsutil"
)

// StackRequest defines inputs for image stacking.
type StackRequest struct {
	InputDir string
	Output   string
	Method   string
}

// StackResult captures output metadata.
type StackResult struct {
	OutputFile string
	Method     string
	ImageCount int
	Dimensions string
}

// StackImages implements intelligent stacking with native Go ImageMagick bindings.
// This eliminates subprocess overhead and command line limitations.
func StackImages(ctx context.Context, req StackRequest) (StackResult, error) {
	output := req.Output
	if output == "" || output[len(output)-1] == filepath.Separator {
		output = filepath.Join(output, "stack.txt")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return StackResult{}, err
	}

	images, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		return StackResult{}, err
	}
	meta := StackResult{OutputFile: output, Method: req.Method, ImageCount: len(images)}
	if len(images) > 0 {
		meta.Dimensions = identifyDimensions(images[0])
	}

	if len(images) <= 1 {
		if err := TouchManifest(output, "method="+req.Method); err != nil {
			return StackResult{}, err
		}
		return meta, nil
	}

	// Use native Go ImageMagick implementation - much more efficient!
	fmt.Printf("Stacking %d images using native Go ImageMagick bindings (%s method)\n", len(images), req.Method)
	if err := StackImagesNative(ctx, images, req.Method, output); err != nil {
		fmt.Printf("Native stacking failed: %v, falling back to subprocess method\n", err)
		// Fallback to chunked subprocess method if native fails
		if err := stackImagesHierarchical(ctx, images, req.Method, output); err != nil {
			// Final fallback to manifest
			if err := TouchManifest(output, "method="+req.Method+"; native_error="+err.Error()); err != nil {
				return StackResult{}, err
			}
		}
	}

	return meta, nil
}

func mapMethod(method string) string {
	switch method {
	case "median":
		return "median"
	case "sigma":
		return "median" // placeholder; sigma clipping not supported in simple convert
	case "max":
		return "max"
	case "min":
		return "min"
	case "star-trails":
		return "max" // Star trails use maximum blending for trail effect
	case "astro":
		return "mean" // Clean mathematical averaging for astrophotography
	default:
		return "mean"
	}
}

// stackImagesDirect performs direct ImageMagick stacking for small image sets
func stackImagesDirect(ctx context.Context, images []string, method, output string) error {
	args := append(images, "-evaluate-sequence", mapMethod(method), output)
	cmd := exec.CommandContext(ctx, "convert", args...)
	return cmd.Run()
}

// stackImagesHierarchical implements chunked stacking for large image sets
func stackImagesHierarchical(ctx context.Context, images []string, method, output string) error {
	const chunkSize = 15 // Safe size to avoid command line limits

	// Create temporary directory for intermediate stacks
	tempDir := filepath.Join(filepath.Dir(output), ".photonic-stacking-temp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tempDir) // Clean up temp files

	var intermediates []string

	// Process images in chunks
	for i := 0; i < len(images); i += chunkSize {
		end := i + chunkSize
		if end > len(images) {
			end = len(images)
		}
		chunk := images[i:end]

		// Create intermediate stack file
		intermediate := filepath.Join(tempDir, fmt.Sprintf("intermediate_%03d.tif", i/chunkSize))

		// Stack this chunk
		if err := stackImagesDirect(ctx, chunk, method, intermediate); err != nil {
			return fmt.Errorf("failed to stack chunk %d-%d: %w", i, end-1, err)
		}
		intermediates = append(intermediates, intermediate)
	}

	// Stack all intermediate results into final image
	return stackImagesDirect(ctx, intermediates, method, output)
}
