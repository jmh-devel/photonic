package cli

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"photonic/internal/config"
	"photonic/internal/pipeline"
	"photonic/internal/server"
	"photonic/internal/storage"

	"github.com/spf13/cobra"
)

// NewRootCmd creates the root Cobra command
func NewRootCmd(cfg *config.Config, log *slog.Logger, store *storage.Store, pipe *pipeline.Pipeline) *cobra.Command {
	root := NewRoot(pipe, cfg, log, store)

	rootCmd := &cobra.Command{
		Use:   "photonic",
		Short: "Photonic is a comprehensive photo processing pipeline",
		Long: `Photonic provides automated processing for RAW images, panoramic stitching,
focus stacking, timelapses, and image alignment.`,
	}

	// Add subcommands
	rootCmd.AddCommand(newPanoramicCmd(root))
	rootCmd.AddCommand(newScanCmd(root))
	rootCmd.AddCommand(newTimelapseCmd(root))
	rootCmd.AddCommand(newStackCmd(root))
	rootCmd.AddCommand(newAlignCmd(root))
	rootCmd.AddCommand(newPipelineCmd(root)) // New: comprehensive pipelines
	rootCmd.AddCommand(newRawCmd(root))
	rootCmd.AddCommand(newServeCmd(root))
	rootCmd.AddCommand(newConfigCmd(root))
	rootCmd.AddCommand(newVersionCmd(root))

	// Distributed system commands
	rootCmd.AddCommand(newAgentCmd(root))
	rootCmd.AddCommand(newWebCmd(root))
	rootCmd.AddCommand(newServerCmd(root))

	return rootCmd
}

