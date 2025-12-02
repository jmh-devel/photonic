package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"photonic/internal/fsutil"
)

// isDirectory checks if a path is an existing directory
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// PanoramicRequest defines inputs for stitching.
type PanoramicRequest struct {
	InputDir   string
	Output     string
	Projection string // cylindrical, spherical, planar, etc.
	Blending   string // multiband, feather, etc.
	Quality    string // fast, normal, high, ultra
	Rotation   string // 90, 180, 270, cw, ccw (deprecated - auto-detected)
	Aggression string // low, moderate, high - control point cleaning aggressiveness
}

// PanoramicResult captures output metadata.
type PanoramicResult struct {
	OutputFile    string
	Projection    string
	Blending      string
	Quality       string
	Aggression    string
	ImageCount    int
	Dimensions    string
	ToolUsed      string
	ProcessedWith string
}

// AssemblePanoramic creates panoramic images using Hugin tools with intelligent fallbacks.
func AssemblePanoramic(ctx context.Context, req PanoramicRequest) (PanoramicResult, error) {
	logger := slog.Default()

	// Set defaults
	if req.Projection == "" {
		req.Projection = "cylindrical"
	}
	if req.Blending == "" {
		req.Blending = "multiband"
	}
	if req.Quality == "" {
		req.Quality = "normal"
	}
	if req.Aggression == "" {
		req.Aggression = "moderate" // Default to moderate cleaning (best for rainbows!)
	}

	output := req.Output
	// If output is a directory or ends with separator, create a filename
	if output == "" || strings.HasSuffix(output, string(filepath.Separator)) || isDirectory(output) {
		if output == "" {
			output = "./output"
		}
		// Ensure output directory exists and create proper filename
		if err := os.MkdirAll(output, 0o755); err != nil {
			return PanoramicResult{}, fmt.Errorf("failed to create output directory: %v", err)
		}
		output = filepath.Join(output, "panoramic.jpg")
	} else {
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return PanoramicResult{}, fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	logger.Info("starting panoramic processing",
		"input_dir", req.InputDir,
		"output", output,
		"projection", req.Projection,
		"blending", req.Blending,
		"quality", req.Quality,
	)

	// Get all images from directory
	allImages, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		logger.Error("failed to list images", "error", err, "input_dir", req.InputDir)
		return PanoramicResult{}, err
	}

	if len(allImages) == 0 {
		logger.Error("no images found", "input_dir", req.InputDir)
		return PanoramicResult{}, fmt.Errorf("no images found in %s", req.InputDir)
	}

	logger.Info("found images for panoramic stitching",
		"total_images", len(allImages),
		"input_dir", req.InputDir,
	)

	// Check if we're using cached images (potential orientation issues)
	if strings.Contains(req.InputDir, "processed") {
		logger.Warn("CACHE WARNING: Using cached images for panoramic - may cause orientation issues. Consider clearing cache for best results.", "input_dir", req.InputDir)
	}

	// Handle RAW files - process them first if needed
	rawFiles, processedFiles := fsutil.SeparateRAWAndProcessed(allImages)
	var workImages []string
	var tempDir string
	var needsCleanup bool

	if len(rawFiles) > 0 {
		logger.Info("processing RAW files for panoramic",
			"raw_count", len(rawFiles),
			"processed_count", len(processedFiles),
		)

		// Create temp directory for converted RAW files
		tempDir = filepath.Join(os.TempDir(), "photonic_panoramic_"+fmt.Sprint(os.Getpid()))
		convertedFiles, err := ConvertRAWBatch(ctx, rawFiles, tempDir)
		if err != nil {
			return PanoramicResult{}, fmt.Errorf("RAW conversion failed: %v", err)
		}

		workImages = append(workImages, convertedFiles...)
		needsCleanup = true
	}

	// Add already processed files
	workImages = append(workImages, processedFiles...)

	if len(workImages) < 2 {
		return PanoramicResult{}, fmt.Errorf("need at least 2 images for panoramic stitching, got %d", len(workImages))
	}

	// Clean up temp files when done
	defer func() {
		if needsCleanup && tempDir != "" {
			logger.Info("cleaning up temporary files", "temp_dir", tempDir)
			os.RemoveAll(tempDir)
		}
	}()

	result := PanoramicResult{
		OutputFile: output,
		Projection: req.Projection,
		Blending:   req.Blending,
		Quality:    req.Quality,
		ImageCount: len(workImages),
	}

	// Try Hugin-based stitching first
	if isHuginAvailable() {
		logger.Info("using Hugin for panoramic stitching", "images", len(workImages))
		err := stitchWithHugin(ctx, workImages, output, req.Projection, req.Blending, req.Quality, req.Aggression, logger)
		if err == nil {
			result.ToolUsed = "hugin"
			result.ProcessedWith = "hugin/executor"
			if req.Aggression != "" {
				result.ProcessedWith += "+cleaning:" + req.Aggression
			}
			if len(workImages) > 0 {
				result.Dimensions = identifyDimensions(workImages[0])
			}

			logger.Info("panoramic stitching completed successfully",
				"tool", "hugin",
				"output", result.OutputFile,
				"images_used", len(workImages),
			)
			return result, nil
		}
		logger.Warn("Hugin stitching failed, trying fallback", "error", err)
	}

	// Fallback to simple horizontal append with ImageMagick
	if commandExists("convert") && len(workImages) > 1 {
		logger.Info("using ImageMagick fallback for panoramic", "images", len(workImages))
		args := append(workImages, "+append", output)
		cmd := exec.CommandContext(ctx, "convert", args...)
		if err := cmd.Run(); err == nil {
			result.ToolUsed = "imagemagick"
			result.ProcessedWith = "convert +append"
			if len(workImages) > 0 {
				result.Dimensions = identifyDimensions(workImages[0])
			}
			logger.Info("panoramic stitching completed with ImageMagick fallback",
				"output", output,
				"images_used", len(workImages),
			)
			return result, nil
		}
		logger.Warn("ImageMagick fallback failed", "error", err)
	}

	// Last resort: create manifest file
	logger.Warn("all panoramic stitching methods failed, creating manifest")
	manifestPath := strings.TrimSuffix(output, filepath.Ext(output)) + ".txt"
	content := fmt.Sprintf("Panoramic stitching failed\nProjection: %s\nBlending: %s\nImages: %d\nInput directory: %s",
		req.Projection, req.Blending, len(workImages), req.InputDir)
	if err := TouchManifest(manifestPath, content); err != nil {
		return PanoramicResult{}, err
	}

	result.OutputFile = manifestPath
	result.ToolUsed = "manifest"
	result.ProcessedWith = "fallback"

	return result, nil
}

