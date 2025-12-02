package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"photonic/internal/config"
)

// RawProcessor defines the interface for RAW conversion tools.
type RawProcessor interface {
	Name() string
	IsAvailable() bool
	Convert(ctx context.Context, req RawConvertRequest) (RawConvertResult, error)
	BatchConvert(ctx context.Context, files []string, outputDir string) ([]string, error)
}

// RawConvertRequest contains conversion inputs.
type RawConvertRequest struct {
	InputFile  string
	OutputFile string
	XMPFile    string
	Config     interface{}
	TempDir    string
}

// RawConvertResult reports a conversion.
type RawConvertResult struct {
	InputFile     string
	OutputFile    string
	ToolUsed      string
	ProcessingLog string
	Success       bool
	Error         error
}

// RawProcessorManager manages multiple RAW processing tools.
type RawProcessorManager struct {
	processors map[string]RawProcessor
	config     *config.RawProcessing
}

// NewRawProcessorManager registers processors from config.
func NewRawProcessorManager(cfg *config.RawProcessing) *RawProcessorManager {
	manager := &RawProcessorManager{
		processors: make(map[string]RawProcessor),
		config:     cfg,
	}
	manager.RegisterProcessor(&DarktableProcessor{config: cfg.Tools.Darktable})
	manager.RegisterProcessor(&ImageMagickProcessor{config: cfg.Tools.ImageMagick, global: cfg})
	manager.RegisterProcessor(&DCrawProcessor{config: cfg.Tools.DCraw})
	manager.RegisterProcessor(&RawTherapeeProcessor{config: cfg.Tools.RawTherapee})
	return manager
}

// RegisterProcessor adds a processor by its Name().
func (m *RawProcessorManager) RegisterProcessor(proc RawProcessor) {
	if proc == nil {
		return
	}
	m.processors[proc.Name()] = proc
}

// GetBestProcessor returns preferred processor based on config and availability.
func (m *RawProcessorManager) GetBestProcessor() RawProcessor {
	if m == nil {
		return nil
	}
	if proc, ok := m.processors[m.config.DefaultTool]; ok && proc.IsAvailable() {
		return proc
	}
	priority := []string{"darktable", "rawtherapee", "imagemagick", "dcraw"}
	for _, name := range priority {
		if proc, ok := m.processors[name]; ok && proc.IsAvailable() {
			return proc
		}
	}
	return nil
}

// OutputPath builds an output filename based on config.
func (m *RawProcessorManager) OutputPath(input string, outputDir string) string {
	base := filepath.Base(input)
	ext := m.config.OutputFormat
	if ext == "" {
		ext = "jpg"
	}
	base = trimExt(base) + "." + ext

	// If no output directory specified, use same directory as input
	if outputDir == "" {
		return filepath.Join(filepath.Dir(input), base)
	}

	// Handle the case where outputDir might be a file instead of directory
	if stat, err := os.Stat(outputDir); err == nil && !stat.IsDir() {
		// If outputDir is actually a file, create a directory with a different name
		outputDir = outputDir + "_dir"
	}

	return filepath.Join(outputDir, base)
}

// EnsureOutputDirectory creates the output directory and handles conflicts
func (m *RawProcessorManager) EnsureOutputDirectory(outputPath string) error {
	outputDir := filepath.Dir(outputPath)

	// Check if something exists at this path
	if stat, err := os.Stat(outputDir); err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("cannot create output directory %s: file exists with same name", outputDir)
		}
		// Directory already exists, that's fine
		return nil
	}

	// Create the directory
	return os.MkdirAll(outputDir, 0o755)
}

// ConvertWithFallback tries multiple processors in order of preference
func (m *RawProcessorManager) ConvertWithFallback(ctx context.Context, inputFile, xmpFile, outputDir string) (RawConvertResult, error) {
	if m == nil {
		return RawConvertResult{}, fmt.Errorf("no raw processor manager available")
	}

	outputFile := m.OutputPath(inputFile, outputDir)

	// Ensure output directory can be created
	if err := m.EnsureOutputDirectory(outputFile); err != nil {
		return RawConvertResult{}, err
	}

	req := RawConvertRequest{
		InputFile:  inputFile,
		OutputFile: outputFile,
		XMPFile:    xmpFile,
		TempDir:    m.config.TempDir,
	}

	// Try processors in order of preference
	priority := []string{m.config.DefaultTool, "imagemagick", "darktable", "dcraw", "rawtherapee"}

	var lastResult RawConvertResult
	var errors []string

	for _, toolName := range priority {
		fmt.Printf("DEBUG: Checking tool: %s\n", toolName)
		if proc, exists := m.processors[toolName]; exists {
			fmt.Printf("DEBUG: Tool %s exists, checking availability\n", toolName)
			if proc.IsAvailable() {
				fmt.Printf("DEBUG: Using tool: %s\n", toolName)
				result, err := proc.Convert(ctx, req)
				lastResult = result

				if err == nil && result.Success {
					fmt.Printf("DEBUG: Tool %s succeeded\n", toolName)
					return result, nil
				}

				// Log the failure for debugging
				errorMsg := fmt.Sprintf("%s failed: %v", toolName, err)
				if result.ProcessingLog != "" {
					errorMsg += fmt.Sprintf(" (log: %s)", strings.TrimSpace(result.ProcessingLog))
				}
				errors = append(errors, errorMsg)
				fmt.Printf("DEBUG: Tool %s failed: %s\n", toolName, errorMsg)
			} else {
				fmt.Printf("DEBUG: Tool %s not available\n", toolName)
				errors = append(errors, fmt.Sprintf("%s not available or disabled", toolName))
			}
		} else {
			fmt.Printf("DEBUG: Tool %s does not exist\n", toolName)
			errors = append(errors, fmt.Sprintf("%s not available or disabled", toolName))
		}
	}

	// All processors failed
	combinedError := fmt.Sprintf("all raw processors failed for %s:\n  %s",
		inputFile, strings.Join(errors, "\n  "))

	return lastResult, fmt.Errorf("%s", combinedError)
}

// DetectAvailable returns list of available processor names
func (m *RawProcessorManager) DetectAvailable() []string {
	var available []string
	for name, proc := range m.processors {
		if proc.IsAvailable() {
			available = append(available, name)
		}
	}
	return available
}

// GetProcessor returns a specific processor by name
func (m *RawProcessorManager) GetProcessor(name string) RawProcessor {
	return m.processors[name]
}

// Processors returns the processor map for direct access
func (m *RawProcessorManager) Processors() map[string]RawProcessor {
	return m.processors
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func trimExt(name string) string {
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}