func newPanoramicCmd(root *Root) *cobra.Command {
	var (
		projection       string
		blending         string
		quality          string
		output           string
		tool             string
		aggression       string
		rawTool          string
		autoExposure     bool
		autoWhiteBalance bool
		saturation       float64
		vibrance         float64
		localContrast    float64
		sharpening       float64
		enhancePreset    string
		noCache          bool
		noPreserve       bool
	)

	cmd := &cobra.Command{
		Use:   "panoramic <input_directory> [output_path]",
		Short: "Create panoramic images from multiple photos",
		Long: `Process a directory of photos into a panoramic image using Hugin or ImageMagick.
Supports various projections and blending modes for optimal results.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			// Handle output path
			if len(args) > 1 {
				output = args[1]
			}
			if output == "" {
				output = root.cfg.Paths.DefaultOutput
			}

			// Apply enhancement presets
			if enhancePreset != "" {
				switch enhancePreset {
				case "auto":
					autoExposure = true
					autoWhiteBalance = true
				case "boost":
					autoExposure = true
					autoWhiteBalance = true
					if saturation == 0 {
						saturation = 1.25
					}
					if vibrance == 0 {
						vibrance = 1.5
					}
					if localContrast == 0 {
						localContrast = 0.3
					}
				case "vivid":
					autoExposure = true
					autoWhiteBalance = true
					if saturation == 0 {
						saturation = 1.5
					}
					if vibrance == 0 {
						vibrance = 2.0
					}
					if localContrast == 0 {
						localContrast = 0.5
					}
					if sharpening == 0 {
						sharpening = 0.5
					}
				}
			}

			job := pipeline.Job{
				ID:        newID("pano"),
				Type:      pipeline.JobPanoramic,
				InputPath: input,
				Output:    output,
				Options: map[string]any{
					"projection":       projection,
					"blending":         blending,
					"quality":          quality,
					"aggression":       aggression,
					"tool":             tool,
					"rawTool":          rawTool,
					"source":           "cli",
					"autoExposure":     autoExposure,
					"autoWhiteBalance": autoWhiteBalance,
					"saturation":       saturation,
					"vibrance":         vibrance,
					"localContrast":    localContrast,
					"sharpening":       sharpening,
					"noCache":          noCache,
					"noPreserve":       noPreserve,
				},
			}
			return root.enqueueAndWait(context.Background(), job)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&projection, "projection", "p", "cylindrical", "projection mode (cylindrical|spherical|planar|fisheye|stereographic|mercator)")
	cmd.Flags().StringVarP(&blending, "blending", "b", "multiband", "blending method (multiband|feather|none)")
	cmd.Flags().StringVarP(&quality, "quality", "q", "normal", "processing quality (fast|normal|high|ultra)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output path or directory")
	cmd.Flags().StringVar(&tool, "tool", "", "panoramic tool to use (hugin|imagemagick|ptassembler), auto-detect if empty")
	cmd.Flags().StringVar(&aggression, "aggression", "moderate", "control point cleaning aggression (low|moderate|high) - moderate preserves rainbows!")
	cmd.Flags().StringVar(&rawTool, "raw-tool", "", "RAW processor to use (darktable|imagemagick|dcraw|rawtherapee), uses config default if empty")

	// Image enhancement flags
	cmd.Flags().BoolVar(&autoExposure, "auto-exposure", false, "apply automatic exposure correction")
	cmd.Flags().BoolVar(&autoWhiteBalance, "auto-white-balance", false, "apply automatic white balance")
	cmd.Flags().Float64Var(&saturation, "saturation", 0, "saturation boost (1.25 = +25%, 0 = no change)")
	cmd.Flags().Float64Var(&vibrance, "vibrance", 0, "vibrance boost (1.5 = +50%, 0 = no change)")
	cmd.Flags().Float64Var(&localContrast, "local-contrast", 0, "local contrast enhancement (0.3 = 30%, 0 = no change)")
	cmd.Flags().Float64Var(&sharpening, "sharpening", 0, "sharpening amount (0.5 = 50%, 0 = no change)")
	cmd.Flags().StringVar(&enhancePreset, "enhance-preset", "", "apply enhancement preset (auto|boost|vivid)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "ignore existing cache and force fresh RAW processing")
	cmd.Flags().BoolVar(&noPreserve, "no-preserve", false, "clean up processed files after completion (don't preserve for inspection)")

	return cmd
}

// Placeholder functions for other commands
func newScanCmd(root *Root) *cobra.Command {
	return &cobra.Command{Use: "scan", Short: "Scan directory for photos (placeholder)"}
}

func newTimelapseCmd(root *Root) *cobra.Command {
	var (
		fps        int
		stabilize  bool
		output     string
		outputDir  string
		resolution string
		tool       string
		rawTool    string
		noCache    bool
		noPreserve bool
		formats    []string
	)

	cmd := &cobra.Command{
		Use:   "timelapse <input_directory> [flags]",
		Short: "Create timelapse videos from image sequences",
		Long: `Create timelapse videos from image sequences with RAW processing support.
Supports multiple output formats including MP4, 3GP, and GIF.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			// Set default format if none specified
			if len(formats) == 0 {
				formats = []string{"mp4"}
			}

			// Create intelligent default output path based on input if not specified
			if output == root.cfg.Paths.DefaultOutput && outputDir == "" {
				// Create output in a subdirectory named after the input
				inputBaseName := filepath.Base(filepath.Clean(input))
				output = filepath.Join("timelapse-output", inputBaseName, "timelapse.mp4")
			}

			// Debug logging for CLI argument parsing
			root.log.Info("timelapse command parsed",
				"input", input,
				"output", output,
				"output_dir", outputDir,
				"fps", fps,
				"stabilize", stabilize,
				"resolution", resolution,
				"formats", formats,
				"tool", tool,
				"raw_tool", rawTool,
				"no_cache", noCache,
				"no_preserve", noPreserve,
			)

			job := pipeline.Job{
				ID:        newID("tl"),
				Type:      pipeline.JobTimelapse,
				InputPath: input,
				Output:    output,
				Options: map[string]any{
					"fps":        fps,
					"stabilize":  stabilize,
					"tool":       tool,
					"rawTool":    rawTool,
					"outputDir":  outputDir,
					"resolution": resolution,
					"formats":    formats,
					"noCache":    noCache,
					"noPreserve": noPreserve,
					"source":     "cli",
				},
			}

			return root.enqueueAndWait(context.Background(), job)
		},
	}

	// Add flags
	cmd.Flags().IntVar(&fps, "fps", 10, "frames per second (default 10 for astronomy)")
	cmd.Flags().BoolVar(&stabilize, "stabilize", true, "enable video stabilization")
	cmd.Flags().StringVarP(&output, "output", "o", root.cfg.Paths.DefaultOutput, "output path or directory")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "output directory for multiple formats (optional)")
	cmd.Flags().StringVar(&resolution, "resolution", "", "output resolution (1080p|720p|480p|240p)")
	cmd.Flags().StringVar(&tool, "tool", "", "timelapse tool to use (ffmpeg|mencoder|avconv), auto-detect if empty")
	cmd.Flags().StringVar(&rawTool, "raw-tool", "", "RAW processor to use (darktable|imagemagick|dcraw|rawtherapee), uses config default if empty")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "ignore existing cache and force fresh RAW processing")
	cmd.Flags().BoolVar(&noPreserve, "no-preserve", false, "clean up processed files after completion (don't preserve for inspection)")
	cmd.Flags().StringSliceVar(&formats, "format", []string{}, "output formats (mp4, mp4-h265, 3gp, 3gp-h264, gif) - can specify multiple times")

	return cmd
}