// isHuginAvailable checks if Hugin tools are available
func isHuginAvailable() bool {
	tools := []string{"pto_gen", "cpfind", "cpclean", "celeste_standalone", "linefind", "autooptimiser", "pano_modify", "nona", "enblend"}
	for _, tool := range tools {
		if !commandExists(tool) {
			return false
		}
	}
	return true
}

// stitchWithHugin performs panoramic stitching using Hugin tools
func stitchWithHugin(ctx context.Context, images []string, output, projection, blending, quality, aggression string, logger *slog.Logger) error {
	logger.Info("DEBUG: stitchWithHugin called with parameters", "projection", projection, "blending", blending, "quality", quality, "aggression", aggression, "image_count", len(images))

	// Create temporary directory for Hugin processing
	workDir := filepath.Join(os.TempDir(), "photonic_hugin_"+fmt.Sprint(os.Getpid()))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("failed to create work directory: %v", err)
	}
	defer os.RemoveAll(workDir)

	// Step 1: Generate project file with pto_gen
	ptoFile := filepath.Join(workDir, "project.pto")
	logger.Info("generating Hugin project file", "pto_file", ptoFile, "images", len(images))

	args := append([]string{"-o", ptoFile}, images...)
	cmd := exec.CommandContext(ctx, "pto_gen", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pto_gen failed: %v, output: %s", err, string(output))
	}

	// Step 2: Find control points with cpfind
	logger.Info("finding control points between images")
	cpFile := filepath.Join(workDir, "project_cp.pto")
	cmd = exec.CommandContext(ctx, "cpfind", "--multirow", "-o", cpFile, ptoFile)
	logger.Info("DEBUG: executing cpfind command", "cmd", "cpfind", "args", []string{"--multirow", "-o", cpFile, ptoFile})
	if cmdOutput, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("cpfind failed, using project without control points", "error", err, "output", string(cmdOutput))
		cpFile = ptoFile // Fall back to original
	}

	// Diagnostic: Count control points found
	if cpCount := countControlPoints(cpFile); cpCount > 0 {
		logger.Info("control points found", "count", cpCount, "file", cpFile)
	} else {
		logger.Warn("no control points found - images may not overlap sufficiently")
	}

	// Step 3: Clean control points with cpclean (if control points were found)
	cleanedFile := cpFile
	if cpFile != ptoFile {
		// Configure aggression level for control point cleaning
		var distanceThreshold string
		var aggressionDesc string
		switch aggression {
		case "low":
			distanceThreshold = "4"
			aggressionDesc = "gentle"
		case "moderate":
			distanceThreshold = "3"
			aggressionDesc = "moderate"
		case "high":
			distanceThreshold = "2"
			aggressionDesc = "aggressive"
		default:
			distanceThreshold = "3" // Default to moderate
			aggressionDesc = "moderate"
		}

		logger.Info("cleaning control points with configurable filtering", "aggression", aggression, "threshold", distanceThreshold, "description", aggressionDesc)
		cleanedFile = filepath.Join(workDir, "project_cleaned.pto")
		cmd = exec.CommandContext(ctx, "cpclean", "--max-distance", distanceThreshold, "-o", cleanedFile, cpFile)
		logger.Info("DEBUG: executing cpclean command", "cmd", "cpclean", "args", []string{"--max-distance", distanceThreshold, "-o", cleanedFile, cpFile})
		if cmdOutput, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("cpclean failed, trying basic cleaning", "aggression", aggressionDesc, "error", err, "output", string(cmdOutput))
			// Fallback to basic cpclean without distance threshold
			cmd = exec.CommandContext(ctx, "cpclean", "-o", cleanedFile, cpFile)
			if cmdOutput, err := cmd.CombinedOutput(); err != nil {
				logger.Warn("basic cpclean also failed, using uncleaned control points", "error", err, "output", string(cmdOutput))
				cleanedFile = cpFile // Fall back to uncleaned
			}
		}
	}

	// Step 4: Skip celeste to preserve rainbow and gradient control points (working settings)
	celesteFile := cleanedFile
	logger.Info("skipping celeste to preserve rainbow and gradient control points")

	// Step 5: Find vertical lines with linefind (helps with alignment)
	linefindFile := celesteFile
	logger.Info("finding vertical lines for better alignment")
	linefindFile = filepath.Join(workDir, "project_lines.pto")
	cmd = exec.CommandContext(ctx, "linefind", "-o", linefindFile, celesteFile)
	logger.Info("DEBUG: executing linefind command", "cmd", "linefind", "args", []string{"-o", linefindFile, celesteFile})
	if cmdOutput, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("linefind failed, skipping line detection", "error", err, "output", string(cmdOutput))
		linefindFile = celesteFile // Fall back to celeste
	}

	// Step 6: Optimize the project (geometrical and photometric)
	optimizedPto := filepath.Join(workDir, "optimized.pto")
	logger.Info("optimizing panoramic project", "input_pto", linefindFile, "output_pto", optimizedPto)

	// Use more conservative optimization to avoid misalignment
	// -a: optimize positions and barrel distortion
	// -m: optimize photometric parameters
	// -l: optimize lens parameters
	// -s: straighten panorama
	cmd = exec.CommandContext(ctx, "autooptimiser", "-a", "-m", "-l", "-s", "-o", optimizedPto, linefindFile)
	logger.Info("DEBUG: executing autooptimiser command", "cmd", "autooptimiser", "args", []string{"-a", "-m", "-l", "-s", "-o", optimizedPto, linefindFile})
	if cmdOutput, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("full autooptimiser failed, trying position-only optimization", "error", err, "output", string(cmdOutput))
		// Fallback: try position-only optimization to avoid lens parameter issues
		cmd = exec.CommandContext(ctx, "autooptimiser", "-a", "-s", "-o", optimizedPto, linefindFile)
		if cmdOutput, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("position-only autooptimiser also failed, using original project", "error", err, "output", string(cmdOutput))
			optimizedPto = linefindFile // Fall back to linefind project
		} else {
			logger.Info("position-only autooptimiser completed successfully")
		}
	} else {
		logger.Info("full autooptimiser completed successfully")
		// Log image count and basic stats from optimized project
		if stats := analyzePTOFile(optimizedPto); stats != nil {
			logger.Info("panoramic project analysis", "images", stats["images"], "control_points", stats["control_points"], "estimated_fov", stats["fov"])
		}
	}

	// Step 7: Set projection type in the PTO file
	if err := updatePTOProjection(optimizedPto, projection); err != nil {
		logger.Warn("failed to update projection, using default", "error", err)
	}

	// Step 8: Calculate optimal canvas size and crop with pano_modify
	finalPto := filepath.Join(workDir, "final.pto")
	logger.Info("calculating optimal canvas size and crop")
	cmd = exec.CommandContext(ctx, "pano_modify", "--canvas=AUTO", "--crop=AUTO", "-o", finalPto, optimizedPto)
	if cmdOutput, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("pano_modify failed, using project without canvas optimization", "error", err, "output", string(cmdOutput))
		finalPto = optimizedPto // Fall back to optimized project
	} else {
		logger.Info("pano_modify completed successfully")
		// Log detailed PTO analysis before nona
		if stats := analyzePTOFile(finalPto); stats != nil {
			logger.Info("final PTO analysis", "images", stats["images"], "control_points", stats["control_points"], "canvas_info", extractCanvasInfo(finalPto))
		}
	}

	// Step 9: Try hugin_executor for complete stitching workflow first
	logger.Info("attempting complete stitching with hugin_executor", "projection", projection, "quality", quality)

	args = []string{"--stitching", "--prefix=" + filepath.Join(workDir, "executor"), finalPto}
	logger.Info("DEBUG: executing hugin_executor command", "cmd", "hugin_executor", "args", args)
	cmd = exec.CommandContext(ctx, "hugin_executor", args...)
	if cmdOutput, err := cmd.CombinedOutput(); err == nil {
		// hugin_executor succeeded - find the output file
		pattern := filepath.Join(workDir, "executor*.tif")
		matches, _ := filepath.Glob(pattern)
		if len(matches) == 0 {
			// Try jpg pattern
			pattern = filepath.Join(workDir, "executor*.jpg")
			matches, _ = filepath.Glob(pattern)
		}

		if len(matches) > 0 {
			// Copy the result to output
			if err := copyFile(matches[0], output); err == nil {
				logger.Info("hugin_executor completed successfully", "output", output, "source", matches[0])
				return nil
			}
		}
		logger.Warn("hugin_executor completed but no output files found", "output", string(cmdOutput))
	} else {
		logger.Warn("hugin_executor failed, falling back to manual nona+enblend", "error", err, "output", string(cmdOutput))
	}

	// Fallback: Manual nona + enblend approach with smaller tile sizes
	logger.Info("rendering panoramic with nona (manual approach)", "projection", projection, "quality", quality)

	outputPrefix := filepath.Join(workDir, "pano")
	args = []string{"-o", outputPrefix, "-m", "TIFF_m"}

	// Quality settings for nona
	switch quality {
	case "fast":
		args = append(args, "-i", "0") // Linear interpolation
	case "high":
		args = append(args, "-i", "2") // Cubic interpolation
	case "ultra":
		args = append(args, "-i", "3", "-a") // Sinc interpolation with antialiasing
	default: // normal
		args = append(args, "-i", "1") // Poly3 interpolation
	}

	args = append(args, finalPto)

	// Log the full nona command for debugging
	logger.Info("executing nona command", "cmd", "nona", "args", args)

	cmd = exec.CommandContext(ctx, "nona", args...)
	if cmdOutput, err := cmd.CombinedOutput(); err != nil {
		logger.Error("nona execution failed", "error", err, "output", string(cmdOutput), "args", args)
		return fmt.Errorf("nona failed: %v, output: %s", err, string(cmdOutput))
	} else {
		logger.Info("nona execution completed", "output", string(cmdOutput))
	}

	// Step 10: Blend the images with enblend
	logger.Info("blending panoramic images", "blending", blending)

	// Find all the output files from nona (usually pano0000.tif, pano0001.tif, etc.)
	pattern := outputPrefix + "*.tif"
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return fmt.Errorf("no output files found from nona: %v, pattern: %s", err, pattern)
	}

	logger.Info("found nona output files for blending", "file_count", len(matches), "files", matches)

	args = []string{"-o", output}

	// Blending options
	switch blending {
	case "feather":
		args = append(args, "--no-optimize") // Simple feathering
	case "multiband":
		args = append(args, "--levels=29") // Full multiband blending
	case "none":
		args = append(args, "--no-blend") // No blending
	default: // multiband default
		args = append(args, "--levels=29")
	}

	args = append(args, matches...)
	cmd = exec.CommandContext(ctx, "enblend", args...)
	if cmdOutput, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enblend failed: %v, output: %s", err, string(cmdOutput))
	}

	logger.Info("Hugin panoramic stitching completed successfully", "output", output)
	return nil
}

