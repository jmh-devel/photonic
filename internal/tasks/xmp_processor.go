// Package tasks provides XMP-guided pano-safe image processing
//
// This implements a darktable .xmp parser and applies only safe, gentle
// ImageMagick operations that won't blow out highlights or reduce control points.
// The goal is to get visual improvements while maintaining panoramic stitching quality.

package tasks

import (
	"encoding/xml"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/gographics/imagick.v3/imagick"
)

// HistoryEntry represents a darktable operation from the XMP history
type HistoryEntry struct {
	Num       int
	Operation string
	Enabled   bool
	Params    string
}

// XMP structure for parsing darktable files
type XMPMeta struct {
	XMLName xml.Name `xml:"xmpmeta"`
	RDF     struct {
		Description struct {
			History struct {
				Seq struct {
					Li []struct {
						Num       string `xml:"num,attr"`
						Operation string `xml:"operation,attr"`
						Enabled   string `xml:"enabled,attr"`
						Params    string `xml:"params,attr"`
					} `xml:"li"`
				} `xml:"Seq"`
			} `xml:"history"`
		} `xml:"Description"`
	} `xml:"RDF"`
}

// Operation constants
const (
	OpTemperature     = "temperature"
	OpChannelMixerRGB = "channelmixerrgb"
	OpExposure        = "exposure"
	OpDenoiseProfile  = "denoiseprofile"
	OpHazeRemoval     = "hazeremoval"
	OpSharpen         = "sharpen"
	OpColorBalanceRGB = "colorbalancergb"
	OpBilat           = "bilat"
	OpToneEqual       = "toneequal"
	OpFilmicRGB       = "filmicrgb"
)

// Modules we hard-skip (blowout-risk or already handled by RAW decoder)
var skipOps = map[string]bool{
	OpToneEqual:    true, // Shadow tone eq - DANGEROUS for panos
	OpFilmicRGB:    true, // We'll do our own gentle S-curve instead
	"rawprepare":   true, // RAW decoder handles this
	"demosaic":     true, // RAW decoder handles this
	"colorin":      true, // RAW decoder handles this
	"colorout":     true, // RAW decoder handles this
	"gamma":        true, // RAW decoder handles this
	"highlights":   true, // RAW decoder handles this
	"cacorrect":    true, // RAW decoder handles this
	"cacorrectrgb": true, // RAW decoder handles this
	"hotpixels":    true, // RAW decoder handles this
	"flip":         true, // RAW decoder handles this
}

// Creative/safe modules that we can map to gentle IM operations
var creativeOps = map[string]bool{
	OpTemperature:     true,
	OpChannelMixerRGB: true,
	OpExposure:        true,
	OpDenoiseProfile:  true,
	OpHazeRemoval:     true,
	OpSharpen:         true,
	OpColorBalanceRGB: true,
	OpBilat:           true,
}

// ParseDarktableModules parses XMP and extracts enabled history entries
func ParseDarktableModules(xmpPath string) ([]HistoryEntry, error) {
	data, err := os.ReadFile(xmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read XMP file: %w", err)
	}

	var xmp XMPMeta
	if err := xml.Unmarshal(data, &xmp); err != nil {
		return nil, fmt.Errorf("failed to parse XMP: %w", err)
	}

	var entries []HistoryEntry
	for _, li := range xmp.RDF.Description.History.Seq.Li {
		// Parse attributes
		num, err := strconv.Atoi(li.Num)
		if err != nil {
			continue // Skip invalid entries
		}

		enabled := li.Enabled == "1"
		operation := li.Operation

		// Only include enabled operations we don't skip
		if enabled && !skipOps[operation] && creativeOps[operation] {
			entries = append(entries, HistoryEntry{
				Num:       num,
				Operation: operation,
				Enabled:   enabled,
				Params:    li.Params,
			})
		}
	}

	// Sort by darktable order (num)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Num < entries[j].Num
	})

	return entries, nil
}

// Gentle pano-safe operations that won't blow out highlights
func applyExposureLook(w *imagick.MagickWand, strength float64) error {
	// Very gentle S-curve using SigmoidalContrast
	// Keep contrast modest to avoid blowing out skies
	contrast := 2.0 * strength // Max 2.0 contrast
	if contrast < 1.0 {
		contrast = 1.0
	}
	return w.SigmoidalContrastImage(true, contrast, 0.5)
}

func applyTemperatureLook(w *imagick.MagickWand, strength float64) error {
	// Very slight warm tint via ModulateImage
	// Hue > 100 warms, < 100 cools. Keep within +/- 2.
	hue := 100.0 + 2.0*strength // Mild warm bias
	return w.ModulateImage(100, 100, hue)
}

func applyColorPop(w *imagick.MagickWand, strength float64) error {
	// Mild saturation boost; nothing crazy
	sat := 100.0 + 8.0*strength // Max +8% saturation
	return w.ModulateImage(100, sat, 100)
}

func applyDenoise(w *imagick.MagickWand, strength float64) error {
	// Use DespeckleImage for noise reduction
	if strength > 0.5 {
		// Apply twice for stronger denoising
		if err := w.DespeckleImage(); err != nil {
			return err
		}
		return w.DespeckleImage()
	}
	return w.DespeckleImage()
}

func applySharpen(w *imagick.MagickWand, strength float64) error {
	// Mild unsharp mask to keep detail, not crispy halos
	sigma := 1.0
	amount := 0.3 + 0.4*strength // 0.3..0.7
	threshold := 0.01
	return w.UnsharpMaskImage(0, sigma, amount, threshold)
}