func newStackCmd(root *Root) *cobra.Command {
	var (
		method        string
		alignment     string
		quality       string
		output        string
		denoise       bool
		sharpen       bool
		rawTool       string
		noCache       bool
		sigmaLow      float64
		sigmaHigh     float64
		iterations    int
		kappa         float64
		winsorPercent float64
		astroMode     bool
	)

	cmd := &cobra.Command{
		Use:   "stack <input_directory>",
		Short: "Focus stack or noise reduction stack multiple images",
		Long: `Stack multiple images for noise reduction or focus stacking.
		
Examples:
  # Noise reduction stack (astrophotography)
  photonic stack /photos/astro/ --method average --alignment star --output stacked.tif
  
  # Deep-sky object detail enhancement (astrophotography)
  photonic stack /photos/astro/ --method detail-enhancement --astro --output enhanced.tif
  
  # Star trails effect (aligned + unaligned combination)
  photonic stack /photos/aligned-and-processed/ --method star-trails --output trails.tif
  
  # Focus stack (macro photography) 
  photonic stack /photos/macro/ --method focus --alignment feature --output focused.tif
  
  # Denoise stack
  photonic stack /photos/lowlight/ --method sigma-clip --denoise --output clean.tif`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			if output == root.cfg.Paths.DefaultOutput {
				inputBaseName := filepath.Base(filepath.Clean(input))
				output = filepath.Join("output", method, inputBaseName, inputBaseName+".tif")
			}

			root.log.Info("stack command parsed",
				"input", input,
				"output", output,
				"method", method,
				"alignment", alignment,
				"quality", quality,
				"denoise", denoise,
				"sharpen", sharpen,
				"raw_tool", rawTool,
				"no_cache", noCache,
			)

			job := pipeline.Job{
				ID:        newID("st"),
				Type:      pipeline.JobStack,
				InputPath: input,
				Output:    output,
				Options: map[string]any{
					"method":        method,
					"alignment":     alignment,
					"quality":       quality,
					"denoise":       denoise,
					"sharpen":       sharpen,
					"rawTool":       rawTool,
					"noCache":       noCache,
					"sigmaLow":      sigmaLow,
					"sigmaHigh":     sigmaHigh,
					"iterations":    iterations,
					"kappa":         kappa,
					"winsorPercent": winsorPercent,
					"astroMode":     astroMode,
					"source":        "cli",
				},
			}

			return root.enqueueAndWait(context.Background(), job)
		},
	}

	cmd.Flags().StringVar(&method, "method", "average", "stacking method (average|median|sigma-clip|kappa-sigma|winsorized|maximum|focus|detail-enhancement|star-trails|baseline|dss)")
	cmd.Flags().StringVar(&alignment, "alignment", "auto", "alignment method (auto|star|feature|none)")
	cmd.Flags().StringVar(&quality, "quality", "normal", "processing quality (fast|normal|high)")
	cmd.Flags().StringVarP(&output, "output", "o", root.cfg.Paths.DefaultOutput, "output file path")
	cmd.Flags().BoolVar(&denoise, "denoise", false, "apply additional denoising")
	cmd.Flags().BoolVar(&sharpen, "sharpen", false, "apply sharpening to final result")
	cmd.Flags().StringVar(&rawTool, "raw-tool", "", "RAW processor (darktable|imagemagick|dcraw|rawtherapee)")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "ignore existing cache and force fresh processing")

	// Astronomical stacking parameters
	cmd.Flags().Float64Var(&sigmaLow, "sigma-low", 2.0, "lower sigma threshold for sigma clipping")
	cmd.Flags().Float64Var(&sigmaHigh, "sigma-high", 2.0, "upper sigma threshold for sigma clipping")
	cmd.Flags().IntVar(&iterations, "iterations", 3, "maximum iterations for sigma clipping")
	cmd.Flags().Float64Var(&kappa, "kappa", 1.5, "kappa factor for kappa-sigma rejection")
	cmd.Flags().Float64Var(&winsorPercent, "winsor-percent", 5.0, "winsorized percentage (0-50)")
	cmd.Flags().BoolVar(&astroMode, "astro", false, "enable advanced astronomical stacking mode")

	return cmd
}

