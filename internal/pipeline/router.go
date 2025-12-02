package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"photonic/internal/config"
	"photonic/internal/fsutil"
	"photonic/internal/storage"
	"photonic/internal/tasks"
)

// router implements Processor and routes jobs to their concrete handlers.
type router struct {
	log         *slog.Logger
	store       *storage.Store
	alignMgr    alignmentManager
	rawMgr      *tasks.RawProcessorManager
	stackFn     stackFunc
	astroFac    astroStackerFactory
	mathStacker astroStacker
}

type alignmentManager interface {
	DetectAlignmentType(images []string) tasks.AlignmentType
	AlignWithTypeAndConfig(ctx context.Context, images []string, outputDir string, quality string, alignType tasks.AlignmentType, config map[string]any) (tasks.AlignmentResult, error)
}

type stackFunc func(ctx context.Context, req tasks.StackRequest) (tasks.StackResult, error)

type astroStacker interface {
	StackImages(ctx context.Context, req tasks.AstroStackRequest) (tasks.AstroStackResult, error)
}

type astroStackerFactory func() astroStacker

func newRouter(logger *slog.Logger, store *storage.Store, alignCfg *config.AlignmentConfig, rawCfg *config.RawProcessing) Processor {
	return &router{
		log:      logger,
		store:    store,
		alignMgr: tasks.NewAlignmentManager(alignCfg),
		rawMgr:   tasks.NewRawProcessorManager(rawCfg),
		stackFn:  tasks.StackImages,
		astroFac: func() astroStacker {
			// Smart stacker selection: enfuse for blending, ImageMagick for pure math
			enfuseStacker := tasks.NewAstroEnfuseStacker()
			if enfuseStacker.IsAvailable() {
				return enfuseStacker
			}
			// Fallback to ImageMagick if enfuse not available
			return tasks.NewAstroStacker()
		},
		// Keep ImageMagick stacker for mathematical operations
		mathStacker: tasks.NewAstroStacker(),
	}
}

func (r *router) Process(ctx context.Context, job Job) Result {
	switch job.Type {
	case JobScan:
		return r.handleScan(ctx, job)
	case JobTimelapse:
		return r.handleTimelapse(ctx, job)
	case JobPanoramic:
		return r.handlePanoramic(ctx, job)
	case JobStack:
		return r.handleStack(ctx, job)
	case JobAlign:
		return r.handleAlign(ctx, job)
	default:
		return Result{Job: job, Error: fmt.Errorf("unknown job type: %s", job.Type)}
	}
}

func (r *router) handleScan(ctx context.Context, job Job) Result {
	summary, err := tasks.Scan(job.InputPath)
	meta := map[string]any{
		"images": len(summary.Images),
		"groups": summary.Groups,
	}
	for _, g := range summary.Groups {
		if r.store != nil {
			_ = r.store.RecordGroup(storage.ImageGroupRecord{
				JobID:           job.ID,
				GroupType:       g.GroupType,
				DetectionMethod: g.Detection,
				BasePath:        g.BasePath,
				ImageCount:      g.Count,
			})
		}
	}
	return Result{Job: job, Error: err, Meta: meta}
}

func (r *router) handleTimelapse(ctx context.Context, job Job) Result {
	fps, _ := job.Options["fps"].(int)
	if fps == 0 {
		fps = 10 // Default to 10fps for astronomy
	}
	stabilize, _ := job.Options["stabilize"].(bool)
	outputDir, _ := job.Options["outputDir"].(string)
	resolution, _ := job.Options["resolution"].(string)
	formats, _ := job.Options["formats"].([]string)
	rawTool, _ := job.Options["rawTool"].(string)

	// Get cache control options
	ignoreCache := false
	if noCache, ok := job.Options["noCache"].(bool); ok {
		ignoreCache = noCache
	}

	preserveCache := true
	if noPreserve, ok := job.Options["noPreserve"].(bool); ok {
		preserveCache = !noPreserve
	}

	// Default to MP4 if no formats specified
	if len(formats) == 0 {
		formats = []string{"mp4"}
	}

	prepDir := job.InputPath
	var tempDir string
	var needsCleanup bool

	if r.rawMgr != nil {
		tmpDir, _, err := tasks.PreprocessDirectory(ctx, job.InputPath, r.rawMgr, r.log, r.store, rawTool, nil, ignoreCache)
		if err == nil && tmpDir != "" {
			prepDir = tmpDir
			tempDir = tmpDir
			needsCleanup = !preserveCache
		}
	}

	// Ensure cleanup happens after timelapse processing
	defer func() {
		if needsCleanup && tempDir != "" {
			r.log.Info("cleaning up temporary processing directory", "temp_dir", tempDir)
			os.RemoveAll(tempDir)
		} else if tempDir != "" {
			r.log.Info("preserving processed files for inspection", "temp_dir", tempDir, "hint", "use --no-preserve to clean up")
		}
	}()

	res, err := tasks.BuildTimelapse(ctx, tasks.TimelapseRequest{
		InputDir:   prepDir,
		Output:     job.Output,
		OutputDir:  outputDir,
		FPS:        fps,
		Stabilize:  stabilize,
		Formats:    formats,
		Resolution: resolution,
	})

	// Build metadata with multiple output files
	outputFiles := make([]map[string]any, len(res.OutputFiles))
	for i, file := range res.OutputFiles {
		outputFiles[i] = map[string]any{
			"path":   file.Path,
			"format": file.Format,
			"codec":  file.Codec,
			"size":   file.Size,
		}
	}

	meta := map[string]any{
		"outputFiles": outputFiles,
		"frameCount":  res.FrameCount,
		"usedFFmpeg":  res.UsedFFmpeg,
		"formats":     formats,
	}
	return Result{Job: job, Error: err, Meta: meta}
}

