# Photonic Enhancement Roadmap

## Current Status Summary (Nov 2025)

### Major Accomplishments ✅

1. **Cobra CLI Framework Migration**
   - Replaced broken positional argument parsing with professional Cobra CLI
   - Flags now work in any order, proper help system
   - Resolved "fucking annoying" flag parsing issues permanently

2. **XMP-Guided Pano-Safe Processing**
   - Implemented darktable .xmp file parser for history extraction
   - Created pano-safe ImageMagick operations that maintain control point detection
   - **SUCCESS**: Maintains 98 control points while applying enhancements (vs 12 with aggressive processing)
   - Applies gentle versions of: exposure, temperature, color balance, denoise, sharpen, haze removal, local contrast
   - Safely ignores dangerous modules: toneequal, filmicrgb (which blow out highlights)

3. **Cache Management Fixes**
   - `--no-cache` properly ignores existing cache files
   - `--no-preserve` cleans up processed files after completion
   - Fixed cache flag semantics and behavior

4. **Processing Quality vs Control Points Balance**
   - **Conservative**: 98 control points, basic quality (108MB panorama)
   - **Aggressive**: 12 control points, blown out images (8.9MB panorama)
   - **XMP-Guided**: 98 control points + enhanced quality (126MB panorama) 🎯

### Current Architecture

```
RAW Processing Pipeline:
┌─────────────┐    ┌─────────────────┐    ┌──────────────────┐
│ RAW + XMP   │ →  │ XMP Parser      │ →  │ Pano-Safe Ops    │
│ Files       │    │ (History)       │    │ (Gentle Enhancement) │
└─────────────┘    └─────────────────┘    └──────────────────┘
                                                    ↓
┌─────────────┐    ┌─────────────────┐    ┌──────────────────┐
│ Final TIFF  │ ←  │ ImageMagick     │ ←  │ Apply in DT Order│
│ for Hugin   │    │ Processing      │    │ (9 operations)   │
└─────────────┘    └─────────────────┘    └──────────────────┘
```

## Short-Term Enhancements (Next 2-4 weeks)

### 1. Timelapse System Validation
- **Priority**: High
- **Description**: Ensure timelapse functionality works with new Cobra CLI and XMP processing
- **Tasks**:
  - Test existing timelapse sample data
  - Validate frame processing consistency
  - Ensure no regression from CLI migration

### 2. XMP Processing Parameter Tuning
- **Priority**: Medium
- **Current Issue**: Still needs fine-tuning for optimal quality vs control points
- **Adjustments Needed**:
  ```go
  // Current conservative settings (0.6 strength)
  // May need adjustment for:
  - Contrast levels (currently 2.0 * strength)
  - Saturation boost (currently 8.0 * strength)
  - Haze removal (currently 0.1 * strength)
  - Sharpening parameters (0.3-0.7 range)
  ```

## Medium-Term Development (1-3 months)

### 3. HDR Processing Pipeline

#### 3.1 Simulated Bracket Generation
**For single RAW files without actual bracketing:**

```
Single RAW → Generate Virtual Brackets
├── Underexposed: -2.0 EV
├── Normal: 0.0 EV  
└── Overexposed: +2.0 EV

Processing Steps:
1. Read RAW with different exposure offsets
2. Apply EV adjustments via:
   - LevelImage() for exposure modification
   - Highlight/Shadow recovery
   - Preserve detail in all ranges
3. Generate 3-5 exposure variants per source image
```

#### 3.2 HDR Tone Mapping Algorithms
**Implementation Priority:**

1. **Reinhard Global** (Simple, fast)
   ```go
   func ReinhardGlobal(hdr *imagick.MagickWand, keyValue float64) error
   ```

2. **Mantiuk TMO** (Perceptual quality)
   ```go
   func MantiukTMO(hdr *imagick.MagickWand, contrast float64) error
   ```

3. **Drago Adaptive Logarithmic** (Good for landscapes)
   ```go
   func DragoTMO(hdr *imagick.MagickWand, bias float64) error
   ```

#### 3.3 HDR Pipeline Architecture
```
Input Options:
├── Actual Bracketed Images (3-7 exposures)
│   └── Direct HDR merge
└── Single RAW Files
    ├── Generate -2EV, 0EV, +2EV variants
    └── Process as bracketed set

HDR Processing:
1. Align images (if needed)
2. Merge exposures → Linear HDR
3. Ghost removal (for moving objects)
4. Tone mapping (user choice)
5. Output 16-bit TIFF or high-quality JPEG
```

