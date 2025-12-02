package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"photonic/internal/config"
)

// AstroDarktableProvenProcessor combines darktable RAW processing with proven alignment tools
type AstroDarktableProvenProcessor struct {
	darktableProcessor *DarktableProcessor
	config             config.DarktableConfig
}

func NewAstroDarktableProvenProcessor(cfg config.DarktableConfig) *AstroDarktableProvenProcessor {
	return &AstroDarktableProvenProcessor{
		darktableProcessor: &DarktableProcessor{config: cfg},
		config:             cfg,
	}
}

func (p *AstroDarktableProvenProcessor) Name() string { return "astro-darktable-proven" }

func (p *AstroDarktableProvenProcessor) SupportsType(alignType AlignmentType) bool {
	return alignType == AlignmentAstro
}

func (p *AstroDarktableProvenProcessor) IsAvailable() bool {
	return p.darktableProcessor.IsAvailable() && (commandExists("align_image_stack") || commandExists("siril") || commandExists("siril-cli"))
}

func (p *AstroDarktableProvenProcessor) EstimateQuality(images []string) (float64, error) {
	if len(images) < 2 {
		return 0.1, nil
	}
	return 0.95, nil // Highest quality - darktable + proven tools
}

func (p *AstroDarktableProvenProcessor) Align(ctx context.Context, req AlignmentRequest) (AlignmentResult, error) {
	start := time.Now()

	fmt.Printf("Starting astronomical alignment with darktable + proven tools...\n")
	fmt.Printf("Input images: %d\n", len(req.Images))
	fmt.Printf("Output directory: %s\n", req.OutputDir)

	// Create output directories
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return AlignmentResult{Success: false, Error: err}, err
	}

	processedDir := filepath.Join(req.OutputDir, "processed")
	if err := os.MkdirAll(processedDir, 0o755); err != nil {
		return AlignmentResult{Success: false, Error: err}, err
	}

	// Stage 1: Process RAW files with darktable
	fmt.Printf("Stage 1: Processing %d RAW files with darktable...\n", len(req.Images))
	processedImages := make([]string, 0, len(req.Images))

	for i, rawImage := range req.Images {
		baseName := filepath.Base(rawImage)
		nameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
		processedPath := filepath.Join(processedDir, fmt.Sprintf("%s.tif", nameWithoutExt))

		fmt.Printf("Processing RAW %d/%d: %s -> %s\n", i+1, len(req.Images), baseName, filepath.Base(processedPath))

		rawReq := RawConvertRequest{
			InputFile:  rawImage,
			OutputFile: processedPath,
			XMPFile:    rawImage + ".xmp",
			Config: map[string]interface{}{
				"format":  "tiff",
				"quality": 95,
			},
		}

		result, err := p.darktableProcessor.Convert(ctx, rawReq)
		if err != nil || !result.Success {
			fmt.Printf("Failed to process RAW image %s: %v\n", rawImage, err)
			continue
		}

		processedImages = append(processedImages, result.OutputFile)
	}

	if len(processedImages) == 0 {
		err := fmt.Errorf("no images were successfully processed by darktable")
		return AlignmentResult{Success: false, Error: err}, err
	}

	fmt.Printf("Stage 1 complete: %d/%d images processed successfully\n", len(processedImages), len(req.Images))

	// Stage 2: Align processed images with proven tools
	fmt.Printf("Stage 2: Aligning %d processed images with proven tools...\n", len(processedImages))

	var toolUsed string
	var alignedImages []string
	var err error

	// Try align_image_stack first (most reliable for astronomical images)
	if commandExists("align_image_stack") {
		alignedImages, toolUsed, err = p.alignWithAlignImageStack(ctx, processedImages, req.OutputDir)
	} else if commandExists("siril") || commandExists("siril-cli") {
		alignedImages, toolUsed, err = p.alignWithSiril(ctx, processedImages, req.OutputDir)
	} else {
		err = fmt.Errorf("no proven alignment tools available")
	}

	if err != nil {
		return AlignmentResult{Success: false, Error: err, ToolUsed: toolUsed}, err
	}

	duration := time.Since(start)
	fmt.Printf("Alignment complete: %d/%d images aligned successfully in %s\n", len(alignedImages), len(processedImages), duration)

	return AlignmentResult{
		AlignedImages:  alignedImages,
		ToolUsed:       fmt.Sprintf("darktable+%s", toolUsed),
		ProcessingTime: duration,
		Success:        true,
	}, nil
}

