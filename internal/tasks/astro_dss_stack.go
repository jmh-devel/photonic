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

// DSSStacker implements astronomical stacking using DeepSkyStacker
type DSSStacker struct{}

func NewDSSStacker() *DSSStacker {
	return &DSSStacker{}
}

// IsAvailable checks if DeepSkyStacker CLI is available
func (s *DSSStacker) IsAvailable() bool {
	_, err := exec.LookPath("DeepSkyStackerCL")
	if err != nil {
		// Check standard installation path
		_, err = os.Stat("/opt/DeepSkyStacker/DeepSkyStackerCL")
	}
	return err == nil
}

// GetDSSPath returns the path to DeepSkyStackerCL
func (s *DSSStacker) GetDSSPath() string {
	if path, err := exec.LookPath("DeepSkyStackerCL"); err == nil {
		return path
	}
	return "/opt/DeepSkyStacker/DeepSkyStackerCL"
}

// StackImages performs astronomical stacking using DeepSkyStacker
func (s *DSSStacker) StackImages(ctx context.Context, req AstroStackRequest) (AstroStackResult, error) {
	start := time.Now()

	if !s.IsAvailable() {
		return AstroStackResult{}, fmt.Errorf("DeepSkyStacker not available - install from https://github.com/deepskystacker/DSS/releases")
	}

	fmt.Printf("Starting astronomical stacking with DeepSkyStacker (professional tool)...\n")

	// Get input images
	images, err := fsutil.ListImages(req.InputDir)
	if err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to list images: %v", err)
	}

	if len(images) < 2 {
		return AstroStackResult{}, fmt.Errorf("need at least 2 images for stacking, got %d", len(images))
	}

	// Create DSS file list in the input directory (DSS expects relative paths)
	dssFileList := filepath.Join(req.InputDir, "dss_stack.dssfilelist")
	if err := s.createDSSFileList(images, dssFileList); err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to create DSS file list: %v", err)
	}

	// Run DeepSkyStacker in the input directory (it works with relative paths)
	fmt.Printf("Stacking %d images with DeepSkyStacker...\n", len(images))

	dssPath := s.GetDSSPath()
	args := []string{"/r", "/s", "/OF32r", "/OC0", filepath.Base(dssFileList)}

	fmt.Printf("Running DeepSkyStacker with args: %v\n", append([]string{dssPath}, args...))

	cmd := exec.CommandContext(ctx, dssPath, args...)
	cmd.Dir = req.InputDir // Run in input directory so relative paths work

	output, err := cmd.CombinedOutput()
	if err != nil {
		return AstroStackResult{}, fmt.Errorf("DeepSkyStacker failed: %v\nOutput: %s", err, output)
	}

	duration := time.Since(start)

	// DSS saves as Autosave.tif in the input directory - move it to the requested output
	autosavePath := filepath.Join(req.InputDir, "Autosave.tif")
	if _, err := os.Stat(autosavePath); err != nil {
		return AstroStackResult{}, fmt.Errorf("DSS did not create expected output file: %v", err)
	}

	// Create output directory if needed
	if err := os.MkdirAll(filepath.Dir(req.Output), 0o755); err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Move Autosave.tif to the requested output path
	if err := os.Rename(autosavePath, req.Output); err != nil {
		return AstroStackResult{}, fmt.Errorf("failed to move DSS output: %v", err)
	} // Clean up DSS file list
	os.Remove(dssFileList)

	fmt.Printf("DeepSkyStacker astronomical stacking complete in %v\n", duration)
	if len(output) > 0 {
		fmt.Printf("DSS output:\n%s\n", output)
	}

	return AstroStackResult{
		OutputFile:     req.Output,
		Method:         "deepskystacker",
		ProcessingTime: duration,
		ImageCount:     len(images),
		SignalToNoise:  0, // DSS doesn't provide this metric easily
		RejectedPixels: 0, // DSS handles this internally
		CosmicRayCount: 0, // DSS handles this internally
	}, nil
}

// createDSSFileList creates a DeepSkyStacker file list in the correct text format
func (s *DSSStacker) createDSSFileList(images []string, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write DSS text format header
	file.WriteString("DSS file list\n")
	file.WriteString("CHECKED\tTYPE\tFILE\n")

	// Add each image as a light frame
	for _, image := range images {
		// Get just the filename without path for DSS
		filename := filepath.Base(image)
		file.WriteString(fmt.Sprintf("1\tlight\t%s\n", filename))
	}

	// Add basic DSS settings (minimal working set)
	file.WriteString("#V5WS#Register/StackAfter#true\n")
	file.WriteString("#V5WS#Stacking/Light_Method#1\n")
	file.WriteString("#V5WS#Stacking/Light_Kappa#2\n")
	file.WriteString("#V5WS#Stacking/Light_Iteration#5\n")
	file.WriteString("#V5WS#Stacking/SaveCalibrated#false\n")
	file.WriteString("#V5WS#Register/DetectionThreshold#0\n")
	file.WriteString("#V5WS#Register/PercentStack#80\n")
	file.WriteString("#V5WS#Register/UseAutoThreshold#true\n")

	return nil
}
