package tasks

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"photonic/internal/config"
)

func TestAlignAstroHybridProcessor(t *testing.T) {
	tmpBin := t.TempDir()
	createFakeDarktable(t, tmpBin)
	createFakeAlignImageStack(t, tmpBin)

	prev := os.Getenv("PATH")
	t.Setenv("PATH", tmpBin+string(os.PathListSeparator)+prev)

	cfg := config.DarktableConfig{Enabled: true}
	proc := NewAstroDarktableProvenProcessor(cfg)

	inputDir := t.TempDir()
	var inputs []string
	for i := 0; i < 3; i++ {
		path := filepath.Join(inputDir, filepath.Join("", "frame_"+strconv.Itoa(i)+".cr2"))
		if err := os.WriteFile(path, []byte("rawdata"), 0o644); err != nil {
			t.Fatalf("failed to write input: %v", err)
		}
		inputs = append(inputs, path)
	}

	outputDir := t.TempDir()
	res, err := proc.Align(context.Background(), AlignmentRequest{
		Images:    inputs,
		AlignType: AlignmentAstro,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("expected successful alignment, got %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got failure: %+v", res)
	}
	if len(res.AlignedImages) != len(inputs) {
		t.Fatalf("expected %d aligned images, got %d", len(inputs), len(res.AlignedImages))
	}
	if res.ToolUsed == "" || res.ToolUsed == "darktable+" {
		t.Fatalf("expected toolUsed to include aligner, got %q", res.ToolUsed)
	}
	for _, out := range res.AlignedImages {
		if _, statErr := os.Stat(out); statErr != nil {
			t.Fatalf("expected output file %s: %v", out, statErr)
		}
	}
}

// Helper utilities
func createFakeDarktable(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "darktable-cli")
	script := `#!/bin/sh
input="$1"
outdir="$2"
base=$(basename "$input")
name="${base%.*}"
mkdir -p "$outdir"
touch "$outdir/$name.tif"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake darktable: %v", err)
	}
}

func createFakeAlignImageStack(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "align_image_stack")
	script := `#!/bin/sh
prefix=""
if [ "$1" = "-a" ]; then
  prefix="$2"
  shift 2
fi
idx=0
for img in "$@"; do
  printf "" > "${prefix}$(printf "%04d" "$idx").tif"
  idx=$((idx+1))
done
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake align_image_stack: %v", err)
	}
}