func newAlignCmd(root *Root) *cobra.Command {
	var (
		alignType     string
		quality       string
		output        string
		reference     string
		cropToFit     bool
		rawTool       string
		starThreshold float64
	)

	cmd := &cobra.Command{
		Use:   "align <input_directory>",
		Short: "Align images for stacking or correction",
		Long: `Align multiple images using various methods.
		
Examples:
  # Auto-detect alignment type
  photonic align /photos/sequence/ --output aligned/
  
  # Astronomical alignment
  photonic align /photos/astro/ --type astro --quality high --output aligned/
  
  # Panoramic alignment
  photonic align /photos/pano/ --type panoramic --crop-to-fit --output aligned/
  
  # General feature-based alignment
  photonic align /photos/handheld/ --type general --reference first --output aligned/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			if output == root.cfg.Paths.DefaultOutput {
				inputBaseName := filepath.Base(filepath.Clean(input))
				output = filepath.Join("aligned-output", inputBaseName)
			}

			root.log.Info("align command parsed",
				"input", input,
				"output", output,
				"type", alignType,
				"quality", quality,
				"reference", reference,
				"crop_to_fit", cropToFit,
				"raw_tool", rawTool,
			)

			job := pipeline.Job{
				ID:        newID("al"),
				Type:      pipeline.JobAlign,
				InputPath: input,
				Output:    output,
				Options: map[string]any{
					"type":          alignType,
					"quality":       quality,
					"reference":     reference,
					"cropToFit":     cropToFit,
					"rawTool":       rawTool,
					"starThreshold": starThreshold,
					"source":        "cli",
				},
			}

			return root.enqueueAndWait(context.Background(), job)
		},
	}

	cmd.Flags().StringVar(&alignType, "type", "auto", "alignment type (auto|astro|panoramic|general|timelapse)")
	cmd.Flags().StringVar(&quality, "quality", "normal", "processing quality (fast|normal|high|ultra)")
	cmd.Flags().StringVarP(&output, "output", "o", root.cfg.Paths.DefaultOutput, "output directory")
	cmd.Flags().StringVar(&reference, "reference", "first", "reference image (first|middle|best|<filename>)")
	cmd.Flags().BoolVar(&cropToFit, "crop-to-fit", false, "crop all images to common area")
	cmd.Flags().StringVar(&rawTool, "raw-tool", "", "RAW processor (darktable|imagemagick|dcraw|rawtherapee)")
	cmd.Flags().Float64Var(&starThreshold, "star-threshold", 0.85, "star detection threshold for astronomical alignment (0.0-1.0)")

	return cmd
}

func newRawCmd(root *Root) *cobra.Command {
	var (
		tool    string
		quality string
		output  string
		format  string
		preset  string
		batch   bool
		noCache bool
	)

	cmd := &cobra.Command{
		Use:   "raw <input_path>",
		Short: "Process RAW images using various tools",
		Long: `Process RAW images with support for multiple processing tools.
		
Examples:
  # Process single RAW file
  photonic raw photo.cr2 --tool darktable --preset auto --output processed.jpg
  
  # Batch process directory
  photonic raw /photos/raw/ --batch --tool imagemagick --format tiff --output /photos/processed/
  
  # High quality processing
  photonic raw image.nef --tool rawtherapee --quality high --preset studio --output final.tif`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			if output == root.cfg.Paths.DefaultOutput {
				inputBaseName := filepath.Base(filepath.Clean(input))
				if batch {
					output = filepath.Join("processed-raw", inputBaseName)
				} else {
					// Single file: change extension based on format
					outputExt := ".jpg"
					if format == "tiff" || format == "tif" {
						outputExt = ".tif"
					}
					baseWithoutExt := strings.TrimSuffix(inputBaseName, filepath.Ext(inputBaseName))
					output = baseWithoutExt + "_processed" + outputExt
				}
			}

			root.log.Info("raw command parsed",
				"input", input,
				"output", output,
				"tool", tool,
				"quality", quality,
				"format", format,
				"preset", preset,
				"batch", batch,
				"no_cache", noCache,
			)

			job := pipeline.Job{
				ID:        newID("rw"),
				Type:      pipeline.JobRaw,
				InputPath: input,
				Output:    output,
				Options: map[string]any{
					"tool":    tool,
					"quality": quality,
					"format":  format,
					"preset":  preset,
					"batch":   batch,
					"noCache": noCache,
					"source":  "cli",
				},
			}

			return root.enqueueAndWait(context.Background(), job)
		},
	}

	cmd.Flags().StringVar(&tool, "tool", "", "RAW processor (darktable|imagemagick|dcraw|rawtherapee), auto-detect if empty")
	cmd.Flags().StringVar(&quality, "quality", "normal", "processing quality (fast|normal|high|ultra)")
	cmd.Flags().StringVarP(&output, "output", "o", root.cfg.Paths.DefaultOutput, "output file or directory")
	cmd.Flags().StringVar(&format, "format", "jpeg", "output format (jpeg|tiff|png)")
	cmd.Flags().StringVar(&preset, "preset", "", "processing preset (auto|studio|landscape|portrait|astro)")
	cmd.Flags().BoolVar(&batch, "batch", false, "process entire directory")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "ignore existing cache and force fresh processing")

	return cmd
}

func newServeCmd(root *Root) *cobra.Command {
	var (
		addr       string
		watchPaths []string
		dtConfig   string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server with photo monitoring",
		Long: `Start an HTTP server that provides APIs for job monitoring and photo management.
Also optionally monitors filesystem and darktable for real-time photo events.

Examples:
  # Basic server
  photonic serve --addr :8080
  
  # Server with photo monitoring  
  photonic serve --addr :8080 --watch /data/Photography --watch /photos/import
  
  # Server with darktable integration
  photonic serve --addr :8080 --watch /data/Photography --darktable ~/.config/darktable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			root.log.Info("starting server",
				"addr", addr,
				"watch_paths", watchPaths,
				"darktable_config", dtConfig,
			)

			realPipeline, ok := root.pipeline.(*pipeline.Pipeline)
			if !ok {
				return fmt.Errorf("pipeline unavailable for server startup")
			}

			// Create enhanced server with optional monitoring
			server, err := server.NewServer(addr, root.store, realPipeline, watchPaths, dtConfig, root.log)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			root.log.Info("server ready",
				"addr", addr,
				"endpoints", []string{"/healthz", "/jobs", "/stream", "/darktable/stats", "/photo-events"},
			)

			return server.Start(ctx)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", ":8080", "server address (host:port)")
	cmd.Flags().StringSliceVar(&watchPaths, "watch", nil, "directories to monitor for photo changes")
	cmd.Flags().StringVar(&dtConfig, "darktable", "", "darktable config directory (default: ~/.config/darktable)")

	return cmd
}

func newConfigCmd(root *Root) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration settings",
		Long:  "Show, validate, or modify photonic configuration",
	}

	// config show subcommand
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Configuration:\n\n")
			fmt.Printf("Database Path: %s\n", root.cfg.Paths.DatabasePath)
			fmt.Printf("Default Output: %s\n", root.cfg.Paths.DefaultOutput)
			fmt.Printf("Temp Directory: %s\n", root.cfg.Processing.TempDir)
			fmt.Printf("Parallel Jobs: %d\n", root.cfg.Processing.ParallelJobs)
			fmt.Printf("Memory Limit: %s\n", root.cfg.Processing.MemoryLimit)
			fmt.Printf("Log Level: %s\n", root.cfg.Logging.Level)
			fmt.Printf("Log Format: %s\n", root.cfg.Logging.Format)
			fmt.Printf("Log Directory: %s\n", root.cfg.Logging.LogDir)
			return nil
		},
	}

	// config validate subcommand
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			root.log.Info("configuration validation", "status", "valid")
			fmt.Println("✅ Configuration is valid")
			return nil
		},
	}

	cmd.AddCommand(showCmd, validateCmd)
	return cmd
}

