# Hugin Tutorials Reference

Source material: scraped first-page copies saved in `output/hugin-tutorials` from https://hugin.sourceforge.io/tutorials/index.shtml. Summaries capture what could be read from the downloads; pages that did not fully render are flagged.

## Official Hugin pages
- **Home** — https://hugin.sourceforge.io/ (`035-hugin-sourceforge-io.html`): landing page; highlights 2025.0.0 release, project status, and navigation to docs, downloads, and community.
- **Community** — https://hugin.sourceforge.io/community/ (`036-hugin-sourceforge-io-community.html`): community charter, coding style guide, and collaboration channels.
- **Documentation** — https://hugin.sourceforge.io/docs/ (`037-hugin-sourceforge-io-docs.html`): pointers to user/developer docs, localisation guidance, and developer mailing lists.
- **Download** — https://hugin.sourceforge.io/download/ (`038-hugin-sourceforge-io-download.html`): release history, platform binaries, and source build instructions.
- **Links** — https://hugin.sourceforge.io/links/ (`039-hugin-sourceforge-io-links.html`): related software (auto control-point generators, seam blenders, HDR, viewing tools).
- **Screenshots** — https://hugin.sourceforge.io/screenshots/ (`040-hugin-sourceforge-io-screenshots.html`): UI previews across releases and translations.
- **Tech/Research** — https://hugin.sourceforge.io/tech/ (`041-hugin-sourceforge-io-tech.html`): photometric alignment/vignetting experiments with sample panoramas.
- **Tutorials index** — https://hugin.sourceforge.io/tutorials/ (`042-hugin-sourceforge-io-tutorials.html`): lists the tutorial set summarised below.
- **SourceForge project** — http://sourceforge.net/projects/hugin (`016-sourceforge-net-projects-hugin.html`): project overview, features list, and download links.
- **Launchpad** — https://launchpad.net/hugin (`073-launchpad-net-hugin.html`): project metadata, release milestones, and bug tracker entry point.
- **Facebook page** — https://www.facebook.com/pg/hugin.panorama (`078-www-facebook-com-pg-hugin.panorama`): social feed (content not scraped beyond shell page).
- **Photo gallery (tagged hugin)** — http://www.flickriver.com/photos/tags/hugin/interesting/ (`022-www-flickriver-com-photos-tags-hugin-interesting.html`): aggregated Flickr examples of panoramas built with Hugin.

