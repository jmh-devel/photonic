package tasks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"photonic/internal/fsutil"
	"photonic/internal/storage"
)

type storageWriter interface {
	RecordImageMetadata(storage.ImageMetadata) error
}

// EnhancementOptions holds image enhancement settings
type EnhancementOptions struct {
	AutoExposure     bool
	AutoWhiteBalance bool
	Saturation       float64
	Vibrance         float64
	LocalContrast    float64
	Sharpening       float64
}

// PreprocessDirectory converts RAW files to working images using the best available processor.
// Returns a temp directory (if conversions were done) and conversion results.
// Implements JPG caching to avoid reprocessing existing converted files.
// preferredTool can be used to override the default tool selection (e.g., "darktable", "imagemagick")
// enhancements can be used to apply image processing enhancements during RAW conversion
// ignoreCache when true forces fresh processing even if cache files exist
func PreprocessDirectory(ctx context.Context, inputDir string, mgr *RawProcessorManager, log *slog.Logger, store storageWriter, preferredTool string, enhancements *EnhancementOptions, ignoreCache bool) (string, []RawConvertResult, error) {
	files, err := fsutil.ListImages(inputDir)
	if err != nil {
		return "", nil, err
	}
	var rawFiles []string
	for _, f := range files {
		ext := filepath.Ext(f)
		if isRawExt(ext) {
			rawFiles = append(rawFiles, f)
		}
	}
	if len(rawFiles) == 0 {
		return "", nil, nil
	}

	if mgr == nil {
		return "", nil, fmt.Errorf("no raw processor manager available")
	}

	// Create cache directory structure
	cacheDir := filepath.Join(inputDir, "processed")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("failed to create cache directory: %v", err)
	}

	// Check if all RAW files have cached JPGs (unless ignoring cache)
	var filesToProcess []string
	var cachedFiles []string

	for _, rawFile := range rawFiles {
		baseName := strings.TrimSuffix(filepath.Base(rawFile), filepath.Ext(rawFile))
		cachedJPG := filepath.Join(cacheDir, baseName+".jpg")

		// If ignoreCache is true, process all files fresh
		if ignoreCache {
			filesToProcess = append(filesToProcess, rawFile)
			continue
		}

		if fileExistsInCache(cachedJPG) {
			// Check if cached file is newer than RAW file
			if isCacheValid(rawFile, cachedJPG) {
				cachedFiles = append(cachedFiles, cachedJPG)
				continue
			}
		}
		filesToProcess = append(filesToProcess, rawFile)
	}

	// Use cache directory if we have cached files, otherwise use temp directory
	var workDir string
	var useCache bool

	if len(cachedFiles) > 0 && len(filesToProcess) == 0 {
		// All files are cached, use cache directory directly
		workDir = cacheDir
		useCache = true
	} else {
		// Some files need processing, use temp directory and copy cached files
		workDir = filepath.Join("/tmp", "photonic-preprocessed-"+fmt.Sprint(os.Getpid()))
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return "", nil, err
		}

		// Copy cached files to temp directory
		for _, cachedFile := range cachedFiles {
			destFile := filepath.Join(workDir, filepath.Base(cachedFile))
			if err := copyFile(cachedFile, destFile); err != nil {
				if log != nil {
					log.Warn("failed to copy cached file", "cached", cachedFile, "error", err)
				}
			}
		}
	}

	if log != nil {
		log.Info("starting RAW file preprocessing",
			"total_files", len(rawFiles),
			"cached_files", len(cachedFiles),
			"files_to_process", len(filesToProcess),
			"input_dir", inputDir,
			"cache_dir", cacheDir,
			"work_dir", workDir,
			"using_cache", useCache,
		)
	}

	var results []RawConvertResult

	// Process files that need conversion
	for i, f := range filesToProcess {
		if log != nil {
			log.Info("processing RAW file",
				"file", filepath.Base(f),
				"progress", fmt.Sprintf("%d/%d", i+1, len(filesToProcess)),
				"percent", fmt.Sprintf("%.1f%%", float64(i+1)/float64(len(filesToProcess))*100),
			)
		}

		var res RawConvertResult
		var err error

		if preferredTool != "" {
			// Use specific tool if requested
			proc := mgr.GetProcessor(preferredTool)
			if proc != nil && proc.IsAvailable() {
				// Apply enhancements to processor configuration if available
				if enhancements != nil {
					switch p := proc.(type) {
					case *ImageMagickProcessor:
						// Apply enhancements to ImageMagick config
						if enhancements.AutoWhiteBalance {
							p.config.AutoWhiteBalance = true
						}
						if enhancements.Saturation > 0 {
							p.config.Saturation = enhancements.Saturation
						}
						if enhancements.Vibrance > 0 {
							p.config.Vibrance = enhancements.Vibrance
						}
						if enhancements.LocalContrast > 0 {
							p.config.LocalContrast = enhancements.LocalContrast
						}
						if enhancements.Sharpening > 0 {
							p.config.Sharpen = fmt.Sprintf("%.1f", enhancements.Sharpening)
							p.config.UnsharpMask = "enabled"
						}
					case *DarktableProcessor:
						// Apply enhancements to Darktable config
						if enhancements.AutoExposure {
							p.config.AutoExposure = true
						}
						if enhancements.AutoWhiteBalance {
							p.config.AutoWhiteBalance = true
						}
						if enhancements.Saturation > 0 {
							p.config.Saturation = enhancements.Saturation
						}
						if enhancements.Vibrance > 0 {
							p.config.Vibrance = enhancements.Vibrance
						}
						if enhancements.LocalContrast > 0 {
							p.config.LocalContrast = enhancements.LocalContrast
						}
						if enhancements.Sharpening > 0 {
							p.config.Sharpening = enhancements.Sharpening
						}
					}
				}

				req := RawConvertRequest{
					InputFile:  f,
					OutputFile: filepath.Join(workDir, strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))+".jpg"),
				}
				res, err = proc.Convert(ctx, req)
			} else {
				// Fall back to default behavior if preferred tool not available
				res, err = mgr.ConvertWithFallback(ctx, f, "", workDir)
			}
		} else {
			// Use default tool selection
			res, err = mgr.ConvertWithFallback(ctx, f, "", workDir)
		}
		results = append(results, res)
		if err != nil {
			if log != nil {
				log.Warn("raw conversion failed",
					"file", filepath.Base(f),
					"error", err,
					"progress", fmt.Sprintf("%d/%d", i+1, len(filesToProcess)),
				)
			}
			continue
		} else {
			// Cache the processed file
			baseName := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			cacheFile := filepath.Join(cacheDir, baseName+".jpg")
			if err := copyFile(res.OutputFile, cacheFile); err != nil {
				if log != nil {
					log.Warn("failed to cache processed file", "output", res.OutputFile, "cache", cacheFile, "error", err)
				}
			}

			if log != nil {
				log.Info("raw conversion completed",
					"file", filepath.Base(f),
					"output", filepath.Base(res.OutputFile),
					"cached", filepath.Base(cacheFile),
					"progress", fmt.Sprintf("%d/%d", i+1, len(filesToProcess)),
				)
			}
		}

		if store != nil {
			if meta, err := ExtractEXIF(ctx, res.OutputFile); err == nil {
				_ = store.RecordImageMetadata(meta)
			}
		}
	}

	if log != nil {
		successCount := 0
		for _, r := range results {
			if r.Error == nil {
				successCount++
			}
		}
		log.Info("RAW preprocessing completed",
			"total_files", len(rawFiles),
			"cached_files", len(cachedFiles),
			"processed_files", len(filesToProcess),
			"successful", successCount,
			"failed", len(filesToProcess)-successCount,
			"work_dir", workDir,
		)
	}

	return workDir, results, nil
}

func isRawExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".cr2", ".cr3", ".nef", ".arw", ".rw2", ".dng", ".raf", ".orf":
		return true
	default:
		return false
	}
}

// fileExistsInCache checks if a file exists
func fileExistsInCache(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// isCacheValid checks if the cached file is newer than the source RAW file
func isCacheValid(rawFile, cacheFile string) bool {
	rawInfo, err := os.Stat(rawFile)
	if err != nil {
		return false
	}

	cacheInfo, err := os.Stat(cacheFile)
	if err != nil {
		return false
	}

	// Cache is valid if it's newer than the RAW file
	return cacheInfo.ModTime().After(rawInfo.ModTime()) || cacheInfo.ModTime().Equal(rawInfo.ModTime())
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Copy file permissions and timestamps
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.Chmod(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	return os.Chtimes(dst, time.Now(), srcInfo.ModTime())
}
