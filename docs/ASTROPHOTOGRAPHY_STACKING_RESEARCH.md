# Astrophotography Stacking Research & Best Practices

## Research Summary

Based on research from LifePixel, Reddit astrophotography communities, and Sky at Night Magazine, here are the key findings about professional astrophotography stacking techniques and why our enfuse approach may be problematic.

## Key Research Findings

### 1. Signal-to-Noise Ratio (SNR) Theory
- **Mathematical Foundation**: SNR improves by the square root of the number of images (√N)
- **Practical Impact**: 
  - 4 images = 1 stop improvement (equivalent to halving ISO)
  - 16 images = 2 stops improvement  
  - 64 images = 3 stops improvement
  - **Diminishing returns** beyond 64 images for most scenarios

### 2. Recommended Image Counts
- **Beginners**: 8-15 images for noticeable improvement
- **Intermediate**: 25-50 images for significant enhancement  
- **Advanced**: 64+ images for maximum quality (diminishing returns beyond this)
- **Professional**: 10-20 hours total exposure time (hundreds of frames)

### 3. Professional Stacking Methods

#### Primary Methods Used by Professionals:
1. **Average/Mean Stacking**: Pure mathematical pixel averaging - **most common**
2. **Median Stacking**: Excellent for outlier rejection (satellites, cosmic rays)
3. **Sigma Clipping**: Iterative outlier rejection with statistical thresholds
4. **Kappa-Sigma**: Advanced statistical rejection method

#### Why These Work:
- **Pure mathematical operations** on pixel values
- **No multi-scale decomposition** that causes artifacts
- **Preserves star shapes** without introducing warping
- **True noise reduction** following √N improvement

### 4. Professional Software Tools

#### Dedicated Astrophotography Software:
- **Deep Sky Stacker (DSS)** - Free, industry standard
- **PixInsight** - Professional grade
- **MaximDL** - High-end professional
- **Sequator** - Free, excellent for beginners
- **Starry Landscape Stacker** - Mac-specific

#### Why NOT Enfuse:
- **Enfuse is designed for HDR/exposure blending**, not astronomical stacking
- **Multi-scale pyramid decomposition** creates artifacts around bright stars
- **Exposure fusion algorithms** assume different exposures, not identical ones
- **Laplacian pyramids** cause the "warp drive" effect we're experiencing

### 5. Critical Requirements for Clean Stacking

#### Alignment:
- **Sub-pixel accuracy** required for sharp stars
- **Star-based alignment** (not feature-based) for astronomical targets
- **Proper dithering** between frames to eliminate hot pixels

#### Calibration Frames:
- **Dark Frames**: Remove hot pixels and thermal noise
- **Flat Frames**: Remove vignetting and dust spots  
- **Bias Frames**: Remove read noise from sensor

#### Mathematical Stacking:
- **Pure pixel averaging** for maximum SNR improvement
- **No blending or fusion** that introduces artifacts
- **Hard rejection** of outliers (cosmic rays, satellites)

## Why Our Enfuse Approach is Problematic

### The Fundamental Issue:
Enfuse was designed for **exposure fusion** (combining different exposures of the same scene), not **signal averaging** (combining identical exposures for noise reduction).

### Specific Problems:
1. **Multi-scale blending** creates artifacts around bright stars
2. **Pyramid decomposition** introduces "warp drive" effects  
3. **Exposure weighting** assumes different exposures, causing bias
4. **Complex blending masks** prevent true mathematical averaging

### Why ImageMagick Failed:
- **Implementation bugs** in our Go ImageMagick bindings
- **Scan line artifacts** from improper pixel buffer handling
- **Not a fundamental limitation** of mathematical averaging

## Recommended Solution

### 1. Implement Dedicated Astronomical Stacker
Replace enfuse with a **dedicated astronomical stacking implementation** that:
- Performs **pure mathematical pixel averaging**
- Uses **sigma clipping** for outlier rejection
- Implements **sub-pixel alignment** accuracy
- Supports **proper calibration frame** integration

### 2. Use Proven External Tools
Integrate with established tools:
- **Deep Sky Stacker**: Industry standard, free
- **Align Image Stack**: Already proven for alignment
- **Custom averaging engine**: Pure Go implementation

### 3. Keep Enfuse for Specialized Cases
Maintain enfuse only for:
- **Star trails**: Where blending is actually desired
- **Focus stacking**: Where depth fusion is needed
- **HDR/exposure fusion**: Original intended use case

## Implementation Strategy

### Phase 1: External Tool Integration
```bash
# Use proven pipeline
darktable (RAW processing) → 
align_image_stack (precise alignment) → 
Deep Sky Stacker (mathematical averaging)
```

### Phase 2: Native Implementation
```go
// Pure mathematical stacking
type AstroStacker interface {
    AverageStack(images []string) error
    SigmaClipStack(images []string, sigma float64) error
    MedianStack(images []string) error
}
```

### Phase 3: Calibration Integration
```go
// Professional calibration
type CalibrationFrames struct {
    Darks []string  // Hot pixel removal
    Flats []string  // Vignetting correction  
    Bias  []string  // Read noise reduction
}
```

## Expected Results

### With Proper Implementation:
- ✅ **Sharp, clean stars** without warping artifacts
- ✅ **True √N SNR improvement** with mathematical averaging  
- ✅ **Professional quality** matching dedicated astro software
- ✅ **No scan lines** with proper pixel buffer handling
- ✅ **Excellent cosmic ray rejection** with sigma clipping

### Performance Targets:
- **11 images**: ~3.3x SNR improvement (1.2 stops)
- **25 images**: ~5x SNR improvement (2.3 stops)
- **50 images**: ~7x SNR improvement (2.8 stops)

## Conclusion

Our research confirms that **enfuse is fundamentally the wrong tool** for astronomical stacking. Professional astrophotographers use **dedicated mathematical stacking** with pure pixel averaging, not exposure fusion techniques.

The "warp drive" effect we're seeing is an **inherent limitation of enfuse's design**, not a parameter tuning issue. We need to implement or integrate **proper astronomical stacking tools** that follow established best practices.

The **ImageMagick approach was conceptually correct** - we just need to fix the implementation bugs rather than abandon mathematical averaging entirely.

---

*Research conducted: December 2, 2025*  
*Sources: LifePixel, Reddit r/astrophotography, Sky at Night Magazine*