// updatePTOProjection modifies the PTO file to set the desired projection
func updatePTOProjection(ptoFile, projection string) error {
	content, err := os.ReadFile(ptoFile)
	if err != nil {
		return err
	}

	// Map projection names to Hugin projection numbers
	projectionMap := map[string]string{
		"cylindrical":   "1",
		"spherical":     "2",
		"planar":        "0",
		"fisheye":       "3",
		"stereographic": "5",
		"mercator":      "6",
	}

	projNum, exists := projectionMap[projection]
	if !exists {
		projNum = "1" // Default to cylindrical
	}

	// Replace the projection parameter in the p line
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "p ") {
			// Update the f parameter (projection type)
			parts := strings.Fields(line)
			for j, part := range parts {
				if strings.HasPrefix(part, "f") {
					parts[j] = "f" + projNum
					break
				}
			}
			lines[i] = strings.Join(parts, " ")
			break
		}
	}

	return os.WriteFile(ptoFile, []byte(strings.Join(lines, "\n")), 0o644)
}

// countControlPoints counts the number of control points in a PTO file
func countControlPoints(ptoFile string) int {
	content, err := os.ReadFile(ptoFile)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "c ") {
			count++
		}
	}
	return count
}

// analyzePTOFile provides basic analysis of a PTO file
func analyzePTOFile(ptoFile string) map[string]interface{} {
	content, err := os.ReadFile(ptoFile)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	stats := map[string]interface{}{
		"images":         0,
		"control_points": 0,
		"fov":            "unknown",
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "i ") {
			stats["images"] = stats["images"].(int) + 1
		} else if strings.HasPrefix(line, "c ") {
			stats["control_points"] = stats["control_points"].(int) + 1
		} else if strings.HasPrefix(line, "p ") {
			// Extract field of view from p line
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "v") {
					stats["fov"] = part[1:] // Remove 'v' prefix
					break
				}
			}
		}
	}

	return stats
}