### 4. Advanced RAW Processing Features

#### 4.1 Intelligent Exposure Analysis
```go
// Analyze RAW histogram to determine optimal processing
func AnalyzeExposure(raw string) (ExposureMetrics, error) {
    // Detect: underexposed, overexposed, well-exposed
    // Return recommendations for processing parameters
}
```

#### 4.2 Batch Processing Optimization
- Parallel processing for large sets
- Progress tracking and resumability
- Memory management for high-resolution files

## Long-Term Vision (3-12 months)

### 5. Advanced Panoramic Features

#### 5.1 Multi-Row Panoramas
- Support for spherical/360° panoramas
- Automatic row detection and alignment
- Zenith/nadir completion

#### 5.2 Gigapixel Processing
- Tiled processing for massive panoramas
- Memory-efficient streaming
- Progressive output generation

### 6. AI-Enhanced Processing

#### 6.1 Intelligent Control Point Detection
- Machine learning for better control point quality assessment
- Adaptive processing based on control point analysis
- Scene-type detection (landscape, architecture, macro)

#### 6.2 Content-Aware Processing
- Sky detection for selective enhancement
- Object recognition for masking
- Automatic scene optimization

### 7. Professional Workflow Integration

#### 7.1 Color Management
- ICC profile support
- Color space conversion
- Professional printing workflows

#### 7.2 Metadata Preservation
- EXIF data management
- XMP workflow integration
- Asset tracking and organization

## Technical Architecture Evolution

### Current Strengths
- ✅ Robust CLI with Cobra framework
- ✅ XMP-guided processing that preserves control points
- ✅ Flexible RAW processor abstraction (darktable, ImageMagick, dcraw, RawTherapee)
- ✅ Professional logging and error handling
- ✅ Cache management and cleanup

### Areas for Enhancement
- 🔄 HDR processing capabilities
- 🔄 Advanced tone mapping
- 🔄 Parallel processing optimization
- 🔄 Memory management for large files
- 🔄 GUI frontend (future consideration)

## Implementation Strategy

### Phase 1: Core HDR (Months 1-2)
1. Simulated bracket generation from single RAW
2. Basic HDR merging
3. Reinhard tone mapping implementation

### Phase 2: Advanced HDR (Months 2-3)
1. Multiple tone mapping algorithms
2. Ghost removal
3. Advanced alignment

### Phase 3: Professional Features (Months 3-6)
1. Multi-row panorama support
2. Color management
3. Workflow optimization

### Phase 4: AI Integration (Months 6-12)
1. Intelligent scene analysis
2. Automated parameter optimization
3. Content-aware processing

## Development Guidelines

### Code Quality Standards
- Maintain test coverage >80%
- Use Go best practices and idioms
- Comprehensive error handling
- Performance profiling for critical paths

### Processing Philosophy
1. **Quality First**: Never compromise final output quality
2. **Control Point Preservation**: Essential for panoramic stitching
3. **User Choice**: Provide options, don't force decisions
4. **Performance Awareness**: Optimize for reasonable processing times
5. **Workflow Integration**: Fit into existing photography workflows

### Documentation Requirements
- API documentation for all public interfaces
- Processing algorithm explanations
- Performance characteristics
- Usage examples and tutorials

---

## Notes on Current Implementation

**XMP-Guided Processing Success Factors:**
- Treats XMP as hints, not literal reproduction
- Skips dangerous modules (toneequal, filmicrgb)
- Uses fixed-strength operations for consistency
- Maintains pano-safety while improving quality
- Achieved 98 control points + enhanced visuals

**Key Learning:** The breakthrough was understanding that panoramic stitching requires **consistent** processing across all frames, not necessarily **perfect** darktable reproduction. The XMP-guided approach gives us the best of both worlds.

**Performance Notes:** 
- XMP-guided: ~9 seconds per RAW file
- Conservative: ~8 seconds per RAW file  
- Quality improvement with minimal time penalty

**Future Research Areas:**
- Optimal strength parameters for different scene types
- Adaptive processing based on image analysis
- Integration with darktable's actual processing engine