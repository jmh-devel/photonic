package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"photonic/internal/config"

	"gopkg.in/gographics/imagick.v3/imagick"
)

// ImageMagickProcessor wraps convert.
type ImageMagickProcessor struct {
	config config.ImageMagickConfig
	global *config.RawProcessing
}

func (p *ImageMagickProcessor) Name() string { return "imagemagick" }
func (p *ImageMagickProcessor) IsAvailable() bool {
	return p.config.Enabled && commandExists("convert")
}

func (p *ImageMagickProcessor) Convert(ctx context.Context, req RawConvertRequest) (RawConvertResult, error) {
	outputDir := filepath.Dir(req.OutputFile)

	// Check if something exists at the output directory path
	if stat, err := os.Stat(outputDir); err == nil && !stat.IsDir() {
		err := fmt.Errorf("cannot create output directory %s: file exists with same name (try using a different --output path)", outputDir)
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "imagick",
			Success:    false,
			Error:      err,
		}, err
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "imagick",
			Success:    false,
			Error:      fmt.Errorf("failed to create output directory: %v", err),
		}, err
	}

	// Verify input file exists
	if !fileExists(req.InputFile) {
		err := fmt.Errorf("input file does not exist: %s", req.InputFile)
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "imagick",
			Success:    false,
			Error:      err,
		}, err
	}

	// Initialize ImageMagick
	imagick.Initialize()
	defer imagick.Terminate()

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	// Use XMP-guided pano-safe processing
	fmt.Printf("DEBUG: Using XMP-guided pano-safe processing\n")

	// Find XMP file for this RAW file
	xmpPath := strings.TrimSuffix(req.InputFile, filepath.Ext(req.InputFile)) + filepath.Ext(req.InputFile) + ".xmp"
	fmt.Printf("DEBUG: Looking for XMP file: %s\n", xmpPath)

	// Determine processing strength (conservative for pano safety)
	strength := 0.6 // Conservative strength to maintain control points
	if p.config.Saturation > 1.0 || p.config.Vibrance > 1.0 {
		strength = 0.8 // Slightly stronger if user wants enhanced colors
	}
	fmt.Printf("DEBUG: Using processing strength: %f\n", strength)

	// Read the RAW file
	fmt.Printf("DEBUG: Reading RAW file with ImageMagick: %s\n", req.InputFile)
	err := mw.ReadImage(req.InputFile)
	if err != nil {
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "imagick",
			Success:    false,
			Error:      fmt.Errorf("failed to read RAW file: %v", err),
		}, err
	}

	// Parse XMP and apply pano-safe enhancements
	if _, err := os.Stat(xmpPath); err == nil {
		fmt.Printf("DEBUG: Found XMP file, parsing darktable history\n")
		entries, err := ParseDarktableModules(xmpPath)
		if err != nil {
			fmt.Printf("DEBUG: XMP parsing failed: %v, using minimal processing\n", err)
			err = p.applyMinimalProcessing(mw, strength)
		} else {
			fmt.Printf("DEBUG: Applying XMP-guided processing with %d operations\n", len(entries))
			err = ApplyPanoLookFromXMP(mw, entries, strength)
		}
	} else {
		fmt.Printf("DEBUG: No XMP file found, using minimal processing\n")
		err = p.applyMinimalProcessing(mw, strength)
	}

	if err != nil {
		fmt.Printf("DEBUG: Processing failed: %v\n", err)
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "imagick",
			Success:    false,
			Error:      fmt.Errorf("image processing failed: %v", err),
		}, err
	}

	// Strip ALL EXIF data including orientation
	fmt.Printf("DEBUG: Stripping all EXIF/profile data\n")
	err = mw.StripImage()
	if err != nil {
		fmt.Printf("DEBUG: Strip failed: %v\n", err)
	}

	// Apply other config options
	if p.config.Resize != "" {
		fmt.Printf("DEBUG: Resizing to %s\n", p.config.Resize)
		// Parse resize string and apply
		// For now, skip resize parsing
	}

	if p.config.Colorspace != "" {
		fmt.Printf("DEBUG: Setting colorspace to %s\n", p.config.Colorspace)
		// Apply colorspace
	}

	// Set JPEG quality
	if p.global != nil && p.global.Quality > 0 {
		fmt.Printf("DEBUG: Setting JPEG quality to %d\n", p.global.Quality)
		err = mw.SetImageCompressionQuality(uint(p.global.Quality))
		if err != nil {
			fmt.Printf("DEBUG: Failed to set quality: %v\n", err)
		}
	}

	// Write the output file
	fmt.Printf("DEBUG: Writing output file: %s\n", req.OutputFile)
	err = mw.WriteImage(req.OutputFile)
	if err != nil {
		return RawConvertResult{
			InputFile:  req.InputFile,
			OutputFile: req.OutputFile,
			ToolUsed:   "imagick",
			Success:    false,
			Error:      fmt.Errorf("failed to write output file: %v", err),
		}, err
	}

	fmt.Printf("DEBUG: ImageMagick conversion completed successfully\n")

	res := RawConvertResult{
		InputFile:     req.InputFile,
		OutputFile:    req.OutputFile,
		ToolUsed:      "imagick",
		ProcessingLog: "ImageMagick Go library conversion",
		Success:       true,
		Error:         nil,
	}

	if !fileExists(req.OutputFile) {
		res.Success = false
		res.Error = fmt.Errorf("ImageMagick completed but output file not found")
	}

	return res, nil
}