// extractCanvasInfo extracts canvas size and crop information from PTO file
func extractCanvasInfo(ptoFile string) string {
	content, err := os.ReadFile(ptoFile)
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "p ") {
			// Extract canvas width and height from p line
			// Format: p f1 w3000 h2000 v70 ...
			parts := strings.Fields(line)
			var width, height, fov string
			for _, part := range parts {
				if strings.HasPrefix(part, "w") {
					width = part[1:]
				} else if strings.HasPrefix(part, "h") {
					height = part[1:]
				} else if strings.HasPrefix(part, "v") {
					fov = part[1:]
				}
			}
			return fmt.Sprintf("canvas=%sx%s, fov=%s", width, height, fov)
		}
	}
	return "no_canvas_info"
}

// applyRotation rotates an image and returns the new output path
func applyRotation(inputPath, rotation string, logger *slog.Logger) (string, error) {
	if rotation == "" {
		return inputPath, nil
	}

	// Parse rotation value
	var rotateDegrees string
	switch rotation {
	case "90", "cw":
		rotateDegrees = "90"
	case "180":
		rotateDegrees = "180"
	case "270", "ccw":
		rotateDegrees = "270"
	default:
		return inputPath, fmt.Errorf("unsupported rotation: %s (supported: 90, 180, 270, cw, ccw)", rotation)
	}

	// Create rotated output filename
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)
	rotatedPath := base + "_rotated" + ext

	// Use ImageMagick convert to rotate the image
	cmd := exec.Command("convert", inputPath, "-rotate", rotateDegrees, rotatedPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return inputPath, fmt.Errorf("rotation failed: %v, output: %s", err, string(output))
	}

	// Replace original with rotated version
	if err := os.Rename(rotatedPath, inputPath); err != nil {
		// If rename fails, try to copy and remove
		if copyErr := copyFile(rotatedPath, inputPath); copyErr == nil {
			os.Remove(rotatedPath)
		} else {
			return inputPath, fmt.Errorf("failed to replace original with rotated image: %v", err)
		}
	}

	logger.Info("image rotation completed", "degrees", rotateDegrees, "path", inputPath)
	return inputPath, nil
}

// rotateImages applies rotation to a list of images and returns the rotated paths
func rotateImages(images []string, rotation string, logger *slog.Logger) ([]string, error) {
	var rotatedImages []string

	for i, imagePath := range images {
		rotatedPath, err := applyRotation(imagePath, rotation, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to rotate image %d (%s): %v", i+1, imagePath, err)
		}
		rotatedImages = append(rotatedImages, rotatedPath)
	}

	return rotatedImages, nil
}