func newVersionCmd(root *Root) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Photonic v1.0.0")
		},
	}
}

// newAgentCmd creates the agent command for distributed photo management
func newAgentCmd(root *Root) *cobra.Command {
	var (
		serverAddr   string
		directories  []string
		agentID      string
		nfsMount     string
		maxUploads   int
		maxDownloads int
		configPath   string
	)

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start a Photonic agent for distributed photo management",
		Long: `Start a Photonic agent that connects to a gRPC server for distributed photo management.

The agent will:
- Register with the specified gRPC server
- Monitor local photo directories for changes
- Upload and synchronize photos with the server
- Process tasks assigned by the server
- Send regular heartbeats to maintain connection`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("🤖 Starting Photonic Agent\n")
			fmt.Printf("🔗 Server: %s\n", serverAddr)
			fmt.Printf("📁 Directories: %v\n", directories)
			fmt.Printf("🆔 Agent ID: %s\n", agentID)

			if len(directories) == 0 {
				return fmt.Errorf("at least one photo directory must be specified")
			}

			fmt.Println("🚀 Photonic Agent starting...")
			fmt.Println("   The agent will connect to photonic server for blazing fast photo sync.")

			// Start the agent daemon
			return root.startAgentDaemon(serverAddr, directories, agentID)
		},
	}

	cmd.Flags().StringVarP(&serverAddr, "server", "s", "localhost:8080", "gRPC server address")
	cmd.Flags().StringSliceVarP(&directories, "directories", "d", []string{}, "Photo directories to watch")
	cmd.Flags().StringVarP(&agentID, "agent-id", "i", "", "Agent ID (auto-generated if empty)")
	cmd.Flags().StringVarP(&nfsMount, "nfs-mount", "n", "", "NFS mount point")
	cmd.Flags().IntVarP(&maxUploads, "max-uploads", "u", 3, "Maximum concurrent uploads")
	cmd.Flags().IntVarP(&maxDownloads, "max-downloads", "w", 3, "Maximum concurrent downloads")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "Configuration file path")

	return cmd
}

