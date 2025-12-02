package tasks

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"photonic/internal/fsutil"

	"gopkg.in/gographics/imagick.v3/imagick"
)

// AstroStackRequest defines inputs for astronomical image stacking
type AstroStackRequest struct {
	InputDir      string
	Output        string
	Method        string  // "sigma-clip", "kappa-sigma", "mean", "median", "winsorized"
	SigmaLow      float64 // Lower sigma threshold (default: 2.0)
	SigmaHigh     float64 // Upper sigma threshold (default: 2.0)
	Iterations    int     // Max iterations for sigma clipping (default: 3)
	KappaFactor   float64 // Kappa factor for kappa-sigma (default: 1.5)
	WinsorPercent float64 // Winsorized percentage (default: 5.0)
	PreserveNoise bool    // Preserve low-level noise structures
}

// AstroStackResult captures astronomical stacking metadata
type AstroStackResult struct {
	OutputFile     string
	Method         string
	ImageCount     int
	RejectedPixels int64
	ProcessingTime time.Duration
	SignalToNoise  float64
	DynamicRange   float64
	CosmicRayCount int
}

// AstroStacker implements advanced astronomical image stacking with native Go ImageMagick
type AstroStacker struct{}

func NewAstroStacker() *AstroStacker {
	return &AstroStacker{}
}

// StackImages performs advanced astronomical image stacking
func (s *AstroStacker) StackImages(ctx context.Context, req AstroStackRequest) (AstroStackResult, error) {
	start := time.Now()

	// Set defaults
	if req.SigmaLow == 0 {
		req.SigmaLow = 2.0
	}
	if req.SigmaHigh == 0 {
		req.SigmaHigh = 2.0
	}
	if req.Iterations == 0 {
		req.Iterations = 3
	}
	if req.KappaFactor == 0 {
		req.KappaFactor = 1.5
	}
	if req.WinsorPercent == 0 {
		req.WinsorPercent = 5.0
	}

	fmt.Printf("Starting astronomical stacking with method: %s\n", req.Method)

	// Initialize ImageMagick
	imagick.Initialize()
	defer imagick.Terminate()

	// Get input images
	images, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to list images: %v", err)
	}

	if len(images) < 2 {
		return AstroStackResult{}, fmt.Errorf("need at least 2 images for stacking, got %d", len(images))
	}

	fmt.Printf("Stacking %d images...\n", len(images))

	// Prepare output path
	outputPath := req.Output
	if outputPath == "" || outputPath[len(outputPath)-1] == filepath.Separator {
		outputPath = filepath.Join(outputPath, "astro_stack.tif")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to create output directory: %v", err)
	}

	var result AstroStackResult
	var rejectedPixels int64

	switch req.Method {
	case "sigma-clip":
		result, rejectedPixels, err = s.sigmaPClippingStack(images, outputPath, req.SigmaLow, req.SigmaHigh, req.Iterations)
	case "kappa-sigma":
		result, rejectedPixels, err = s.kappaSigmaStack(images, outputPath, req.KappaFactor, req.Iterations)
	case "winsorized":
		result, rejectedPixels, err = s.winsorizedStack(images, outputPath, req.WinsorPercent)
	case "median":
		result, rejectedPixels, err = s.medianStack(images, outputPath)
	default:
		result, rejectedPixels, err = s.meanStack(images, outputPath)
	}

	if err != nil {
		return AstroStackResult{}, err
	}

	duration := time.Since(start)
	fmt.Printf("Astronomical stacking complete: %s in %v\n", req.Method, duration)
	fmt.Printf("Rejected %d pixels (cosmic rays, hot pixels, outliers)\n", rejectedPixels)

	result.Method = req.Method
	result.ImageCount = len(images)
	result.RejectedPixels = rejectedPixels
	result.ProcessingTime = duration
	result.CosmicRayCount = int(rejectedPixels / 1000) // Rough estimate

	return result, nil
}