func (p *AstroDarktableProvenProcessor) alignWithAlignImageStack(ctx context.Context, images []string, outputDir string) ([]string, string, error) {
	tempPrefix := filepath.Join(outputDir, "temp_aligned_")

	args := []string{"-a", tempPrefix}
	args = append(args, images...)

	fmt.Printf("Running align_image_stack with %d images...\n", len(images))
	cmd := exec.CommandContext(ctx, "align_image_stack", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, "align_image_stack", fmt.Errorf("align_image_stack failed: %v\nOutput: %s", err, string(output))
	}

	// Rename the generated files to our expected format
	var alignedImages []string
	for i := range images {
		srcFile := fmt.Sprintf("%s%04d.tif", tempPrefix, i)

		if _, statErr := os.Stat(srcFile); statErr != nil {
			fmt.Printf("Warning: expected output file %s not found\n", srcFile)
			continue
		}

		// Extract original name for better naming
		origPath := images[i]
		baseName := filepath.Base(origPath)
		nameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
		dstFile := filepath.Join(outputDir, fmt.Sprintf("aligned_%03d_%s.tif", i, nameWithoutExt))

		if renameErr := os.Rename(srcFile, dstFile); renameErr != nil {
			fmt.Printf("Warning: failed to rename %s to %s: %v\n", srcFile, dstFile, renameErr)
			alignedImages = append(alignedImages, srcFile) // Use original name
		} else {
			alignedImages = append(alignedImages, dstFile)
		}
	}

	fmt.Printf("align_image_stack output:\n%s\n", string(output))
	return alignedImages, "align_image_stack", nil
}

func (p *AstroDarktableProvenProcessor) alignWithSiril(ctx context.Context, images []string, outputDir string) ([]string, string, error) {
	sirilBin := "siril"
	if commandExists("siril-cli") {
		sirilBin = "siril-cli"
	}

	// Create a proper Siril script for astronomical alignment
	script := fmt.Sprintf(`requires 1.0.0
cd %s
convert %s -out=%s
register %s
`,
		outputDir,
		filepath.Dir(images[0]),
		filepath.Join(outputDir, "siril_"),
		"siril_*.tif")

	fmt.Printf("Running %s with %d images...\n", sirilBin, len(images))
	fmt.Printf("Siril script:\n%s\n", script)

	cmd := exec.CommandContext(ctx, sirilBin, "-s", "-")
	cmd.Dir = outputDir
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, sirilBin, fmt.Errorf("siril failed: %v\nOutput: %s", err, string(output))
	}

	// Find the generated files (Siril usually creates r_*.tif files)
	var alignedImages []string
	for i := range images {
		possibleFiles := []string{
			filepath.Join(outputDir, fmt.Sprintf("r_siril_%04d.tif", i)),
			filepath.Join(outputDir, fmt.Sprintf("siril_%04d.tif", i)),
			filepath.Join(outputDir, fmt.Sprintf("aligned_%04d.tif", i)),
		}

		var foundFile string
		for _, candidate := range possibleFiles {
			if _, statErr := os.Stat(candidate); statErr == nil {
				foundFile = candidate
				break
			}
		}

		if foundFile != "" {
			// Rename to our expected format
			origPath := images[i]
			baseName := filepath.Base(origPath)
			nameWithoutExt := baseName[:len(baseName)-len(filepath.Ext(baseName))]
			dstFile := filepath.Join(outputDir, fmt.Sprintf("aligned_%03d_%s.tif", i, nameWithoutExt))

			if renameErr := os.Rename(foundFile, dstFile); renameErr != nil {
				alignedImages = append(alignedImages, foundFile)
			} else {
				alignedImages = append(alignedImages, dstFile)
			}
		}
	}

	fmt.Printf("Siril output:\n%s\n", string(output))
	return alignedImages, sirilBin, nil
}