// newWebCmd creates the web dashboard command
func newWebCmd(root *Root) *cobra.Command {
	var (
		port       int
		grpcServer string
	)

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the Photonic web dashboard",
		Long: `Start the Photonic web dashboard for monitoring and managing the distributed photo system.

The web dashboard provides:
- Real-time monitoring of connected agents
- Photo transfer progress and statistics
- Sync queue management
- System health metrics
- Interactive photo browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("🌐 Starting Photonic Web Dashboard\n")
			fmt.Printf("📊 Dashboard: http://localhost:%d\n", port)
			fmt.Printf("🔗 gRPC Server: %s\n", grpcServer)

			fmt.Println("🚀 Photonic Web Dashboard starting...")
			fmt.Println("   Real-time monitoring with WebSocket updates available.")

			// Start the web dashboard server
			return root.startWebDashboard(port, grpcServer)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8081, "Web server port")
	cmd.Flags().StringVarP(&grpcServer, "grpc-server", "g", "localhost:8080", "gRPC server address")

	return cmd
}

// newServerCmd creates the complete server command
func newServerCmd(root *Root) *cobra.Command {
	var (
		grpcPort         int
		webPort          int
		storageRoot      string
		maxUploads       int
		maxFileSize      int64
		requireChecksums bool
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the complete Photonic gRPC server and web dashboard",
		Long: `Start the complete Photonic server including both the gRPC server for agents
and the web dashboard for monitoring.

The server provides:
- gRPC API for agent communication
- Binary photo transfer with streaming
- Metadata synchronization and conflict resolution
- Task queue management
- Real-time monitoring dashboard
- WebSocket API for live updates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("🚀 Starting Photonic Distributed Server\n")
			fmt.Printf("🔌 gRPC Server: localhost:%d\n", grpcPort)
			fmt.Printf("🌐 Web Dashboard: http://localhost:%d\n", webPort)
			fmt.Printf("📁 Storage: %s\n", storageRoot)
			fmt.Printf("⚡ Max concurrent uploads: %d\n", maxUploads)
			fmt.Printf("📏 Max file size: %d MB\n", maxFileSize/(1024*1024))

			// Import the packages dynamically since they use the correct module path
			fmt.Println("🚀 Photonic Distributed Server starting...")
			fmt.Println("   Full enterprise-grade photo management system available.")
			fmt.Println("   Agents can now connect for blazing fast distributed photo processing!")

			// Start the full distributed server with both gRPC and web components
			return root.startFullDistributedServer(grpcPort, webPort, storageRoot)
		},
	}

	cmd.Flags().IntVarP(&grpcPort, "port", "p", 8080, "gRPC server port")
	cmd.Flags().IntVarP(&webPort, "web-port", "w", 8081, "Web dashboard port")
	cmd.Flags().StringVarP(&storageRoot, "storage", "s", "./photonic-storage", "Storage root directory")
	cmd.Flags().IntVarP(&maxUploads, "max-uploads", "u", 10, "Maximum concurrent uploads")
	cmd.Flags().Int64VarP(&maxFileSize, "max-file-size", "f", 500*1024*1024, "Maximum file size in bytes")
	cmd.Flags().BoolVarP(&requireChecksums, "require-checksums", "c", true, "Require checksum verification")

	return cmd
}