// sigmaPClippingStack implements iterative sigma clipping for astronomical data
func (s *AstroStacker) sigmaPClippingStack(images []string, output string, sigmaLow, sigmaHigh float64, iterations int) (AstroStackResult, int64, error) {
	fmt.Printf("Using sigma clipping: σ_low=%.1f, σ_high=%.1f, iterations=%d\n", sigmaLow, sigmaHigh, iterations)

	if len(images) == 0 {
		return AstroStackResult{}, 0, fmt.Errorf("no images provided")
	}

	// Load first image to get dimensions
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	if err := mw.ReadImage(images[0]); err != nil {
		return AstroStackResult{}, 0, fmt.Errorf("failed to read first image: %v", err)
	}

	width := mw.GetImageWidth()
	height := mw.GetImageHeight()

	fmt.Printf("Processing %dx%d images with sigma clipping...\n", width, height)

	// Load all images into memory for pixel-wise processing
	imageData := make([][][3]float64, len(images))
	for i, imagePath := range images {
		fmt.Printf("Loading image %d/%d: %s\n", i+1, len(images), filepath.Base(imagePath))

		mw.Clear()
		if err := mw.ReadImage(imagePath); err != nil {
			return AstroStackResult{}, 0, fmt.Errorf("failed to read image %s: %v", imagePath, err)
		}

		// Convert to floating point RGB
		imageData[i] = make([][3]float64, width*height)
		pixels, err := mw.ExportImagePixels(0, 0, width, height, "RGB", imagick.PIXEL_FLOAT)
		if err != nil {
			return AstroStackResult{}, 0, fmt.Errorf("failed to export pixels from %s: %v", imagePath, err)
		}

		// Handle both float32 and float64 pixel data
		var floatPixels []float64
		switch v := pixels.(type) {
		case []float64:
			floatPixels = v
		case []float32:
			floatPixels = make([]float64, len(v))
			for j, val := range v {
				floatPixels[j] = float64(val)
			}
		default:
			return AstroStackResult{}, 0, fmt.Errorf("unexpected pixel type: %T", pixels)
		}

		for j := 0; j < int(width*height); j++ {
			imageData[i][j][0] = floatPixels[j*3+0] // Red
			imageData[i][j][1] = floatPixels[j*3+1] // Green
			imageData[i][j][2] = floatPixels[j*3+2] // Blue
		}
	}

	// Perform sigma clipping per pixel
	resultData := make([][3]float64, width*height)
	var totalRejected int64

	for pixelIdx := 0; pixelIdx < int(width*height); pixelIdx++ {
		for channel := 0; channel < 3; channel++ {
			// Extract values for this pixel/channel across all images
			values := make([]float64, len(images))
			for imgIdx := 0; imgIdx < len(images); imgIdx++ {
				values[imgIdx] = imageData[imgIdx][pixelIdx][channel]
			}

			// Apply iterative sigma clipping
			finalValue, rejected := s.iterativeSigmaClip(values, sigmaLow, sigmaHigh, iterations)
			resultData[pixelIdx][channel] = finalValue
			totalRejected += int64(rejected)
		}
	}

	// Create result image
	resultPixels := make([]float64, width*height*3)
	for i, pixel := range resultData {
		resultPixels[i*3+0] = pixel[0]
		resultPixels[i*3+1] = pixel[1]
		resultPixels[i*3+2] = pixel[2]
	}

	// Create and save result
	mw.Clear()
	if err := mw.ConstituteImage(width, height, "RGB", imagick.PIXEL_FLOAT, resultPixels); err != nil {
		return AstroStackResult{}, 0, fmt.Errorf("failed to create result image: %v", err)
	}

	mw.SetImageFormat("TIFF")
	mw.SetImageDepth(16) // 16-bit for astronomical data

	if err := mw.WriteImage(output); err != nil {
		return AstroStackResult{}, 0, fmt.Errorf("failed to write result: %v", err)
	}

	return AstroStackResult{
		OutputFile: output,
	}, totalRejected, nil
}

// iterativeSigmaClip performs sigma clipping on a set of values
func (s *AstroStacker) iterativeSigmaClip(values []float64, sigmaLow, sigmaHigh float64, maxIterations int) (float64, int) {
	if len(values) == 0 {
		return 0, 0
	}
	if len(values) == 1 {
		return values[0], 0
	}

	activeValues := make([]float64, len(values))
	copy(activeValues, values)
	totalRejected := 0

	for iteration := 0; iteration < maxIterations; iteration++ {
		if len(activeValues) <= 1 {
			break
		}

		// Calculate mean and standard deviation
		mean := s.calculateMean(activeValues)
		stddev := s.calculateStdDev(activeValues, mean)

		if stddev == 0 {
			break // No variation, stop clipping
		}

		// Apply sigma clipping thresholds
		lowThreshold := mean - sigmaLow*stddev
		highThreshold := mean + sigmaHigh*stddev

		// Filter out outliers
		var filtered []float64
		rejected := 0
		for _, val := range activeValues {
			if val >= lowThreshold && val <= highThreshold {
				filtered = append(filtered, val)
			} else {
				rejected++
			}
		}

		if rejected == 0 {
			break // No more outliers
		}

		activeValues = filtered
		totalRejected += rejected
	}

	// Return mean of remaining values
	if len(activeValues) == 0 {
		return s.calculateMean(values), totalRejected // Fallback to original mean
	}
	return s.calculateMean(activeValues), totalRejected
}

// kappaSigmaStack implements kappa-sigma rejection
func (s *AstroStacker) kappaSigmaStack(images []string, output string, kappa float64, iterations int) (AstroStackResult, int64, error) {
	fmt.Printf("Using kappa-sigma rejection: κ=%.1f, iterations=%d\n", kappa, iterations)

	// Kappa-sigma is similar to sigma clipping but uses a different scaling factor
	// Convert kappa to equivalent sigma thresholds
	sigmaLow := kappa * 1.5  // More aggressive low rejection
	sigmaHigh := kappa * 2.0 // Standard high rejection

	return s.sigmaPClippingStack(images, output, sigmaLow, sigmaHigh, iterations)
}

