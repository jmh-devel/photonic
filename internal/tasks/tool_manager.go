package tasks

import (
	"fmt"
	"os/exec"
	"photonic/internal/config"
	"strings"
)

// ToolManager handles automatic tool selection and fallbacks
type ToolManager struct {
	cfg *config.Config
}

// NewToolManager creates a new tool manager with configuration
func NewToolManager(cfg *config.Config) *ToolManager {
	return &ToolManager{cfg: cfg}
}

// ToolStatus represents the availability of a tool
type ToolStatus struct {
	Available bool
	Version   string
	Path      string
	Error     error
}

// CheckTool verifies if a tool is available and working
func (tm *ToolManager) CheckTool(toolName string) ToolStatus {
	// Map logical tool names to actual binary names
	var binaryName string
	switch toolName {
	case "imagemagick":
		binaryName = "convert"
	case "hugin":
		// Check for key hugin tools
		binaryName = "pto_gen" // Primary hugin tool for project generation
	case "darktable":
		binaryName = "darktable-cli"
	case "rawtherapee":
		binaryName = "rawtherapee-cli"
	default:
		binaryName = toolName
	}

	path, err := exec.LookPath(binaryName)
	if err != nil {
		return ToolStatus{Available: false, Error: err}
	}

	// Try to get version for additional verification
	var versionCmd []string
	switch toolName {
	case "imagemagick":
		versionCmd = []string{"convert", "-version"}
	case "darktable":
		versionCmd = []string{"darktable-cli", "--version"}
	case "dcraw":
		versionCmd = []string{"dcraw"} // dcraw shows help when run without args
	case "rawtherapee":
		versionCmd = []string{"rawtherapee-cli", "-v"}
	case "ffmpeg":
		versionCmd = []string{"ffmpeg", "-version"}
	case "hugin":
		// Check multiple hugin tools are available
		huginTools := []string{"pto_gen", "cpfind", "autooptimiser", "hugin_executor"}
		for _, tool := range huginTools {
			if _, err := exec.LookPath(tool); err != nil {
				return ToolStatus{Available: false, Error: fmt.Errorf("missing hugin tool: %s", tool)}
			}
		}
		versionCmd = []string{"pto_gen", "--help"} // Use --help instead of --version
	case "ale":
		versionCmd = []string{"ale", "--version"}
	case "siril":
		versionCmd = []string{"siril", "--version"}
	case "enfuse":
		versionCmd = []string{"enfuse", "--version"}
	case "enblend":
		versionCmd = []string{"enblend", "--version"}
	case "align_image_stack":
		versionCmd = []string{"align_image_stack", "--help"} // Use --help instead of --version
	case "autopano-sift":
		versionCmd = []string{"autopano-sift", "--version"}
	default:
		// For unknown tools, just check if they exist
		return ToolStatus{Available: true, Path: path}
	}

	if len(versionCmd) > 0 {
		cmd := exec.Command(versionCmd[0], versionCmd[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Some tools (like dcraw) return non-zero exit codes for version/help
			// but still show useful output
			if len(output) > 0 {
				version := extractVersion(string(output))
				return ToolStatus{Available: true, Version: version, Path: path}
			}
			return ToolStatus{Available: false, Path: path, Error: err}
		}
		version := extractVersion(string(output))
		return ToolStatus{Available: true, Version: version, Path: path}
	}

	return ToolStatus{Available: true, Path: path}
}

// GetAvailableRAWTool returns the best available RAW processing tool
func (tm *ToolManager) GetAvailableRAWTool() (string, error) {
	tools := []string{tm.cfg.Tools.RAWProcessing.Preferred}
	tools = append(tools, tm.cfg.Tools.RAWProcessing.Fallbacks...)

	for _, tool := range tools {
		if status := tm.CheckTool(tool); status.Available {
			return tool, nil
		}
	}
	return "", fmt.Errorf("no available RAW processing tools found")
}

// GetAvailablePanoramicTool returns the best available panoramic stitching tool
func (tm *ToolManager) GetAvailablePanoramicTool() (string, error) {
	tools := []string{tm.cfg.Tools.PanoramicTool.Preferred}
	tools = append(tools, tm.cfg.Tools.PanoramicTool.Fallbacks...)

	for _, tool := range tools {
		if status := tm.CheckTool(tool); status.Available {
			return tool, nil
		}
	}
	return "", fmt.Errorf("no available panoramic stitching tools found")
}

// GetAvailableStackingTool returns the best available image stacking tool
func (tm *ToolManager) GetAvailableStackingTool() (string, error) {
	tools := []string{tm.cfg.Tools.StackingTool.Preferred}
	tools = append(tools, tm.cfg.Tools.StackingTool.Fallbacks...)

	for _, tool := range tools {
		if status := tm.CheckTool(tool); status.Available {
			return tool, nil
		}
	}
	return "", fmt.Errorf("no available image stacking tools found")
}

// GetAvailableTimelapseTool returns the best available timelapse tool
func (tm *ToolManager) GetAvailableTimelapseTool() (string, error) {
	tools := []string{tm.cfg.Tools.TimelapseTool.Preferred}
	tools = append(tools, tm.cfg.Tools.TimelapseTool.Fallbacks...)

	for _, tool := range tools {
		if status := tm.CheckTool(tool); status.Available {
			return tool, nil
		}
	}
	return "", fmt.Errorf("no available timelapse tools found")
}

// GetAvailableAlignmentTool returns the best available alignment tool
func (tm *ToolManager) GetAvailableAlignmentTool() (string, error) {
	tools := []string{tm.cfg.Tools.AlignmentTool.Preferred}
	tools = append(tools, tm.cfg.Tools.AlignmentTool.Fallbacks...)

	for _, tool := range tools {
		if status := tm.CheckTool(tool); status.Available {
			return tool, nil
		}
	}
	return "", fmt.Errorf("no available alignment tools found")
}

// GetToolStatus returns comprehensive status of all configured tools
func (tm *ToolManager) GetToolStatus() map[string]map[string]ToolStatus {
	status := make(map[string]map[string]ToolStatus)

	// RAW processing tools
	rawTools := []string{tm.cfg.Tools.RAWProcessing.Preferred}
	rawTools = append(rawTools, tm.cfg.Tools.RAWProcessing.Fallbacks...)
	status["raw"] = make(map[string]ToolStatus)
	for _, tool := range rawTools {
		var checkName string
		switch tool {
		case "imagemagick":
			checkName = "convert"
		case "darktable":
			checkName = "darktable-cli"
		default:
			checkName = tool
		}
		status["raw"][tool] = tm.CheckTool(checkName)
	}

	// Panoramic tools
	panoTools := []string{tm.cfg.Tools.PanoramicTool.Preferred}
	panoTools = append(panoTools, tm.cfg.Tools.PanoramicTool.Fallbacks...)
	status["panoramic"] = make(map[string]ToolStatus)
	for _, tool := range panoTools {
		var checkName string
		switch tool {
		case "hugin":
			checkName = "hugin_executor"
		case "imagemagick":
			checkName = "convert"
		default:
			checkName = tool
		}
		status["panoramic"][tool] = tm.CheckTool(checkName)
	}

	// Stacking tools
	stackTools := []string{tm.cfg.Tools.StackingTool.Preferred}
	stackTools = append(stackTools, tm.cfg.Tools.StackingTool.Fallbacks...)
	status["stacking"] = make(map[string]ToolStatus)
	for _, tool := range stackTools {
		status["stacking"][tool] = tm.CheckTool(tool)
	}

	// Timelapse tools
	tlTools := []string{tm.cfg.Tools.TimelapseTool.Preferred}
	tlTools = append(tlTools, tm.cfg.Tools.TimelapseTool.Fallbacks...)
	status["timelapse"] = make(map[string]ToolStatus)
	for _, tool := range tlTools {
		status["timelapse"][tool] = tm.CheckTool(tool)
	}

	// Alignment tools
	alignTools := []string{tm.cfg.Tools.AlignmentTool.Preferred}
	alignTools = append(alignTools, tm.cfg.Tools.AlignmentTool.Fallbacks...)
	status["alignment"] = make(map[string]ToolStatus)
	for _, tool := range alignTools {
		status["alignment"][tool] = tm.CheckTool(tool)
	}

	return status
}

// extractVersion extracts version information from tool output
func extractVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "version") || strings.Contains(line, "Version") {
			return line
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return "unknown"
}

// InstallTool attempts to install a missing tool (for future auto-dependency system)
func (tm *ToolManager) InstallTool(toolName string) error {
	// Future implementation: check package manager, download binaries, etc.
	return fmt.Errorf("automatic tool installation not yet implemented for %s", toolName)
}