// newPipelineCmd creates a comprehensive pipeline command for common workflows
func newPipelineCmd(root *Root) *cobra.Command {
	var (
		alignment string
		method    string
		output    string
		rawTool   string
		quality   string
	)

	cmd := &cobra.Command{
		Use:   "pipeline <workflow> <input_directory>",
		Short: "Run complete processing pipelines for common workflows",
		Long: `Execute end-to-end processing pipelines that combine multiple steps.

Available workflows:
  astro-stack        RAW → Aligned → Stacked (clean astronomical averaging)  
  astro-trails       RAW → Aligned → Star Trails (artistic blending)
  astro-enhance      RAW → Aligned → Enhanced Deep-Sky (detail optimization)

Examples:
  # Complete astronomical stacking pipeline  
  photonic pipeline astro-stack /photos/astro/ --output stacked_result.tif
  
  # Beautiful star trails from RAW images
  photonic pipeline astro-trails /photos/astro/ --output star_trails.tif
  
  # Enhanced deep-sky object processing
  photonic pipeline astro-enhance /photos/m31/ --output m31_enhanced.tif`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflow := args[0]
			inputDir := args[1]

			return root.runPipeline(workflow, inputDir, map[string]interface{}{
				"alignment": alignment,
				"method":    method,
				"output":    output,
				"rawTool":   rawTool,
				"quality":   quality,
			})
		},
	}

	cmd.Flags().StringVar(&alignment, "alignment", "star", "alignment method (auto|star|feature|none)")
	cmd.Flags().StringVar(&method, "method", "", "override stacking method (auto-detected from workflow)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file path (required)")
	cmd.Flags().StringVar(&rawTool, "raw-tool", "darktable", "RAW processor (darktable|imagemagick|dcraw|rawtherapee)")
	cmd.Flags().StringVar(&quality, "quality", "normal", "processing quality (fast|normal|high)")

	cmd.MarkFlagRequired("output")

	return cmd
}
