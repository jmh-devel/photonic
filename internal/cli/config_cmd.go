package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
)

func (r *Root) cmdConfig(ctx context.Context, args []string) error {
	_ = ctx
	if len(args) == 0 {
		return r.configShow()
	}
	switch args[0] {
	case "show":
		return r.configShow()
	case "test-processors":
		return r.configTestProcessors(ctx)
	default:
		return fmt.Errorf("unknown config command: %s", args[0])
	}
}

func (r *Root) configShow() error {
	fmt.Printf("Current configuration:\n")
	cfgPath := os.Getenv("PHOTONIC_CONFIG")
	if cfgPath == "" {
		cfgPath = "(default) ~/.config/photonic/config.json"
	}
	fmt.Printf("Config file: %s\n", cfgPath)
	fmt.Printf("\nRAW Processing:\n")
	fmt.Printf("  Default tool: %s\n", r.cfg.Raw.DefaultTool)
	fmt.Printf("  Output format: %s\n", r.cfg.Raw.OutputFormat)
	fmt.Printf("  Quality: %d\n", r.cfg.Raw.Quality)
	fmt.Printf("  Use XMP: %t\n", r.cfg.Raw.UseXMP)
	fmt.Printf("  Temp directory: %s\n", r.cfg.Raw.TempDir)
	fmt.Printf("\nEnabled processors:\n")
	if r.cfg.Raw.Tools.Darktable.Enabled {
		fmt.Printf("  - darktable\n")
	}
	if r.cfg.Raw.Tools.ImageMagick.Enabled {
		fmt.Printf("  - imagemagick\n")
	}
	if r.cfg.Raw.Tools.DCraw.Enabled {
		fmt.Printf("  - dcraw\n")
	}
	if r.cfg.Raw.Tools.RawTherapee.Enabled {
		fmt.Printf("  - rawtherapee\n")
	}
	return nil
}

func (r *Root) configTestProcessors(ctx context.Context) error {
	fmt.Printf("Testing RAW processors...\n\n")
	mgr := r.newRawManager()
	available := mgr.DetectAvailable()
	if len(available) == 0 {
		fmt.Printf("❌ No RAW processors available\n")
		return nil
	}
	for _, name := range available {
		fmt.Printf("✅ %s: Available\n", name)
	}
	return nil
}

func (r *Root) cmdVersion() error {
	fmt.Printf("Photonic v1.0.0-dev\n")
	fmt.Printf("Built with Go %s\n", runtime.Version())
	fmt.Printf("Available processors:\n")
	mgr := r.newRawManager()
	for name, proc := range mgr.Processors() {
		status := "❌ unavailable"
		if proc.IsAvailable() {
			status = "✅ available"
		}
		fmt.Printf("  %s: %s\n", name, status)
	}
	return nil
}
