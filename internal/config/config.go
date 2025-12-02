package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	defaultConfigPath = "~/.config/photonic/config.json"
	defaultParallel   = 4
)

// Config holds user-editable settings for the pipeline.
type Config struct {
	Processing Processing      `json:"processing"`
	Logging    Logging         `json:"logging"`
	Paths      Paths           `json:"paths"`
	Raw        RawProcessing   `json:"raw_processing"`
	Alignment  AlignmentConfig `json:"alignment"`
	Tools      ToolPreferences `json:"tools"`
}

// Processing captures execution preferences.
type Processing struct {
	ParallelJobs int    `json:"parallel_jobs"`
	TempDir      string `json:"temp_dir"`
	MemoryLimit  string `json:"memory_limit"`
}

// Logging controls logging verbosity and destinations.
type Logging struct {
	Level      string `json:"level"`       // debug, info, warn, error
	Format     string `json:"format"`      // text, json
	FileOutput bool   `json:"file_output"` // Enable file logging
	LogDir     string `json:"log_dir"`     // Directory for log files
	MaxSize    int    `json:"max_size"`    // Max size in MB before rotation
	MaxBackups int    `json:"max_backups"` // Number of backup files to keep
	MaxAge     int    `json:"max_age"`     // Days to keep log files
}

// Paths configures default input/output locations.
type Paths struct {
	DefaultInput  string `json:"default_input"`
	DefaultOutput string `json:"default_output"`
	DatabasePath  string `json:"database_path"`
}

// ToolPreferences defines which tools to use for each operation type.
type ToolPreferences struct {
	RAWProcessing RAWToolConfig       `json:"raw_processing"`
	PanoramicTool PanoramicToolConfig `json:"panoramic"`
	StackingTool  StackingToolConfig  `json:"stacking"`
	TimelapseTool TimelapseToolConfig `json:"timelapse"`
	AlignmentTool AlignmentToolConfig `json:"alignment"`
}

type RAWToolConfig struct {
	Preferred string   `json:"preferred"` // "imagemagick", "darktable", "dcraw", "rawtherapee"
	Fallbacks []string `json:"fallbacks"`
	Quality   int      `json:"quality"`
	Format    string   `json:"format"` // "jpg", "tiff", "png"
}

type PanoramicToolConfig struct {
	Preferred string   `json:"preferred"` // "hugin", "imagemagick"
	Fallbacks []string `json:"fallbacks"`
	Blending  string   `json:"blending"`  // "enblend", "enfuse", "none"
	Alignment string   `json:"alignment"` // "cpfind", "autopano", "manual"
}

type StackingToolConfig struct {
	Preferred string   `json:"preferred"` // "ale", "siril", "imagemagick", "enfuse"
	Fallbacks []string `json:"fallbacks"`
	Mode      string   `json:"mode"`      // "noise_reduction", "focus", "hdr", "exposure"
	Alignment string   `json:"alignment"` // "translation", "euclidean", "projective"
}

type TimelapseToolConfig struct {
	Preferred string   `json:"preferred"` // "ffmpeg", "mencoder", "avconv"
	Fallbacks []string `json:"fallbacks"`
	Codec     string   `json:"codec"`   // "h264", "h265", "vp9"
	Quality   string   `json:"quality"` // "high", "medium", "low"
}

type AlignmentToolConfig struct {
	Preferred string   `json:"preferred"` // "align_image_stack", "hugin", "imagemagick"
	Fallbacks []string `json:"fallbacks"`
	Method    string   `json:"method"`    // "feature", "correlation", "manual"
	Precision string   `json:"precision"` // "subpixel", "pixel"
}

// AlignmentConfig controls alignment processors.
type AlignmentConfig struct {
	DefaultProcessor string                   `json:"default_processor"`
	TempDirectory    string                   `json:"temp_directory"`
	MaxConcurrency   int                      `json:"max_concurrency"`
	Darktable        DarktableConfig          `json:"darktable"`
	Astro            AstroAlignmentConfig     `json:"astro"`
	Panoramic        PanoramicAlignmentConfig `json:"panoramic"`
	General          GeneralAlignmentConfig   `json:"general"`
}

