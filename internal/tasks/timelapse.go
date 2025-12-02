package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"photonic/internal/fsutil"
	"strings"
	"time"
)

// backupExistingFile creates a backup of an existing file with timestamp
func backupExistingFile(filepath string) error {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath + ".backup." + timestamp

	return os.Rename(filepath, backupPath)
}

// TimelapseRequest describes a timelapse job.
type TimelapseRequest struct {
	InputDir   string
	Output     string
	OutputDir  string // Optional: specify directory for multiple formats
	FPS        int
	Stabilize  bool
	Formats    []string // Supported: "mp4", "3gp", "3gp-h264", "gif"
	Resolution string   // Optional: "1080p", "720p", "480p", "240p" for mobile
}

// TimelapseResult captures output metadata.
type TimelapseResult struct {
	OutputFiles []OutputFile
	FrameCount  int
	UsedFFmpeg  bool
}

// OutputFile represents a single output file with its format details
type OutputFile struct {
	Path   string
	Format string
	Codec  string
	Size   int64 // file size in bytes
}

// BuildTimelapse attempts to run ffmpeg over the directory with multiple format support.
func BuildTimelapse(ctx context.Context, req TimelapseRequest) (TimelapseResult, error) {
	logger := slog.Default()

	// Set defaults
	if len(req.Formats) == 0 {
		req.Formats = []string{"mp4"} // Default to MP4
	}
	if req.FPS == 0 {
		req.FPS = 10 // Default to 10fps for astronomy timelapses
	}

	// Log the start of timelapse processing
	logger.Info("starting timelapse processing",
		"input_dir", req.InputDir,
		"output", req.Output,
		"output_dir", req.OutputDir,
		"fps", req.FPS,
		"stabilize", req.Stabilize,
		"formats", req.Formats,
		"resolution", req.Resolution,
	)

	// Get all images from directory
	allImages, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		logger.Error("failed to list images", "error", err, "input_dir", req.InputDir)
		return TimelapseResult{}, err
	}

	if len(allImages) == 0 {
		logger.Error("no images found", "input_dir", req.InputDir)
		return TimelapseResult{}, fmt.Errorf("no images found in %s", req.InputDir)
	}

	logger.Info("found images for timelapse",
		"total_images", len(allImages),
		"input_dir", req.InputDir,
	)

	// Separate RAW and processed files
	rawFiles, processedFiles := fsutil.SeparateRAWAndProcessed(allImages)

	var processFiles []string
	var tempDir string
	var needsCleanup bool

	// If we have RAW files, convert them to temporary JPEGs
	if len(rawFiles) > 0 {
		logger.Info("processing RAW files",
			"raw_count", len(rawFiles),
			"processed_count", len(processedFiles),
		)

		tempDir = filepath.Join(os.TempDir(), "photonic_timelapse_"+fmt.Sprint(os.Getpid()))
		convertedFiles, err := ConvertRAWBatch(ctx, rawFiles, tempDir)
		if err != nil {
			return TimelapseResult{}, fmt.Errorf("RAW conversion failed: %v", err)
		}
		processFiles = append(processFiles, convertedFiles...)
		needsCleanup = true
	}

	// Add already processed files
	processFiles = append(processFiles, processedFiles...)

	if len(processFiles) == 0 {
		return TimelapseResult{}, fmt.Errorf("no processable images found")
	}

	// Clean up temp files when done
	defer func() {
		if needsCleanup {
			os.RemoveAll(tempDir)
		}
	}()

	// Use the first processed file's directory for pattern matching
	var pattern string
	if needsCleanup && len(processFiles) > 0 {
		// For converted files, use temp directory pattern
		pattern = filepath.Join(tempDir, "*.jpg")
	} else {
		// For existing files, use original pattern approach
		pattern = filepath.Join(req.InputDir, "*.jpg")
	}

	// Generate output files for each format
	var outputFiles []OutputFile
	var baseOutput string

	if req.OutputDir != "" {
		// Use output directory
		if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
			logger.Error("failed to create output directory", "error", err, "output_dir", req.OutputDir)
			return TimelapseResult{}, err
		}
		baseOutput = filepath.Join(req.OutputDir, "timelapse")
	} else {
		// Use single output file
		baseOutput = strings.TrimSuffix(req.Output, filepath.Ext(req.Output))
		outputDir := filepath.Dir(req.Output)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			logger.Error("failed to create output directory", "error", err, "output_dir", outputDir)
			return TimelapseResult{}, err
		}
	}

	// Process each requested format
	for _, format := range req.Formats {
		outputFile, err := generateTimelapseFormat(ctx, pattern, baseOutput, format, req.FPS, req.Resolution)
		if err != nil {
			logger.Error("failed to generate format", "format", format, "error", err)
			continue
		}
		outputFiles = append(outputFiles, outputFile)

		logger.Info("generated timelapse format successfully",
			"format", format,
			"output_file", outputFile.Path,
			"codec", outputFile.Codec,
		)
	}

	if len(outputFiles) == 0 {
		logger.Error("failed to generate any output formats")
		return TimelapseResult{}, fmt.Errorf("failed to generate any output formats")
	}

	return TimelapseResult{
		OutputFiles: outputFiles,
		UsedFFmpeg:  true,
		FrameCount:  len(processFiles),
	}, nil
}

