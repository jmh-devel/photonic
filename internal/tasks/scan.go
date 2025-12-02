package tasks

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"photonic/internal/fsutil"
)

// ScanResult captures detected assets.
type ScanResult struct {
	Images []string
	Groups []ImageGroup
}

// ImageGroup represents a detected set of related images.
type ImageGroup struct {
	GroupType string // timelapse|panoramic|stack
	BasePath  string
	Count     int
	Detection string
}

// Scan walks the directory and does simple grouping by sibling directory.
func Scan(input string) (ScanResult, error) {
	files, err := fsutil.ListImages(input)
	if err != nil {
		return ScanResult{}, err
	}
	sort.Strings(files)
	groups := groupFiles(files)
	return ScanResult{Images: files, Groups: groups}, nil
}

func groupFiles(files []string) []ImageGroup {
	if len(files) == 0 {
		return nil
	}
	dirMap := map[string][]string{}
	for _, f := range files {
		dirMap[filepath.Dir(f)] = append(dirMap[filepath.Dir(f)], f)
	}
	var groups []ImageGroup
	for dir, fs := range dirMap {
		sort.Strings(fs)
		groups = append(groups, classifyByPattern(dir, fs)...)
		groups = append(groups, classifyByTimestamp(dir, fs)...)
		if len(fs) > 0 {
			groups = append(groups, ImageGroup{
				GroupType: classify(len(fs)),
				BasePath:  dir,
				Count:     len(fs),
				Detection: "directory_size",
			})
		}
	}
	seen := map[string]bool{}
	var uniq []ImageGroup
	for _, g := range groups {
		key := g.BasePath + "|" + g.Detection + "|" + g.GroupType
		if seen[key] {
			continue
		}
		seen[key] = true
		uniq = append(uniq, g)
	}
	sort.Slice(uniq, func(i, j int) bool {
		if uniq[i].BasePath == uniq[j].BasePath {
			return uniq[i].Detection < uniq[j].Detection
		}
		return uniq[i].BasePath < uniq[j].BasePath
	})
	return uniq
}

func classify(count int) string {
	switch {
	case count >= 50:
		return "timelapse"
	case count >= 5:
		return "panoramic"
	default:
		return "stack"
	}
}

func classifyByPattern(dir string, files []string) []ImageGroup {
	re := regexp.MustCompile(`^(?P<prefix>.*?)(?P<num>\\d+)(?P<suffix>\\D*)$`)
	byPrefix := map[string][]string{}
	for _, f := range files {
		name := filepath.Base(f)
		m := re.FindStringSubmatch(name)
		if len(m) == 0 {
			continue
		}
		prefix := m[1]
		byPrefix[prefix] = append(byPrefix[prefix], f)
	}
	var groups []ImageGroup
	for _, fs := range byPrefix {
		if len(fs) < 3 {
			continue
		}
		groups = append(groups, ImageGroup{
			GroupType: classify(len(fs)),
			BasePath:  dir,
			Count:     len(fs),
			Detection: "filename_sequence",
		})
	}
	return groups
}

func classifyByTimestamp(dir string, files []string) []ImageGroup {
	if len(files) == 0 {
		return nil
	}
	type fileInfo struct {
		path string
		t    time.Time
	}
	var infos []fileInfo
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		infos = append(infos, fileInfo{path: f, t: st.ModTime()})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].t.Before(infos[j].t) })
	const gap = 60 * time.Second
	var groups []ImageGroup
	start := 0
	for i := 1; i <= len(infos); i++ {
		if i == len(infos) || infos[i].t.Sub(infos[i-1].t) > gap {
			count := i - start
			if count >= 3 {
				groups = append(groups, ImageGroup{
					GroupType: classify(count),
					BasePath:  dir,
					Count:     count,
					Detection: "timestamp_cluster",
				})
			}
			start = i
		}
	}
	return groups
}

// TouchManifest writes a small manifest file for downstream steps.
func TouchManifest(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"+content+"\n"), 0o644)
}
