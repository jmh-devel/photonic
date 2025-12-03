# Photonic Development Roadmap

## DeepSkyStacker Integration & Future Improvements

### Current State (December 2025)

✅ **Completed:**
- DSS CLI integration with `--method dss`
- DSS text file format support
- Basic pipeline integration
- Working end-to-end DSS stacking

### Immediate Improvements Needed

🔧 **High Priority:**
- **Fix double alignment issue**: DSS should work on original RAW files when alignment is requested
- **Output file handling**: Move DSS Autosave.tif to correct output path
- **Add calibration frame support**: Bias, dark, flat, dark-flat frame options
- **CLI parameter mapping**: Map photonic options to DSS settings

🔧 **Medium Priority:**
- **Error handling**: Better DSS failure detection and error reporting
- **Progress reporting**: Parse DSS output for progress updates
- **Settings persistence**: Save DSS configuration in photonic config
- **File cleanup**: Remove temporary .dssfilelist files

### Long-term Strategic Goals

📈 **Study & Learn from DSS:**
1. **Analyze DSS source code** (C++/Qt available on GitHub)
2. **Study registration algorithms**: How DSS detects and matches stars
3. **Understand stacking math**: DSS's superior noise reduction techniques
4. **Research calibration processing**: Proper bias/dark/flat frame handling

📈 **Improve Native Photonic Implementation:**
1. **Enhanced star detection**: Implement DSS-level star finding
2. **Better registration**: More accurate alignment algorithms  
3. **Advanced stacking**: Sigma-clipping, kappa-sigma with proper math
4. **Calibration support**: Full bias/dark/flat processing
5. **Noise reduction**: Study DSS's superior noise handling

📈 **Eventually Replace DSS Dependency:**
- Goal: Native Go implementation as good as DSS
- Maintain DSS as fallback option
- Performance: Native Go should be faster than external process

### Performance Comparison (Current)

| Method | Time | Quality | Stars Detected | Notes |
|--------|------|---------|----------------|-------|
| Enfuse (Test 5) | ~8 min | Good | 40-78 per image | Our optimized version |
| DSS | ~21 sec | Excellent | 13-227 per image | Professional quality |

**DSS is 20x faster and higher quality** - this validates our approach to study and learn from it.

### Technical Debt to Address

- **Double alignment**: Fix workflow when both alignment and DSS are requested
- **Error handling**: DSS failures are not properly caught
- **File management**: Temporary files not cleaned up properly
- **Progress feedback**: No progress during long DSS operations

### Research Resources

- **DSS Source**: https://github.com/deepskystacker/DSS
- **DSS Documentation**: https://deepwiki.com/deepskystacker/DSS/
- **Astrophoto Theory**: Study DSS's algorithms in `DeepSkyStackerKernel/`
- **Registration Engine**: `RegisterEngine.cpp` - star detection & matching
- **Stacking Engine**: `StackingEngine.cpp` - noise reduction & combination

### Success Metrics

1. **Quality**: Native implementation matches DSS output quality
2. **Performance**: Native Go faster than external DSS process
3. **Features**: Full calibration frame support
4. **Reliability**: Better error handling than current DSS integration

---

*This roadmap will be updated as we make progress and learn more from DSS source code analysis.*