// generateTimelapseFormat creates a single timelapse in the specified format
func generateTimelapseFormat(ctx context.Context, pattern, baseOutput, format string, fps int, resolution string) (OutputFile, error) {
	logger := slog.Default()

	var outputPath string
	var args []string

	// Base args for all formats
	baseArgs := []string{"-y", "-pattern_type", "glob", "-i", pattern, "-r", fmt.Sprint(fps)}

	switch format {
	case "mp4":
		outputPath = baseOutput + ".mp4"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		args = append(baseArgs,
			"-c:v", "libx264",
			"-profile:v", "high444", // High 4:4:4 Predictive profile like working file
			"-level", "4.0",
			"-pix_fmt", "yuvj444p", // Full range 4:4:4 like working file
			"-b:v", "7600k", // Match bitrate of working file
			"-maxrate", "8000k",
			"-bufsize", "8000k",
		)
		if resolution != "" {
			args = append(args, "-vf", getVideoFilter(resolution))
		}
		args = append(args, outputPath)

	case "mp4-h265":
		outputPath = baseOutput + "-h265.mp4"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		args = append(baseArgs,
			"-c:v", "libx265",
			"-preset", "medium",
			"-crf", "28",
			"-pix_fmt", "yuv420p",
		)
		if resolution != "" {
			args = append(args, "-vf", getVideoFilter(resolution))
		}
		args = append(args, outputPath)

	case "3gp":
		// 3GP with MPEG4 Simple Profile for maximum compatibility with old phones
		// Based on working aug11-12-aurora-stars-libxvid.3gp parameters
		outputPath = baseOutput + "-compatible.3gp"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		args = append(baseArgs,
			"-c:v", "mpeg4",
			"-profile:v", "0", // Simple Profile
			"-level", "3",
			"-vtag", "mp4v",
			"-b:v", "960k", // Match bitrate of working file
			"-maxrate", "1200k",
			"-bufsize", "1200k",
			"-pix_fmt", "yuv420p",
			"-f", "3gp", // Ensure 3GP format
			"-brand", "3gp4", // Match major brand of working file
		)
		// Keep original resolution initially - user can specify smaller if needed
		if resolution != "" {
			args = append(args, "-vf", getVideoFilter(resolution))
		}
		args = append(args, outputPath)

	case "3gp-h264":
		// 3GP with H.264 for newer phones that support it
		outputPath = baseOutput + "-h264.3gp"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		args = append(baseArgs,
			"-c:v", "libx264",
			"-profile:v", "baseline",
			"-level", "3.0",
			"-b:v", "800k",
			"-maxrate", "1200k",
			"-bufsize", "1200k",
			"-vf", "scale=480:320", // Slightly higher resolution
			outputPath,
		)

	case "3gp-mobile":
		// 3GP with MPEG4 Simple Profile optimized for old flip phones (smaller resolution)
		outputPath = baseOutput + "-mobile.3gp"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		args = append(baseArgs,
			"-c:v", "mpeg4",
			"-profile:v", "0", // Simple Profile
			"-level", "1", // Lower level for old devices
			"-vtag", "mp4v",
			"-b:v", "300k", // Lower bitrate for small screens
			"-maxrate", "400k",
			"-bufsize", "400k",
			"-pix_fmt", "yuv420p",
			"-vf", "scale=320:240", // Small resolution for flip phones
			"-r", "12", // Lower framerate for old devices
			"-f", "3gp",
			"-brand", "3gp4",
		)
		args = append(args, outputPath)

	case "gif":
		outputPath = baseOutput + ".gif"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		// Simpler GIF generation that handles varying image sizes
		args = append(baseArgs,
			"-vf", fmt.Sprintf("fps=%d,scale=480:480:force_original_aspect_ratio=decrease:flags=lanczos,pad=480:480:(ow-iw)/2:(oh-ih)/2", fps),
			"-y",
			outputPath,
		)

	default:
		return OutputFile{}, fmt.Errorf("unsupported format: %s", format)
	}

	logger.Info("executing ffmpeg command",
		"format", format,
		"args", args,
		"pattern", pattern,
		"output_file", outputPath,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("ffmpeg failed for format",
			"format", format,
			"error", err,
			"ffmpeg_output", string(output),
		)
		return OutputFile{}, fmt.Errorf("ffmpeg failed for %s: %v", format, err)
	}

	// Get file size
	stat, err := os.Stat(outputPath)
	if err != nil {
		logger.Error("failed to stat output file", "file", outputPath, "error", err)
		return OutputFile{}, err
	}

	codec := getCodecForFormat(format)

	return OutputFile{
		Path:   outputPath,
		Format: format,
		Codec:  codec,
		Size:   stat.Size(),
	}, nil
}

// getVideoFilter returns the appropriate video filter for resolution scaling
func getVideoFilter(resolution string) string {
	switch resolution {
	case "1080p":
		return "scale=1920:1080"
	case "720p":
		return "scale=1280:720"
	case "480p":
		return "scale=854:480"
	case "240p":
		return "scale=426:240"
	default:
		return "scale=1920:1080" // Default to 1080p
	}
}

// getCodecForFormat returns the codec name used for each format
func getCodecForFormat(format string) string {
	switch format {
	case "mp4":
		return "libx264-high444"
	case "mp4-h265":
		return "libx265"
	case "3gp":
		return "mpeg4-simple"
	case "3gp-mobile":
		return "mpeg4-simple-mobile"
	case "3gp-h264":
		return "libx264-baseline"
	case "gif":
		return "gif"
	default:
		return "unknown"
	}
}
