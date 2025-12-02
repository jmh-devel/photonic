# Photonic Enhancement Roadmap

> **🎯 NEW VISION**: This plan has been superseded by our comprehensive photo library management system vision. See `docs/COMPREHENSIVE_PHOTO_LIBRARY_VISION.md` for the complete roadmap.

## Current Status (✅ COMPLETED)
- [x] **Fixed context cancellation bug** - Timelapse processing now works reliably
- [x] **Implemented XMP-guided processing** - Darktable XMP files properly applied
- [x] **Added Cobra CLI framework** - Professional command structure with proper flag parsing
- [x] **Enhanced timelapse output control** - Proper directory targeting and file backup
- [x] **Fixed GIF generation** - Handles varying image dimensions correctly
- [x] **Cache management working** - `--no-cache` and `--no-preserve` flags operational

## Immediate Next Steps (Current Focus)
- [ ] **Server enhancement** - Transform basic HTTP server into full photo library API
- [ ] **Database schema expansion** - Add photo metadata, collections, keywords tables
- [ ] **File system watcher** - Monitor photography folders for automatic import
- [ ] **Smart processing workflows** - Rule-based automation for different photo types

---

## Legacy Enhancement Tasks (For Reference)

### Original Issues Identified:
1. ~~**Timelapse batch conversion fails**~~ ✅ **FIXED** - Context cancellation resolved
2. **Naive panoramic stitching** - Still needs proper feature matching with hugin
3. **No real stacking implementation** - Missing focus stacking, noise reduction, HDR  
4. ~~**Poor error handling**~~ ✅ **IMPROVED** - Better error handling and logging added

### Tool Integration Plan (Still Valid):

#### Phase 1: ✅ Current Issues **COMPLETED**

### Phase 2: Panoramic Stitching (hugin-tools)
```bash
# Replace current concatenation with proper stitching
pto_gen *.jpg -o project.pto
cpfind --multirow -o project.pto project.pto
cpclean -o project.pto project.pto
linefind -o project.pto project.pto
autooptimiser -a -m -l -s -o project.pto project.pto
pano_modify --straighten --center --fov=AUTO -o project.pto project.pto
hugin_executor --stitching --prefix=output project.pto
```

### Phase 3: Image Stacking (ALE)
```bash
# Noise reduction stacking
mogrify -resize 2048 *.tif
mogrify -format ppm *.tif
ale --md 64 *.ppm stacked.png
convert -normalize stacked.png final.png

# Focus stacking with alignment
ale --euclidean --follow *.ppm focus_stack.png

# HDR-like exposure blending
enfuse --exposure-weight=1 --saturation-weight=0.2 --contrast-weight=1 *.jpg -o hdr.tif
```

### Phase 4: Dependency Management System
Design auto-installer that:
- Checks `which command` first
- Falls back to `apt install package` if available
- Downloads pre-compiled binaries as last resort
- Supports cross-platform (Linux/macOS/Windows)

### Phase 5: Enhanced RAW Processing
- Scene detection for optimal conversion settings
- Batch processing with partial failure tolerance
- Quality retention analysis
- Format optimization (TIFF for processing, JPEG for final)

## Test Data Structure:
```
test-data/
├── raw/           # Single RAW files for conversion testing
├── timelapse/     # Sequence of images for video creation
├── panoramic/     # Overlapping images for stitching
└── stacking/      # Multiple exposures of same scene for noise reduction/focus
```

## Dependencies to Install:
```bash
apt install hugin-tools enblend enfuse ale imagemagick siril
```

## Quality Optimization Strategy:
- RAW → 16-bit TIFF for intermediate processing
- Use scene analysis for optimal conversion parameters
- Preserve maximum dynamic range through pipeline
- Final output format based on intended use case