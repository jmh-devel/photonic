package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"photonic/internal/fsutil"
	"sort"
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
	InputDir    string
	Output      string
	OutputDir   string // Optional: specify directory for multiple formats
	FPS         int
	Stabilize   bool
	Formats     []string // Supported: "mp4", "3gp", "3gp-h264", "gif"
	Resolution  string   // Optional legacy single output resolution.
	Resolutions []string // Optional multiple output resolutions.
	Quality     string   // H.264 quality: high, medium, or low.
}

// TimelapseResult captures output metadata.
type TimelapseResult struct {
	OutputFiles []OutputFile
	FrameCount  int
	UsedFFmpeg  bool
}

// OutputFile represents a single output file with its format details
type OutputFile struct {
	Path       string
	Format     string
	Codec      string
	Resolution string
	Width      int
	Height     int
	Size       int64 // file size in bytes
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
	if req.Quality == "" {
		req.Quality = "medium"
	}
	if req.Quality != "high" && req.Quality != "medium" && req.Quality != "low" {
		return TimelapseResult{}, fmt.Errorf("unsupported quality %q (choose high, medium, or low)", req.Quality)
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
	sort.Strings(allImages)

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
	sort.Strings(processFiles)

	if len(processFiles) == 0 {
		return TimelapseResult{}, fmt.Errorf("no processable images found")
	}

	// Clean up temp files when done
	defer func() {
		if needsCleanup {
			os.RemoveAll(tempDir)
		}
	}()

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
	resolutions := req.Resolutions
	if len(resolutions) == 0 {
		resolutions = []string{req.Resolution}
	}
	if len(resolutions) == 0 || (len(resolutions) == 1 && resolutions[0] == "") {
		resolutions = []string{"1080p"}
	}
	for _, format := range req.Formats {
		for _, resolution := range resolutions {
			width, height, canonical, err := resolutionSize(resolution)
			if err != nil {
				return TimelapseResult{}, err
			}
			outputBase := baseOutput
			if len(resolutions) > 1 || req.OutputDir != "" {
				outputBase += "-" + canonical
			}
			outputFile, err := generateTimelapseFormat(ctx, processFiles, outputBase, format, req.FPS, canonical, req.Quality, width, height)
			if err != nil {
				logger.Error("failed to generate rendition", "format", format, "resolution", canonical, "error", err)
				return TimelapseResult{OutputFiles: outputFiles, FrameCount: len(processFiles), UsedFFmpeg: true}, err
			}
			outputFiles = append(outputFiles, outputFile)
			logger.Info("generated timelapse rendition successfully", "format", format, "resolution", canonical, "output_file", outputFile.Path, "codec", outputFile.Codec)
		}
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
func generateTimelapseFormat(ctx context.Context, frames []string, baseOutput, format string, fps int, resolution, quality string, width, height int) (OutputFile, error) {
	logger := slog.Default()

	var outputPath string
	var args []string

	fitFilter := getVideoFilter(resolution)
	baseArgs := []string{"-y"}
	if framesHaveSameExtension(frames) {
		// The normal Darktable export is a homogeneous TIFF sequence. The concat
		// demuxer streams those frames one at a time instead of opening hundreds
		// of full-resolution TIFF decoders simultaneously.
		manifestDir, err := os.MkdirTemp("", "photonic-timelapse-*")
		if err != nil {
			return OutputFile{}, fmt.Errorf("create frame manifest directory: %w", err)
		}
		defer os.RemoveAll(manifestDir)
		manifest := filepath.Join(manifestDir, "frames.ffconcat")
		if err := writeConcatManifest(manifest, frames, fps); err != nil {
			return OutputFile{}, err
		}
		baseArgs = append(baseArgs, "-f", "concat", "-safe", "0", "-i", manifest, "-r", fmt.Sprint(fps), "-vf", fitFilter)
	} else {
		// Mixed formats need separate decoders, as the concat demuxer assumes all
		// files use the first frame's codec. Pass filenames as argv elements.
		for _, frame := range frames {
			baseArgs = append(baseArgs, "-loop", "1", "-framerate", fmt.Sprint(fps), "-t", fmt.Sprintf("%.9f", 1/float64(fps)), "-i", frame)
		}
		filterParts := make([]string, 0, len(frames)+1)
		concatInputs := make([]string, 0, len(frames))
		for i := range frames {
			label := fmt.Sprintf("v%d", i)
			filterParts = append(filterParts, fmt.Sprintf("[%d:v]%s,setpts=PTS-STARTPTS[%s]", i, fitFilter, label))
			concatInputs = append(concatInputs, "["+label+"]")
		}
		filterParts = append(filterParts, strings.Join(concatInputs, "")+fmt.Sprintf("concat=n=%d:v=1:a=0[outv]", len(frames)))
		baseArgs = append(baseArgs, "-filter_complex", strings.Join(filterParts, ";"), "-map", "[outv]", "-r", fmt.Sprint(fps))
	}

	switch format {
	case "mp4":
		outputPath = baseOutput + ".mp4"
		// Backup existing file if it exists
		if err := backupExistingFile(outputPath); err != nil {
			logger.Warn("failed to backup existing file", "file", outputPath, "error", err)
		}
		args = append(baseArgs,
			"-c:v", "libx264",
			"-profile:v", "high",
			"-pix_fmt", "yuv420p",
			"-crf", getTimelapseCRF(quality),
			"-preset", "slow",
			"-movflags", "+faststart",
		)
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
			"-movflags", "+faststart",
		)
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
		args = append(baseArgs, outputPath)

	default:
		return OutputFile{}, fmt.Errorf("unsupported format: %s", format)
	}

	logger.Info("executing ffmpeg command",
		"format", format,
		"frame_count", len(frames),
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
		Path:       outputPath,
		Format:     format,
		Codec:      codec,
		Resolution: resolution,
		Width:      width,
		Height:     height,
		Size:       stat.Size(),
	}, nil
}

func framesHaveSameExtension(frames []string) bool {
	if len(frames) < 2 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(frames[0]))
	for _, frame := range frames[1:] {
		if strings.ToLower(filepath.Ext(frame)) != ext {
			return false
		}
	}
	return true
}

func writeConcatManifest(path string, frames []string, fps int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create frame manifest: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, "ffconcat version 1.0"); err != nil {
		return err
	}
	for _, frame := range frames {
		abs, err := filepath.Abs(frame)
		if err != nil {
			return fmt.Errorf("resolve frame path %q: %w", frame, err)
		}
		quoted := strings.ReplaceAll(strings.ReplaceAll(abs, `\`, `\\`), `'`, `\'`)
		if _, err := fmt.Fprintf(f, "file '%s'\nduration %.9f\n", quoted, 1/float64(fps)); err != nil {
			return err
		}
	}
	if len(frames) > 0 {
		abs, err := filepath.Abs(frames[len(frames)-1])
		if err != nil {
			return err
		}
		quoted := strings.ReplaceAll(strings.ReplaceAll(abs, `\`, `\\`), `'`, `\'`)
		if _, err := fmt.Fprintf(f, "file '%s'\n", quoted); err != nil {
			return err
		}
	}
	return f.Close()
}

// getVideoFilter returns the appropriate video filter for resolution scaling
func getVideoFilter(resolution string) string {
	w, h, _, err := resolutionSize(resolution)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2:flags=lanczos,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1", w, h, w, h)
}

func resolutionSize(resolution string) (int, int, string, error) {
	switch strings.ToLower(resolution) {
	case "4k", "2160p", "uhd":
		return 3840, 2160, "4k", nil
	case "1440p":
		return 2560, 1440, "1440p", nil
	case "1080p":
		return 1920, 1080, "1080p", nil
	case "720p":
		return 1280, 720, "720p", nil
	case "480p":
		return 854, 480, "480p", nil
	case "240p":
		return 426, 240, "240p", nil
	default:
		return 0, 0, "", fmt.Errorf("unsupported resolution %q (choose 4k, 1440p, 1080p, 720p, 480p, or 240p)", resolution)
	}
}

func getTimelapseCRF(quality string) string {
	switch quality {
	case "high":
		return "18"
	case "low":
		return "23"
	default:
		return "20"
	}
}

// getCodecForFormat returns the codec name used for each format
func getCodecForFormat(format string) string {
	switch format {
	case "mp4":
		return "libx264"
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