func (r *router) handlePanoramic(ctx context.Context, job Job) Result {
	projection, _ := job.Options["projection"].(string)
	if projection == "" {
		projection = "cylindrical"
	}
	blending, _ := job.Options["blending"].(string)
	if blending == "" {
		blending = "multiband"
	}
	quality, _ := job.Options["quality"].(string)
	if quality == "" {
		quality = "normal"
	}
	aggression, _ := job.Options["aggression"].(string)
	if aggression == "" {
		aggression = "moderate"
	}
	rawTool, _ := job.Options["rawTool"].(string)

	inputDir := job.InputPath
	var tempDir string
	var needsCleanup bool

	if r.rawMgr != nil {
		// Extract enhancement options from job
		enhancements := &tasks.EnhancementOptions{
			AutoExposure:     getBoolOption(job.Options, "autoExposure"),
			AutoWhiteBalance: getBoolOption(job.Options, "autoWhiteBalance"),
			Saturation:       getFloat64Option(job.Options, "saturation"),
			Vibrance:         getFloat64Option(job.Options, "vibrance"),
			LocalContrast:    getFloat64Option(job.Options, "localContrast"),
			Sharpening:       getFloat64Option(job.Options, "sharpening"),
		}

		// Extract cache control flags
		ignoreCache := getBoolOption(job.Options, "noCache")
		noPreserve := getBoolOption(job.Options, "noPreserve")

		tmpDir, _, err := tasks.PreprocessDirectory(ctx, job.InputPath, r.rawMgr, r.log, r.store, rawTool, enhancements, ignoreCache)
		if err == nil && tmpDir != "" {
			inputDir = tmpDir
			tempDir = tmpDir
			needsCleanup = noPreserve // Only clean up if noPreserve is true
		}
	}

	// Check if cache cleanup is disabled (default: preserve cache)
	noCache := getBoolOption(job.Options, "noCache")

	// Ensure cleanup happens after panoramic processing only if --no-cache is specified
	defer func() {
		if needsCleanup && tempDir != "" && noCache {
			r.log.Info("cleaning up temporary processing directory (--no-cache specified)", "temp_dir", tempDir)
			os.RemoveAll(tempDir)
		} else if needsCleanup && tempDir != "" {
			r.log.Info("preserving processed files for inspection", "temp_dir", tempDir, "hint", "use --no-cache to clean up")
		}
	}()

	res, err := tasks.AssemblePanoramic(ctx, tasks.PanoramicRequest{
		InputDir:   inputDir,
		Output:     job.Output,
		Projection: projection,
		Blending:   blending,
		Quality:    quality,
		Aggression: aggression,
	})
	meta := map[string]any{
		"output":        res.OutputFile,
		"projection":    res.Projection,
		"blending":      res.Blending,
		"quality":       res.Quality,
		"aggression":    res.Aggression,
		"imageCount":    res.ImageCount,
		"toolUsed":      res.ToolUsed,
		"processedWith": res.ProcessedWith,
		"dimensions":    res.Dimensions,
	}
	return Result{Job: job, Error: err, Meta: meta}
}

