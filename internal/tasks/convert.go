package tasks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"photonic/internal/fsutil"
)

// ConvertRequest defines inputs for RAW conversion.
type ConvertRequest struct {
	InputPath string
	OutputDir string
	Format    string // "jpg", "png", "tiff"
	Quality   int    // JPEG quality 1-100
	Resize    string // e.g., "1920x1080" or "" for original size
}

// ConvertResult captures conversion metadata.
type ConvertResult struct {
	InputFile     string
	OutputFile    string
	OriginalSize  int64
	ConvertedSize int64
	Format        string
}

// ConvertRAWToFormat converts a single RAW file to specified format using ImageMagick.
func ConvertRAWToFormat(ctx context.Context, req ConvertRequest) (ConvertResult, error) {
	if !fsutil.IsRAWFile(req.InputPath) {
		return ConvertResult{}, fmt.Errorf("not a RAW file: %s", req.InputPath)
	}

	// Get original file size
	stat, err := os.Stat(req.InputPath)
	if err != nil {
		return ConvertResult{}, err
	}
	originalSize := stat.Size()

	// Generate output filename
	baseName := strings.TrimSuffix(filepath.Base(req.InputPath), filepath.Ext(req.InputPath))
	outputFile := filepath.Join(req.OutputDir, baseName+"."+req.Format)

	// Ensure output directory exists
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return ConvertResult{}, err
	}

	// Build ImageMagick command with proper EXIF orientation handling
	args := []string{req.InputPath}

	// Auto-orient based on EXIF orientation data - this fixes panoramic orientation issues
	args = append(args, "-auto-orient")

	// Add resize if specified
	if req.Resize != "" {
		args = append(args, "-resize", req.Resize)
	}

	// Add quality for JPEG
	if req.Format == "jpg" && req.Quality > 0 {
		args = append(args, "-quality", fmt.Sprint(req.Quality))
	}

	args = append(args, outputFile)

	// Execute conversion
	cmd := exec.CommandContext(ctx, "convert", args...)
	fmt.Printf("DEBUG: Executing convert command: convert %s\n", strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		return ConvertResult{}, fmt.Errorf("ImageMagick conversion failed: %v", err)
	}

	// NUCLEAR OPTION: Strip all orientation EXIF data instead of preserving it
	if commandExists("exiftool") {
		// Remove orientation tags that cause panoramic stitching issues
		exifCmd := exec.CommandContext(ctx, "exiftool",
			"-overwrite_original",
			"-Orientation=",
			"-CameraOrientation=",
			outputFile)
		_ = exifCmd.Run()
		fmt.Printf("DEBUG: Stripped EXIF orientation data\n")
	} // Get converted file size
	convertedStat, err := os.Stat(outputFile)
	if err != nil {
		return ConvertResult{}, err
	}

	return ConvertResult{
		InputFile:     req.InputPath,
		OutputFile:    outputFile,
		OriginalSize:  originalSize,
		ConvertedSize: convertedStat.Size(),
		Format:        req.Format,
	}, nil
}

// ConvertRAWBatch converts multiple RAW files to temporary JPEGs for processing.
func ConvertRAWBatch(ctx context.Context, inputFiles []string, tempDir string) ([]string, error) {
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, err
	}

	var convertedFiles []string
	for _, inputFile := range inputFiles {
		if !fsutil.IsRAWFile(inputFile) {
			// If it's already a supported format, just add it
			if fsutil.IsImageFile(inputFile) {
				convertedFiles = append(convertedFiles, inputFile)
			}
			continue
		}

		req := ConvertRequest{
			InputPath: inputFile,
			OutputDir: tempDir,
			Format:    "jpg",
			Quality:   90, // High quality for processing
		}

		result, err := ConvertRAWToFormat(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %s: %v", inputFile, err)
		}

		convertedFiles = append(convertedFiles, result.OutputFile)
	}

	return convertedFiles, nil
}

// CleanupTempFiles removes temporary converted files.
func CleanupTempFiles(files []string) error {
	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
