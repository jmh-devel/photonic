package tasks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolutionSizeAndFitFilter(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"4k", 3840, 2160},
		{"1080p", 1920, 1080},
		{"720p", 1280, 720},
	} {
		w, h, canonical, err := resolutionSize(tc.name)
		if err != nil || w != tc.w || h != tc.h || canonical != tc.name {
			t.Fatalf("resolutionSize(%q) = %d x %d, %q, %v", tc.name, w, h, canonical, err)
		}
		filter := getVideoFilter(tc.name)
		if !strings.Contains(filter, "force_original_aspect_ratio=decrease") || !strings.Contains(filter, "pad=") || !strings.Contains(filter, "setsar=1") {
			t.Errorf("fit filter for %s does not preserve aspect and pad: %s", tc.name, filter)
		}
	}
	if _, _, _, err := resolutionSize("8k"); err == nil {
		t.Fatal("expected unsupported resolution to fail")
	}
}

func TestFramesHaveSameExtension(t *testing.T) {
	if !framesHaveSameExtension([]string{"a.TIFF", "b.tiff"}) {
		t.Fatal("case-insensitive matching TIFF extensions should use the streaming concat path")
	}
	if framesHaveSameExtension([]string{"a.tif", "b.jpg"}) {
		t.Fatal("mixed image extensions must use independent decoders")
	}
}

func TestBuildTimelapseMixedTIFFJPEGShareSet(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	input := t.TempDir()
	output := t.TempDir()
	for _, frame := range []struct{ name, codec, color string }{
		{"frame 001.tif", "tiff", "red"},
		{"frame 002.jpg", "mjpeg", "blue"},
	} {
		path := filepath.Join(input, frame.name)
		cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c="+frame.color+":s=120x80", "-frames:v", "1", "-c:v", frame.codec, path)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generate %s fixture: %v: %s", frame.name, err, output)
		}
	}

	result, err := BuildTimelapse(context.Background(), TimelapseRequest{
		InputDir:    input,
		OutputDir:   output,
		FPS:         2,
		Formats:     []string{"mp4"},
		Resolutions: []string{"4k", "1080p"},
		Quality:     "low",
	})
	if err != nil {
		t.Fatalf("BuildTimelapse() error: %v", err)
	}
	if result.FrameCount != 2 || len(result.OutputFiles) != 2 {
		t.Fatalf("expected 2 source frames and 2 renditions, got %d and %d", result.FrameCount, len(result.OutputFiles))
	}
	for _, file := range result.OutputFiles {
		if file.Width == 0 || file.Height == 0 || file.Size == 0 {
			t.Errorf("incomplete output metadata: %+v", file)
		}
		cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", file.Path)
		data, err := cmd.Output()
		if err != nil {
			t.Fatalf("probe %s: %v", file.Path, err)
		}
		want := "1920x1080"
		if file.Resolution == "4k" {
			want = "3840x2160"
		}
		if got := strings.TrimSpace(string(data)); got != want {
			t.Errorf("%s dimensions = %q, want %q", file.Path, got, want)
		}
	}

	tiffOnly := t.TempDir()
	singleTIFF, err := os.ReadFile(filepath.Join(input, "frame 001.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tiffOnly, "single frame.tif"), singleTIFF, 0o600); err != nil {
		t.Fatal(err)
	}
	streamed, err := BuildTimelapse(context.Background(), TimelapseRequest{
		InputDir:   tiffOnly,
		Output:     filepath.Join(t.TempDir(), "single.mp4"),
		FPS:        2,
		Formats:    []string{"mp4"},
		Resolution: "1080p",
		Quality:    "low",
	})
	if err != nil {
		t.Fatalf("BuildTimelapse() TIFF-only error: %v", err)
	}
	if streamed.FrameCount != 1 || len(streamed.OutputFiles) != 1 || streamed.OutputFiles[0].Width != 1920 || streamed.OutputFiles[0].Height != 1080 {
		t.Fatalf("unexpected TIFF-only render result: %+v", streamed)
	}
}