type AstroAlignmentConfig struct {
	Enabled           bool     `json:"enabled"`
	StarDetection     string   `json:"star_detection"`
	PlateSolver       bool     `json:"platesolver"`
	Subpixel          bool     `json:"subpixel"`
	DriftCompensation bool     `json:"drift_compensation"`
	MaxStars          int      `json:"max_stars"`
	MinStarBrightness float64  `json:"min_brightness"`
	SIRILPath         string   `json:"siril_path"`
	AstrometryAPIKey  string   `json:"astrometry_key"`
	ExtraArgs         []string `json:"extra_args"`
}

type PanoramicAlignmentConfig struct {
	Enabled           bool     `json:"enabled"`
	FeatureDetector   string   `json:"feature_detector"`
	MatcherType       string   `json:"matcher"`
	MinOverlap        float64  `json:"min_overlap"`
	MaxReprojError    float64  `json:"max_reproj_error"`
	BundleAdjustment  bool     `json:"bundle_adjustment"`
	HuginToolsPath    string   `json:"hugin_tools_path"`
	UseHuginAlignment bool     `json:"use_hugin"`
	ExtraArgs         []string `json:"extra_args"`
}

type GeneralAlignmentConfig struct {
	Enabled          bool     `json:"enabled"`
	Method           string   `json:"method"`
	FeatureDetector  string   `json:"feature_detector"`
	TemplateMatching bool     `json:"template_matching"`
	PhaseCorrelation bool     `json:"phase_correlation"`
	MultiScale       bool     `json:"multiscale"`
	RobustEstimation bool     `json:"robust_estimation"`
	ExtraArgs        []string `json:"extra_args"`
}

// RawProcessing configures RAW conversions.
type RawProcessing struct {
	DefaultTool  string   `json:"default_tool"`
	OutputFormat string   `json:"output_format"`
	Quality      int      `json:"quality"`
	UseXMP       bool     `json:"use_xmp"`
	TempDir      string   `json:"temp_dir"`
	Tools        RawTools `json:"tools"`
}

type RawTools struct {
	Darktable   DarktableConfig   `json:"darktable"`
	ImageMagick ImageMagickConfig `json:"imagemagick"`
	DCraw       DCrawConfig       `json:"dcraw"`
	RawTherapee RawTherapeeConfig `json:"rawtherapee"`
}

