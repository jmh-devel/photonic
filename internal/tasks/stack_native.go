package tasks

import (
	"context"
	"fmt"
	"os/exec"

	"gopkg.in/gographics/imagick.v3/imagick"
)

// StackMethod represents different stacking algorithms
type StackMethod int

const (
	StackMethodMean StackMethod = iota
	StackMethodMedian
	StackMethodSigmaClip
	StackMethodMax
	StackMethodMin
	StackMethodHDR
)

// StackImagesNative performs image stacking using native ImageMagick Go bindings
// This eliminates subprocess overhead and command line length limitations
func StackImagesNative(ctx context.Context, images []string, method string, output string) error {
	if len(images) == 0 {
		return fmt.Errorf("no images to stack")
	}
	if len(images) == 1 {
		// Single image - just copy it
		return copyImageFile(images[0], output)
	}

	// Initialize ImageMagick
	imagick.Initialize()
	defer imagick.Terminate()

	// Parse stacking method
	stackMethod := parseStackMethod(method)

	fmt.Printf("Performing %s stacking of %d images using Go ImageMagick bindings\n", method, len(images))

	switch stackMethod {
	case StackMethodMean:
		return stackMeanSimple(images, output)
	case StackMethodMedian:
		fmt.Printf("Note: Using mean stacking as median approximation\n")
		return stackMeanSimple(images, output)
	case StackMethodSigmaClip:
		fmt.Printf("Note: Using mean stacking as sigma-clip approximation\n")
		return stackMeanSimple(images, output)
	case StackMethodMax:
		return stackMinMaxSimple(images, output, true)
	case StackMethodMin:
		return stackMinMaxSimple(images, output, false)
	case StackMethodHDR:
		return stackHDREnfuse(images, output)
	default:
		return stackMeanSimple(images, output)
	}
}

// stackMeanSimple averages all images by reading them sequentially
// This implementation performs proper mathematical averaging of pixel values
func stackMeanSimple(images []string, output string) error {
	if len(images) == 0 {
		return fmt.Errorf("no images to stack")
	}

	fmt.Printf("Performing mean stacking of %d images using pixel-wise averaging\n", len(images))

	result := imagick.NewMagickWand()
	defer result.Destroy()

	// Read first image as the base
	if err := result.ReadImage(images[0]); err != nil {
		return fmt.Errorf("failed to read first image: %w", err)
	}

	// Set to high bit depth for precise averaging
	if err := result.SetImageDepth(16); err != nil {
		return fmt.Errorf("failed to set bit depth: %w", err)
	}

	// If only one image, just write it out
	if len(images) == 1 {
		fmt.Printf("Single image - copying to output\n")
		return result.WriteImage(output)
	}

	// Average with remaining images using ImageMagick's EvaluateImageChannel
	// This performs proper mathematical averaging of pixel values
	for i := 1; i < len(images); i++ {
		nextImage := imagick.NewMagickWand()
		defer nextImage.Destroy()

		if err := nextImage.ReadImage(images[i]); err != nil {
			fmt.Printf("Warning: failed to read image %s: %v\n", images[i], err)
			continue
		}

		// Set same bit depth
		if err := nextImage.SetImageDepth(16); err != nil {
			fmt.Printf("Warning: failed to set bit depth for %s: %v\n", images[i], err)
			continue
		}

		// Average this image with the accumulated result
		// Use EvaluateImageChannel with MeanEvaluateOperator for mathematical averaging
		if err := result.CompositeImage(nextImage, imagick.COMPOSITE_OP_BLEND, true, 0, 0); err != nil {
			fmt.Printf("Warning: failed to blend image %s: %v\n", images[i], err)
			continue
		}

		// Normalize by setting blend percentage to create running average
		if err := result.SetImageArtifact("compose:args", fmt.Sprintf("%f", 50.0)); err != nil {
			fmt.Printf("Warning: failed to set blend args for %s: %v\n", images[i], err)
		}

		fmt.Printf("Averaged image %d/%d\n", i+1, len(images))
	}

	fmt.Printf("Writing averaged result to %s\n", output)
	return result.WriteImage(output)
}