## Core Hugin tutorial pages (English unless stated)
- **New features in Hugin 2015.0.0** — https://hugin.sourceforge.io/tutorials/hugin-2015.0.0/ (`054-hugin-sourceforge-io-tutorials-hugin-2015-0.0`): control-point editing directly in Fast Preview; new `hugin_executor` replaces makefile backend; internal lens database; auto exposure-stack detection with unlink option; Verdandi blender; PTBatcherGUI gains thumbnails.
- **New user interface (2013)** — https://hugin.sourceforge.io/tutorials/new-gui (`059-hugin-sourceforge-io-tutorials-new-gui.html`): Simple/Advanced/Expert modes; workflow centered on Fast Preview with Assistant steps (Load → Align → Create); defaults to automated assistant but exposes expert controls.
- **Overview** — https://hugin.sourceforge.io/tutorials/overview/ (`060-hugin-sourceforge-io-tutorials-overview.html` and translations `061` FR, `062` JA): end-to-end stitching stages—collect images, supply FOV/EXIF, add hints for placement, optimize, choose projections, and output.
- **Stitching two photos** — https://hugin.sourceforge.io/tutorials/two-photos/ (`070-hugin-sourceforge-io-tutorials-two-photos.html` + FR `071`, JA `072`): fully automatic Align in Assistant; under-the-hood look at control points, optimizer, and preview; downloadable sample pair.
- **Stitching multi-row photos** — https://hugin.sourceforge.io/tutorials/multi-row/ (`057` + JA `058`): multi-row handheld sets via Assistant with a control-point generator (e.g., autopano-sift-C); shows full workflow with sample images.
- **Stitching photos from different lenses** — https://hugin.sourceforge.io/tutorials/multi-lens/ (`055` + JA `056`): mixes images from different focal lengths/cameras; Align handles differing lens parameters; exposes lens groups and optimization steps.
- **Simulating an architectural projection** — https://hugin.sourceforge.io/tutorials/architectural/ (`046` + FR `047`, JA `048`): use vertical control points to correct barrel distortion/perspective on a single photo; non-panoramic architectural correction.
- **Perspective correction** — https://hugin.sourceforge.io/tutorials/perspective/ (`063` + JA `064`): remove perspective deformation with horizontal/vertical control points; accurate enough for building surveying.
- **Stitching flat scanned images** — https://hugin.sourceforge.io/tutorials/scans/ (`065`): combine multiple scans of oversized documents (maps/posters/LP covers); handles rotation differences more reliably than manual GIMP assembly.
- **Stitching murals in mosaic mode** — https://hugin.sourceforge.io/tutorials/Mosaic-mode/ (`044`): mosaic mode for flat murals shot from varying positions/angles; includes masking to remove occluders; non-rotational stitching.
- **Using blend masks** — https://hugin.sourceforge.io/tutorials/Blend-masks/ (`043`): add include/exclude masks to control blender choices in overlap areas (e.g., avoid ghosted moving subjects).
- **Handling over-exposure in stitched images** — https://hugin.sourceforge.io/tutorials/Over-exposure/ (`045`): walkthrough correcting mismatched exposures between shots (example courthouse images) using Hugin’s interface and exposure tools.
- **Stitching auto-exposed panoramas** — https://hugin.sourceforge.io/tutorials/auto-exposure/ (`049` + JA `050`): for bracketed or auto-exposure sequences, combine seam blending and exposure fusion; supports stacks and blended/fused outputs.
- **Creating 360° enfused panoramas** — https://hugin.sourceforge.io/tutorials/enfuse-360/ (`052` + JA `053`): use Enfuse to merge bracketed 360° shots, mitigating simultaneous over/under-exposure; preview of 0.7.0 capabilities.
- **Simple lens calibration** — https://hugin.sourceforge.io/tutorials/calibration/ (`051`): derive lens distortion coefficients (a, b, c) using straight-line targets; notes built-in “Calibrate lens GUI”.
- **Transverse chromatic aberration correction** — https://hugin.sourceforge.io/tutorials/tca/ (`068`): estimate TCA parameters via channel control points and PTOptimizer; coefficients usable with fulla or PanoTools plugins.
- **Surveying buildings** — https://hugin.sourceforge.io/tutorials/surveying/ (`067`): advanced workflow combining Hugin and a 3D modeller to derive building geometry and camera pose from a single photo.
- **Tileable textures** — https://hugin.sourceforge.io/tutorials/tileable-textures/ (`069`): turn repeating patterns into seamless 1D/2D tiles by leveling images and stitching edges.

## Other linked tutorials/resources
- **Focus stacking macro with Enfuse** — http://blog.patdavid.net/2013/01/focus-stacking-macro-photos-enfuse.html (`005`): article (content not fully captured) describes Enfuse workflow for stacking macro shots at varied focus distances.
- **“Réalisez des panoramas en 3 clics !”** — http://apppaper.toile-libre.org/AppPaper/Index/Entries/2010/7/17_Hugin_1.html (`004`): French quick-start promising 3-click panorama assembly (page is minimal in capture).
- **Stereographic panorama with Hugin** — http://louprad.over-blog.com/article-tutoriel-realiser-une-photo-panoramique-stereographique-avec-hugin-104706365.html (`011`): French guide to creating stereographic/“little planet” panoramas with downloadable sample photos.
- **German single-row pano** — http://mawe-web.de/Tutorial/panorama.html (`012`): Windows-oriented German walkthrough for a fast single-row panorama from capture through stitching.
- **Lensfun project** — http://lensfun.berlios.de/ (`008`): lens correction database/software referenced for importing/exporting lens profiles.
- **PanoTools Wiki main/tutorials** — http://wiki.panotools.org/ (`018`) and http://wiki.panotools.org/Tutorials (`019`): community knowledge base; tutorials covering control-point creation, blending, HDR, and projection theory.
- **Linear panoramas** — http://www.dojoe.net/tutorials/linear-pano/ (`021`): Joachim Fenkes’ mural/linear panorama tutorial; shoot overlapping segments along a wall and stitch into a single straight projection.
- **John Houghton’s tutorials** — http://www.johnhpanos.com/tuts.htm (`023`): collection of PTGui/PTAssembler articles explaining optimizer behaviour and panorama theory applicable to Hugin.
- **LinuxFocus multi-language article (EN/ES/DE/FR/ID/IT/PT/TR)** — (`026`–`033`): “Creating panoramic views using Hugin, Enblend and The Gimp”; covers choosing source images, stitching in Hugin, blending with Enblend, and postwork in GIMP. Each file is a language translation of the same guide.
- **Flickriver pano gallery** — http://www.flickriver.com/photos/tags/hugin/interesting/ (`022`): photo inspiration rather than instructional content.
- **Carpebble “Using Hugin”** — https://sites.google.com/site/carpebble/home/360x180-panoramas/using-hugin (`077`): site navigation indicates resources for capturing/processing 360x180 panospheres; scraped page lacked detailed body.
- **Alister Ling – Enfuse only** — https://sites.google.com/site/alistargazing/home/image-processing/hugin-tutorial-enfuse-only (`075`): intended tutorial for using Enfuse without panorama steps; capture only shows site chrome.
- **Alister Ling – Time-lapse deshake** — https://sites.google.com/site/alistargazing/home/image-processing/time-lapse-deshake (`076`): aims to describe using Hugin/nona-deshake for stabilising time-lapse sequences; body not captured.
- **HDR/tone-map pano with Canon 5D fisheye** — http://www.lightspacewater.net/Tutorials/PhotoPano2/paper/ (not retrieved): not present in downloaded set.
- **Other external links (e.g., edu-perez HDR/focus stack, aloriel Spanish guides)**: these were missing/404 at scrape time, so no details captured.