type DarktableConfig struct {
	Enabled          bool     `json:"enabled"`
	ApplyPresets     bool     `json:"apply_presets"`
	HighQuality      bool     `json:"high_quality"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	Style            string   `json:"style"`
	StyleOverwrite   bool     `json:"style_overwrite"`
	ExportMasks      bool     `json:"export_masks"`
	AutoExposure     bool     `json:"auto_exposure"`      // Auto exposure correction
	AutoWhiteBalance bool     `json:"auto_white_balance"` // Auto white balance
	Saturation       float64  `json:"saturation"`         // Saturation boost (1.25 = +25%)
	Vibrance         float64  `json:"vibrance"`           // Vibrance boost (1.5 = +50%)
	LocalContrast    float64  `json:"local_contrast"`     // Local contrast enhancement (0.3 = 30%)
	Sharpening       float64  `json:"sharpening"`         // Sharpening amount (0.5 = 50%)
	ExtraArgs        []string `json:"extra_args"`
}

type ImageMagickConfig struct {
	Enabled          bool     `json:"enabled"`
	Resize           string   `json:"resize"`
	Sharpen          string   `json:"sharpen"`
	Contrast         string   `json:"contrast"`
	Brightness       string   `json:"brightness"`
	Colorspace       string   `json:"colorspace"`
	Exposure         float64  `json:"exposure"`           // Auto exposure adjustment
	AutoWhiteBalance bool     `json:"auto_white_balance"` // Auto white balance
	Saturation       float64  `json:"saturation"`         // Saturation boost (1.25 = +25%)
	Vibrance         float64  `json:"vibrance"`           // Vibrance boost (1.5 = +50%)
	LocalContrast    float64  `json:"local_contrast"`     // Local contrast enhancement (0.3 = 30%)
	UnsharpMask      string   `json:"unsharp_mask"`       // Unsharp mask parameters
	ExtraArgs        []string `json:"extra_args"`
}

type DCrawConfig struct {
	Enabled      bool     `json:"enabled"`
	WhiteBalance string   `json:"white_balance"`
	ColorMatrix  int      `json:"color_matrix"`
	Gamma        string   `json:"gamma"`
	Brightness   float64  `json:"brightness"`
	ExtraArgs    []string `json:"extra_args"`
}

type RawTherapeeConfig struct {
	Enabled           bool     `json:"enabled"`
	ProcessingProfile string   `json:"processing_profile"`
	OutputProfile     string   `json:"output_profile"`
	ExtraArgs         []string `json:"extra_args"`
}

// Load reads configuration from disk, falling back to sensible defaults.
func Load() (*Config, error) {
	cfg := defaultConfig()

	configPath := os.Getenv("PHOTONIC_CONFIG")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	expanded, err := expandUser(configPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(expanded)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	if err := dec.Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Processing: Processing{
			ParallelJobs: defaultParallel,
			TempDir:      os.TempDir(),
			MemoryLimit:  "8GB",
		},
		Logging: Logging{
			Level:      "info",
			Format:     "text",
			FileOutput: true,
			LogDir:     "./logs",
			MaxSize:    100, // 100MB
			MaxBackups: 5,
			MaxAge:     30, // 30 days
		},
		Paths: Paths{
			DefaultInput:  ".",
			DefaultOutput: "./output",
			DatabasePath:  filepath.Join(os.TempDir(), "photonic.db"),
		},
		Alignment: AlignmentConfig{
			DefaultProcessor: "auto",
			TempDirectory:    filepath.Join(os.TempDir(), "photonic-align"),
			MaxConcurrency:   4,
			Darktable:        DarktableConfig{Enabled: true, ApplyPresets: true, HighQuality: true},
			Astro:            AstroAlignmentConfig{Enabled: true, StarDetection: "opencv", MaxStars: 2000, MinStarBrightness: 0.1},
			Panoramic:        PanoramicAlignmentConfig{Enabled: true, FeatureDetector: "sift", MatcherType: "flann", MinOverlap: 0.15, MaxReprojError: 4.0, BundleAdjustment: true, UseHuginAlignment: true},
			General:          GeneralAlignmentConfig{Enabled: true, Method: "feature", FeatureDetector: "orb", MultiScale: true, RobustEstimation: true},
		},
		Raw: RawProcessing{
			DefaultTool:  "imagemagick",
			OutputFormat: "jpg",
			Quality:      90,
			UseXMP:       true,
			TempDir:      filepath.Join(os.TempDir(), "photonic-raw"),
			Tools: RawTools{
				Darktable:   DarktableConfig{Enabled: true, ApplyPresets: true, HighQuality: true},
				ImageMagick: ImageMagickConfig{Enabled: true},
				DCraw:       DCrawConfig{Enabled: false},
				RawTherapee: RawTherapeeConfig{Enabled: false},
			},
		},
		Tools: ToolPreferences{
			RAWProcessing: RAWToolConfig{
				Preferred: "imagemagick",
				Fallbacks: []string{"darktable", "dcraw", "rawtherapee"},
				Quality:   90,
				Format:    "tiff", // 16-bit TIFF for best quality retention
			},
			PanoramicTool: PanoramicToolConfig{
				Preferred: "hugin",
				Fallbacks: []string{"imagemagick"},
				Blending:  "enblend",
				Alignment: "cpfind",
			},
			StackingTool: StackingToolConfig{
				Preferred: "ale",
				Fallbacks: []string{"siril", "enfuse", "imagemagick"},
				Mode:      "noise_reduction",
				Alignment: "euclidean",
			},
			TimelapseTool: TimelapseToolConfig{
				Preferred: "ffmpeg",
				Fallbacks: []string{"mencoder", "avconv"},
				Codec:     "h264",
				Quality:   "high",
			},
			AlignmentTool: AlignmentToolConfig{
				Preferred: "align_image_stack",
				Fallbacks: []string{"hugin", "imagemagick"},
				Method:    "feature",
				Precision: "subpixel",
			},
		},
	}
}

func expandUser(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, path[2:]), nil
}