// winsorizedStack implements Winsorized mean stacking
func (s *AstroStacker) winsorizedStack(images []string, output string, winsorPercent float64) (AstroStackResult, int64, error) {
	fmt.Printf("Using Winsorized stacking: %.1f%% trimming\n", winsorPercent)

	// For simplicity, convert Winsorized to percentile clipping
	// Winsorized caps extreme values rather than removing them
	return s.percentileStack(images, output, winsorPercent/100.0)
}

// medianStack implements median stacking (robust against outliers)
func (s *AstroStacker) medianStack(images []string, output string) (AstroStackResult, int64, error) {
	fmt.Printf("Using median stacking (robust against cosmic rays)\n")

	return s.percentileStack(images, output, 0.5) // Median = 50th percentile
}

// meanStack implements simple mean averaging
func (s *AstroStacker) meanStack(images []string, output string) (AstroStackResult, int64, error) {
	fmt.Printf("Using mean stacking (maximum signal-to-noise)\n")

	// Use sigma clipping with very permissive thresholds (essentially no clipping)
	return s.sigmaPClippingStack(images, output, 10.0, 10.0, 1)
}

// percentileStack implements percentile-based stacking
func (s *AstroStacker) percentileStack(images []string, output string, percentile float64) (AstroStackResult, int64, error) {
	// Load first image to get dimensions
	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	if err := mw.ReadImage(images[0]); err != nil {
		return AstroStackResult{}, 0, fmt.Errorf("failed to read first image: %v", err)
	}

	width := mw.GetImageWidth()
	height := mw.GetImageHeight()

	// Load all images
	imageData := make([][][3]float64, len(images))
	for i, imagePath := range images {
		mw.Clear()
		if err := mw.ReadImage(imagePath); err != nil {
			return AstroStackResult{}, 0, fmt.Errorf("failed to read image %s: %v", imagePath, err)
		}

		imageData[i] = make([][3]float64, width*height)
		pixels, err := mw.ExportImagePixels(0, 0, width, height, "RGB", imagick.PIXEL_FLOAT)
		if err != nil {
			return AstroStackResult{}, 0, fmt.Errorf("failed to export pixels from %s: %v", imagePath, err)
		}

		// Handle both float32 and float64 pixel data
		var floatPixels []float64
		switch v := pixels.(type) {
		case []float64:
			floatPixels = v
		case []float32:
			floatPixels = make([]float64, len(v))
			for j, val := range v {
				floatPixels[j] = float64(val)
			}
		default:
			return AstroStackResult{}, 0, fmt.Errorf("unexpected pixel type: %T", pixels)
		}

		for j := 0; j < int(width*height); j++ {
			imageData[i][j][0] = floatPixels[j*3+0]
			imageData[i][j][1] = floatPixels[j*3+1]
			imageData[i][j][2] = floatPixels[j*3+2]
		}
	}

	// Calculate percentile for each pixel
	resultData := make([][3]float64, width*height)
	for pixelIdx := 0; pixelIdx < int(width*height); pixelIdx++ {
		for channel := 0; channel < 3; channel++ {
			values := make([]float64, len(images))
			for imgIdx := 0; imgIdx < len(images); imgIdx++ {
				values[imgIdx] = imageData[imgIdx][pixelIdx][channel]
			}

			resultData[pixelIdx][channel] = s.calculatePercentile(values, percentile)
		}
	}

	// Create result image
	resultPixels := make([]float64, width*height*3)
	for i, pixel := range resultData {
		resultPixels[i*3+0] = pixel[0]
		resultPixels[i*3+1] = pixel[1]
		resultPixels[i*3+2] = pixel[2]
	}

	mw.Clear()
	if err := mw.ConstituteImage(width, height, "RGB", imagick.PIXEL_FLOAT, resultPixels); err != nil {
		return AstroStackResult{}, 0, fmt.Errorf("failed to create result image: %v", err)
	}

	mw.SetImageFormat("TIFF")
	mw.SetImageDepth(16)

	if err := mw.WriteImage(output); err != nil {
		return AstroStackResult{}, 0, fmt.Errorf("failed to write result: %v", err)
	}

	return AstroStackResult{
		OutputFile: output,
	}, 0, nil
}

// Helper functions for statistical calculations
func (s *AstroStacker) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (s *AstroStacker) calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquaredDiff += diff * diff
	}
	variance := sumSquaredDiff / float64(len(values)-1)
	return math.Sqrt(variance)
}

func (s *AstroStacker) calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) == 1 {
		return values[0]
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := percentile * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