## Usage notes
- Pages in non-English languages mirror the English tutorial content; use them for locale-specific phrasing/screens.
- Several Google Sites pages and some external links did not yield full bodies in the scrape; revisit live URLs if detailed steps are needed.

## Automation-focused takeaways (panorama building playbook)
- **Standard 2-photo / multi-row** (`070`, `057`): load images, auto Align, then stitch. For automation: always run a control-point generator (autopano-sift-C/CPFind), group images into rows if EXIF pitch hints are missing, and let optimizer refine yaw/pitch/roll plus lens params. Use Fast Preview cropping to auto-set output bounds.
- **Mixed lenses/cameras** (`055`): create per-lens image groups; optimize each group’s HFOV/distortion separately before global alignment. Keep EXIF where possible; otherwise seed HFOV from lens DB.
- **Architectural/perspective fixes** (`046`, `063`): add vertical/horizontal control points to enforce level lines; choose rectilinear projection. Automation hint: detect strong lines (Hough) and emit vertical CPs to stabilize optimizer for single-image rectification.
- **Flat scans / murals / mosaic** (`065`, `044`): set “mosaic” instead of “panorama” (no single projection center); allow position unlinked; blend with masks to omit occluders. Good for planar scenes shot from multiple viewpoints.
- **Exposure handling** (`049`, `045`, `052`): when EXIF shows bracketing stacks, auto-detect stacks and run “fused + blended” (enfuse then seam blend). For auto-exposed sequences, favor seam blending plus exposure fusion; fall back to Verdandi if Enblend not available. Flag mismatched exposures for operator review.
- **Blend masks** (`043`): allow optional include/exclude masks to avoid ghosting (moving people, parallax). In automation, surface mask UI only when multiple candidates overlap; default to automatic seams.
- **Lens calibration / TCA** (`051`, `068`): if no profile present, run quick line-based calibration to estimate distortion a/b/c. For TCA, derive channel control points and calculate coefficients for fulla; cache per-lens results in a DB (lensfun-compatible).
- **Tileable textures** (`069`): for pattern sources, straighten via vertical/horizontal CPs, then stitch edges; export square crops. Useful for generating seamless materials automatically.
- **Surveying/3D assist** (`067`): advanced single-photo pose reconstruction—low automation value unless adding a modelling step; keep as manual path.
- **UI/engine updates** (`054`, `059`): prefer `hugin_executor` pipeline; use Fast Preview CP editing APIs if exposed; Verdandi blender is built-in fallback; PTBatcherGUI shows thumbnails for batch runs.
- **Fallback knowledge bases** (`018`, `019`, LinuxFocus `026`–`033`): deep dives on control points, blending choices, and post-processing; use as reference when automatic steps fail.