func (r *router) handleStack(ctx context.Context, job Job) Result {
	method, _ := job.Options["method"].(string)
	if method == "" {
		method = "average"
	}
	rawTool, _ := job.Options["rawTool"].(string)

	// Check if astronomical stacking is requested
	astroMode, _ := job.Options["astroMode"].(bool)

	// Extract astronomical stacking parameters
	sigmaLow, _ := job.Options["sigmaLow"].(float64)
	sigmaHigh, _ := job.Options["sigmaHigh"].(float64)
	iterations, _ := job.Options["iterations"].(int)
	kappa, _ := job.Options["kappa"].(float64)
	winsorPercent, _ := job.Options["winsorPercent"].(float64)

	// Extract cache control flags
	ignoreCache := getBoolOption(job.Options, "noCache")

	inputDir := job.InputPath
	if r.rawMgr != nil {
		if tmp, _, err := tasks.PreprocessDirectory(ctx, job.InputPath, r.rawMgr, r.log, r.store, rawTool, nil, ignoreCache); err == nil && tmp != "" {
			inputDir = tmp
		}
	}

	// Use astronomical stacking for advanced methods
	if astroMode || method == "sigma-clip" || method == "kappa-sigma" || method == "winsorized" {
		var astroStacker astroStacker

		// Smart routing based on research and what actually works
		if method == "star-trails" {
			// Use enfuse for star-trails (blending desired)
			astroStacker = r.astroFac()
		} else if method == "average" || method == "mean" {
			// For average/mean with --astro, use simple stacking (works!)
			// Bypass complex sigma clipping that causes TV static
			res, err := r.stackFn(ctx, tasks.StackRequest{
				InputDir: inputDir,
				Output:   job.Output,
				Method:   "astro", // This maps to clean mathematical averaging
			})
			meta := map[string]any{
				"output": res.OutputFile,
				"method": "astro-" + method,
			}
			return Result{Job: job, Error: err, Meta: meta}
		} else {
			// Use mathematical stacking for other advanced methods
			astroStacker = r.mathStacker
		}

		astroRes, err := astroStacker.StackImages(ctx, tasks.AstroStackRequest{
			InputDir:      inputDir,
			Output:        job.Output,
			Method:        method,
			SigmaLow:      sigmaLow,
			SigmaHigh:     sigmaHigh,
			Iterations:    iterations,
			KappaFactor:   kappa,
			WinsorPercent: winsorPercent,
		})

		meta := map[string]any{
			"output":         astroRes.OutputFile,
			"method":         astroRes.Method,
			"imageCount":     astroRes.ImageCount,
			"rejectedPixels": astroRes.RejectedPixels,
			"processingTime": astroRes.ProcessingTime.String(),
			"cosmicRayCount": astroRes.CosmicRayCount,
			"signalToNoise":  astroRes.SignalToNoise,
		}
		return Result{Job: job, Error: err, Meta: meta}
	}

	// Use standard stacking for basic methods
	res, err := r.stackFn(ctx, tasks.StackRequest{
		InputDir: inputDir,
		Output:   job.Output,
		Method:   method,
	})
	meta := map[string]any{
		"output": res.OutputFile,
		"method": res.Method,
	}
	return Result{Job: job, Error: err, Meta: meta}
}

func (r *router) handleAlign(ctx context.Context, job Job) Result {
	manager := r.alignMgr
	if manager == nil {
		manager = tasks.NewAlignmentManager(&config.AlignmentConfig{})
	}
	images := job.Options["images"]
	imgList, _ := images.([]string)
	if len(imgList) == 0 && job.InputPath != "" {
		// Scan directory for images if no explicit list provided
		if files, err := fsutil.ListImages(job.InputPath); err == nil {
			imgList = files
		} else {
			// Fallback to treating InputPath as single file
			imgList = []string{job.InputPath}
		}
	}
	quality, _ := job.Options["quality"].(string)
	if quality == "" {
		quality = "normal"
	}

	// Get star threshold parameter
	starThreshold, _ := job.Options["starThreshold"].(float64)
	if starThreshold <= 0 {
		starThreshold = 0.85 // default threshold
	}

	// Get explicit alignment type from job options instead of auto-detecting
	alignTypeStr, _ := job.Options["type"].(string)
	if alignTypeStr == "" {
		alignTypeStr, _ = job.Options["atype"].(string)
	}
	var alignType tasks.AlignmentType
	switch alignTypeStr {
	case "astro":
		alignType = tasks.AlignmentAstro
	case "panoramic":
		alignType = tasks.AlignmentPanoramic
	case "general":
		alignType = tasks.AlignmentGeneral
	case "timelapse":
		alignType = tasks.AlignmentTimelapse
	default:
		// Only use auto-detection if type is "auto" or unspecified
		alignType = manager.DetectAlignmentType(imgList)
	}

	// Create config with star threshold for astronomical alignment
	alignConfig := map[string]any{
		"starThreshold": starThreshold,
	}

	res, err := manager.AlignWithTypeAndConfig(ctx, imgList, job.Output, quality, alignType, alignConfig)
	meta := map[string]any{
		"tool":    res.ToolUsed,
		"success": res.Success,
		"warn":    res.Warnings,
	}
	return Result{Job: job, Error: err, Meta: meta}
}

// Helper functions to safely extract typed options from job.Options map
func getBoolOption(options map[string]any, key string) bool {
	if val, ok := options[key].(bool); ok {
		return val
	}
	return false
}

func getFloat64Option(options map[string]any, key string) float64 {
	if val, ok := options[key].(float64); ok {
		return val
	}
	return 0.0
}