func applyHazeRemoval(w *imagick.MagickWand, strength float64) error {
	// Very subtle contrast stretch; too much will blow out skies
	// Keep it tiny - 0.1% max, use float values
	blackPoint := 0.001 * strength     // 0..0.001 (0.1%)
	whitePoint := 1.0 - 0.001*strength // 0.999..1.0
	return w.ContrastStretchImage(blackPoint, whitePoint)
}

func applyLocalContrast(w *imagick.MagickWand, strength float64) error {
	// Bilateral/bilat → treat as a local contrast "clarity" effect
	// Very mild additional sigmoidal contrast
	contrast := 1.0 + 0.8*strength // 1..1.8 max
	return w.SigmoidalContrastImage(true, contrast, 0.5)
}

// ApplyPanoLookFromXMP applies pano-safe operations based on XMP history
func ApplyPanoLookFromXMP(w *imagick.MagickWand, entries []HistoryEntry, strength float64) error {
	if strength <= 0 {
		strength = 0.7 // Default gentle strength
	} else if strength > 1 {
		strength = 1
	}

	// Always ensure baseline state
	if err := w.SetImageDepth(16); err != nil {
		return fmt.Errorf("failed to set image depth: %w", err)
	}
	if err := w.SetImageColorspace(imagick.COLORSPACE_SRGB); err != nil {
		return fmt.Errorf("failed to set colorspace: %w", err)
	}
	if err := w.AutoOrientImage(); err != nil {
		return fmt.Errorf("failed to auto-orient: %w", err)
	}

	// Apply operations in darktable order
	for _, entry := range entries {
		switch entry.Operation {
		case OpExposure:
			if err := applyExposureLook(w, strength); err != nil {
				return fmt.Errorf("exposure look failed: %w", err)
			}

		case OpTemperature:
			if err := applyTemperatureLook(w, strength); err != nil {
				return fmt.Errorf("temperature look failed: %w", err)
			}

		case OpChannelMixerRGB, OpColorBalanceRGB:
			// Both can translate to saturation + slight pop
			if err := applyColorPop(w, strength); err != nil {
				return fmt.Errorf("color pop failed: %w", err)
			}

		case OpDenoiseProfile:
			if err := applyDenoise(w, strength); err != nil {
				return fmt.Errorf("denoise failed: %w", err)
			}

		case OpHazeRemoval:
			if err := applyHazeRemoval(w, strength); err != nil {
				return fmt.Errorf("haze removal failed: %w", err)
			}

		case OpSharpen:
			if err := applySharpen(w, strength); err != nil {
				return fmt.Errorf("sharpen failed: %w", err)
			}

		case OpBilat:
			if err := applyLocalContrast(w, strength*0.5); err != nil {
				return fmt.Errorf("local contrast failed: %w", err)
			}
		}
	}

	return nil
}

// ProcessRawWithXMP processes a RAW file using XMP-guided pano-safe operations
func ProcessRawWithXMP(inputRaw, xmpPath, outputPath string, strength float64) error {
	// Find corresponding XMP if not explicitly provided
	if xmpPath == "" {
		xmpPath = strings.TrimSuffix(inputRaw, ".CR2") + ".CR2.xmp"
		if _, err := os.Stat(xmpPath); os.IsNotExist(err) {
			// No XMP file, apply minimal processing
			return processWithMinimalEnhancement(inputRaw, outputPath, strength)
		}
	}

	// Parse XMP history
	entries, err := ParseDarktableModules(xmpPath)
	if err != nil {
		// If XMP parsing fails, fall back to minimal processing
		return processWithMinimalEnhancement(inputRaw, outputPath, strength)
	}

	// Initialize ImageMagick
	imagick.Initialize()
	defer imagick.Terminate()

	w := imagick.NewMagickWand()
	defer w.Destroy()

	// Read RAW image
	if err := w.ReadImage(inputRaw); err != nil {
		return fmt.Errorf("failed to read RAW image: %w", err)
	}

	// Apply XMP-guided look
	if err := ApplyPanoLookFromXMP(w, entries, strength); err != nil {
		return fmt.Errorf("failed to apply XMP look: %w", err)
	}

	// Export as 16-bit TIFF for Hugin
	if err := w.SetImageFormat("tiff"); err != nil {
		return fmt.Errorf("failed to set TIFF format: %w", err)
	}

	if err := w.WriteImage(outputPath); err != nil {
		return fmt.Errorf("failed to write processed image: %w", err)
	}

	return nil
}

// processWithMinimalEnhancement applies basic processing when no XMP is available
func processWithMinimalEnhancement(inputRaw, outputPath string, strength float64) error {
	imagick.Initialize()
	defer imagick.Terminate()

	w := imagick.NewMagickWand()
	defer w.Destroy()

	if err := w.ReadImage(inputRaw); err != nil {
		return fmt.Errorf("failed to read RAW image: %w", err)
	}

	// Basic setup
	if err := w.SetImageDepth(16); err != nil {
		return err
	}
	if err := w.SetImageColorspace(imagick.COLORSPACE_SRGB); err != nil {
		return err
	}
	if err := w.AutoOrientImage(); err != nil {
		return err
	}

	// Very minimal enhancement when no XMP guidance
	if strength > 0 {
		// Just a tiny bit of contrast and saturation
		if err := w.SigmoidalContrastImage(true, 1.5*strength, 0.5); err != nil {
			return err
		}

		sat := 100.0 + 5.0*strength
		if err := w.ModulateImage(100, sat, 100); err != nil {
			return err
		}
	}

	if err := w.SetImageFormat("tiff"); err != nil {
		return err
	}

	return w.WriteImage(outputPath)
}