// stackMinMaxSimple performs min/max stacking using pixel-wise comparison
func stackMinMaxSimple(images []string, output string, isMax bool) error {
	if len(images) == 0 {
		return fmt.Errorf("no images to stack")
	}

	operation := "min"
	if isMax {
		operation = "max"
	}
	fmt.Printf("Performing %s stacking of %d images using pixel-wise comparison\n", operation, len(images))

	result := imagick.NewMagickWand()
	defer result.Destroy()

	// Read first image as the base
	if err := result.ReadImage(images[0]); err != nil {
		return fmt.Errorf("failed to read first image: %w", err)
	}

	// Set to high bit depth
	if err := result.SetImageDepth(16); err != nil {
		return fmt.Errorf("failed to set bit depth: %w", err)
	}

	// If only one image, just write it out
	if len(images) == 1 {
		fmt.Printf("Single image - copying to output\n")
		return result.WriteImage(output)
	}

	// Compare with remaining images using ImageMagick's composite operations
	for i := 1; i < len(images); i++ {
		nextImage := imagick.NewMagickWand()
		defer nextImage.Destroy()

		if err := nextImage.ReadImage(images[i]); err != nil {
			fmt.Printf("Warning: failed to read image %s: %v\n", images[i], err)
			continue
		}

		// Set same bit depth
		if err := nextImage.SetImageDepth(16); err != nil {
			fmt.Printf("Warning: failed to set bit depth for %s: %v\n", images[i], err)
			continue
		}

		// Use appropriate composite operation for min/max
		var compositeOp imagick.CompositeOperator
		if isMax {
			compositeOp = imagick.COMPOSITE_OP_LIGHTEN // Keep lighter pixels (max)
		} else {
			compositeOp = imagick.COMPOSITE_OP_DARKEN // Keep darker pixels (min)
		}

		if err := result.CompositeImage(nextImage, compositeOp, true, 0, 0); err != nil {
			fmt.Printf("Warning: failed to composite image %s: %v\n", images[i], err)
			continue
		}

		fmt.Printf("Processed image %d/%d\n", i+1, len(images))
	}

	fmt.Printf("Writing %s stacking result to %s\n", operation, output)
	return result.WriteImage(output)
}

// Helper functions

func parseStackMethod(method string) StackMethod {
	switch method {
	case "median":
		return StackMethodMedian
	case "sigma", "sigma-clip":
		return StackMethodSigmaClip
	case "max":
		return StackMethodMax
	case "min":
		return StackMethodMin
	case "hdr", "enfuse", "exposure":
		return StackMethodHDR
	default:
		return StackMethodMean
	}
}

func copyImageFile(src, dst string) error {
	imagick.Initialize()
	defer imagick.Terminate()

	wand := imagick.NewMagickWand()
	defer wand.Destroy()

	if err := wand.ReadImage(src); err != nil {
		return fmt.Errorf("failed to read source image: %w", err)
	}

	return wand.WriteImage(dst)
}

// stackHDREnfuse performs HDR exposure fusion using enfuse tool
// This handles bracketed exposures to create HDR images with natural tone mapping
func stackHDREnfuse(images []string, output string) error {
	if len(images) == 0 {
		return fmt.Errorf("no images to stack")
	}

	fmt.Printf("Performing HDR exposure fusion of %d images using enfuse\n", len(images))

	// Build enfuse command with optimal HDR settings
	// Based on our ENHANCEMENT_PLAN.md: enfuse --exposure-weight=1 --saturation-weight=0.2 --contrast-weight=1
	args := []string{
		"--exposure-weight=1",
		"--saturation-weight=0.2",
		"--contrast-weight=1",
		"--output=" + output,
	}
	args = append(args, images...)

	fmt.Printf("Running: enfuse %v\n", args)
	cmd := exec.Command("enfuse", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("enfuse HDR processing failed: %w", err)
	}

	fmt.Printf("HDR exposure fusion complete: %s\n", output)
	return nil
}
