# TIFF timelapse share exports

Photonic's timelapse renderer accepts TIFF/TIF frames alongside the other image
formats returned by its image scanner. Frames are sorted by path and passed as
an explicit sequence, so mixed TIFF/JPEG/PNG inputs do not depend on a file glob
or the first frame's decoder. Homogeneous sequences are streamed through an
FFmpeg concat manifest; mixed formats use separate decoder inputs. Paths are
passed as arguments or safely quoted in the manifest, never through a shell.

## YouTube share set

Use `--share-set youtube` to create a 4K UHD and 1080p H.264 MP4 pair in one
run. Use `--share-set youtube-mobile` to add a 720p rendition. Each output is
named with its resolution suffix, for example `timelapse-4k.mp4` and
`timelapse-1080p.mp4`. Existing outputs retain Photonic's timestamped backup
behavior when a new source render is needed. The highest requested rendition
is rendered first; lower MP4 renditions are downsampled from that rendered
video. A valid higher-resolution sibling output can also supply a requested
lower resolution, avoiding another read of the source frames.

```sh
photonic timelapse /path/to/darktable-tiff-frames \
  --share-set youtube \
  --fps 24 \
  --quality high \
  --output-dir /path/to/photography-staging/projects/timelapse/rendered
```

Use `--resolution 1080p` (or `4k`, `1440p`, `720p`, `480p`, `240p`) for one
output. `--quality` accepts `high`, `medium`, or `low` and maps to H.264 CRF
18, 20, or 23. The default is medium.

## Framing and image data

Each rendition uses Lanczos scaling to fit the complete source frame inside an
exact 16:9 canvas, then adds centered black padding. It does not stretch or
crop the source. For example, a 3:2 frame is fitted to 3240x2160 within a
3840x2160 4K canvas, or 1620x1080 within a 1920x1080 canvas.

The renderer reads source frames and writes only the requested video outputs.
It does not change the input files or Darktable catalog. The output video is
H.264 with `yuv420p` for broad device compatibility. Higher resolution and
higher quality increase encoding time and file size.

## Current staging pilot

For the 257-frame Darktable TIFF export at
`/mnt/jmh/vault/photography-staging/projects/timelapse/2026/11/11`, write
renditions to a separate `rendered` directory so the TIFF sequence remains
untouched:

```sh
photonic timelapse /mnt/jmh/vault/photography-staging/projects/timelapse/2026/11/11 \
  --share-set youtube \
  --fps 24 \
  --quality high \
  --output-dir /mnt/jmh/vault/photography-staging/projects/timelapse/2026/11/11/rendered
```

The first run should be monitored for duration and output size. A 4K encode
from 257 large 16-bit TIFF frames can take substantially longer than the
1080p version. This command is not run by the build or tests.