// applyMinimalProcessing applies basic enhancement when no XMP is available
func (p *ImageMagickProcessor) applyMinimalProcessing(mw *imagick.MagickWand, strength float64) error {
	// Basic setup
	if err := mw.SetImageDepth(16); err != nil {
		return fmt.Errorf("failed to set image depth: %w", err)
	}
	if err := mw.SetImageColorspace(imagick.COLORSPACE_SRGB); err != nil {
		return fmt.Errorf("failed to set colorspace: %w", err)
	}
	if err := mw.AutoOrientImage(); err != nil {
		return fmt.Errorf("failed to auto-orient: %w", err)
	}

	// Very minimal enhancement to avoid control point issues
	if strength > 0 {
		// Just a tiny bit of contrast and saturation
		if err := mw.SigmoidalContrastImage(true, 1.2*strength, 0.5); err != nil {
			return fmt.Errorf("contrast failed: %w", err)
		}

		// Apply config settings if present
		if p.config.Saturation > 1.0 {
			sat := 100.0 + (p.config.Saturation-1.0)*100*0.5*strength // Scale by strength
			if err := mw.ModulateImage(100, sat, 100); err != nil {
				return fmt.Errorf("saturation failed: %w", err)
			}
		}

		if p.config.Vibrance > 1.0 {
			sat := 100.0 + (p.config.Vibrance-1.0)*50*0.5*strength // Scale by strength, less aggressive
			if err := mw.ModulateImage(100, sat, 100); err != nil {
				return fmt.Errorf("vibrance failed: %w", err)
			}
		}
	}

	return nil
}

func (p *ImageMagickProcessor) BatchConvert(ctx context.Context, files []string, outputDir string) ([]string, error) {
	var outs []string
	ext := p.global.OutputFormat
	if ext == "" {
		ext = "jpg"
	}
	for _, f := range files {
		out := filepath.Join(outputDir, trimExt(filepath.Base(f))+"."+strings.TrimPrefix(ext, "."))
		_, _ = p.Convert(ctx, RawConvertRequest{InputFile: f, OutputFile: out})
		outs = append(outs, out)
	}
	return outs, nil
}

// TODO: Implement intelligent tone analysis and automatic tone equalization
// These functions will analyze image characteristics to determine optimal tone mapping

// analyzeShadowZones analyzes the shadow regions of an image to determine
// if tone equalization should be applied and with what parameters
func (p *ImageMagickProcessor) analyzeShadowZones(mw *imagick.MagickWand) (shouldApplyToneEQ bool, shadowLevel float64, err error) {
	// TODO: Implement shadow zone histogram analysis
	// - Calculate histogram for lower 30% of luminance values
	// - Detect if shadows are significantly underexposed
	// - Determine safe tone equalization parameters that won't blow out shadows
	// - Return recommendation for tone EQ application

	return false, 0.0, fmt.Errorf("shadow zone analysis not yet implemented")
}

// calculateOptimalToneCurve analyzes an image's tonal distribution and calculates
// optimal tone curve parameters similar to darktable's toneequal + filmicrgb
func (p *ImageMagickProcessor) calculateOptimalToneCurve(mw *imagick.MagickWand) (shadowLift, midtoneGamma, highlightCompression float64, err error) {
	// TODO: Implement intelligent tone curve calculation
	// - Analyze image histogram across all channels
	// - Detect exposure characteristics (underexposed, overexposed, balanced)
	// - Calculate safe shadow lift without clipping
	// - Determine midtone gamma adjustment for optimal contrast
	// - Set highlight compression to preserve detail
	// - Return parameters for LevelImage and SigmoidalContrastImage

	return 0.02, 1.2, 0.98, fmt.Errorf("optimal tone curve calculation not yet implemented")
}

// autoToneEqualizer applies intelligent tone equalization based on image analysis
// This will replace the current naive approach that can blow out shadows
func (p *ImageMagickProcessor) autoToneEqualizer(mw *imagick.MagickWand) error {
	// TODO: Implement smart auto tone equalization
	// 1. Call analyzeShadowZones to determine if tone EQ is safe
	// 2. Call calculateOptimalToneCurve to get optimal parameters
	// 3. Apply tone adjustments only if they improve the image
	// 4. Use selective masking to protect already-good tonal areas
	// 5. Monitor for highlight/shadow clipping and back off if detected

	fmt.Printf("DEBUG: Auto tone equalizer not yet implemented - using safe defaults\n")

	// For now, apply very conservative tone adjustment
	shadowLift, midtoneGamma, highlightCompression, err := p.calculateOptimalToneCurve(mw)
	if err != nil {
		// Use ultra-safe defaults
		return mw.LevelImage(0.01, 1.1, 0.99)
	}

	return mw.LevelImage(shadowLift, midtoneGamma, highlightCompression)
}
