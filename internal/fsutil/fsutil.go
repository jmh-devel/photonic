package fsutil

import (
	"os"
	"path/filepath"
	"strings"
)

var imageExts = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".tif":  {},
	".tiff": {},
	".dng":  {},
	".nef":  {},
	".cr2":  {},
	".cr3":  {},
	".arw":  {},
	".rw2":  {},
	".orf":  {},
	".pef":  {},
	".raf":  {},
	".srw":  {},
	".x3f":  {},
}

var rawExts = map[string]struct{}{
	".dng": {},
	".nef": {},
	".cr2": {},
	".cr3": {},
	".arw": {},
	".rw2": {},
	".orf": {},
	".pef": {},
	".raf": {},
	".srw": {},
	".x3f": {},
}

// ListImages returns all image-like files under root.
func ListImages(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := imageExts[ext]; ok {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// FirstExisting returns the first path that exists.
func FirstExisting(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// IsRAWFile checks if a file is a RAW camera format.
func IsRAWFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, isRaw := rawExts[ext]
	return isRaw
}

// IsImageFile checks if a file is any supported image format.
func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, isImage := imageExts[ext]
	return isImage
}

// SeparateRAWAndProcessed separates RAW files from processed images.
func SeparateRAWAndProcessed(files []string) (rawFiles, processedFiles []string) {
	for _, file := range files {
		if IsRAWFile(file) {
			rawFiles = append(rawFiles, file)
		} else if IsImageFile(file) {
			processedFiles = append(processedFiles, file)
		}
	}
	return rawFiles, processedFiles
}
