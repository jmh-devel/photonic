package cli

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"photonic/internal/config"
	"photonic/internal/pipeline"
	"photonic/internal/storage"
	"photonic/internal/tasks"
)

func TestRunDispatchesProcessingCommands(t *testing.T) {
	root, fakePipe, _, _ := newTestRoot(t)
	temp := t.TempDir()

	cases := []struct {
		name       string
		args       []string
		expectType pipeline.JobType
	}{
		{"scan", []string{"scan", temp}, pipeline.JobScan},
		{"timelapse", []string{"timelapse", temp, "--fps", "12", "--resolution", "720p", "--format", "gif", "--tool", "ffmpeg"}, pipeline.JobTimelapse},
		{"panoramic", []string{"panoramic", temp, "--quality", "ultra", "--auto-exposure"}, pipeline.JobPanoramic},
		{"stack", []string{"stack", temp, "--method", "median", "--sigma-low", "1.1", "--astro"}, pipeline.JobStack},
		{"batch", []string{"batch", temp, "--output", filepath.Join(temp, "outdir")}, pipeline.JobScan},
		{"align", []string{"align", filepath.Join(temp, "a.jpg"), filepath.Join(temp, "b.jpg")}, pipeline.JobAlign},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakePipe.reset()
			if err := root.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("run failed: %v", err)
			}
			if len(fakePipe.jobs) != 1 {
				t.Fatalf("expected one job, got %d", len(fakePipe.jobs))
			}
			if fakePipe.jobs[0].Type != tc.expectType {
				t.Fatalf("expected type %s, got %s", tc.expectType, fakePipe.jobs[0].Type)
			}
		})
	}
}

func TestRunValidatesArguments(t *testing.T) {
	root, _, _, _ := newTestRoot(t)
	if err := root.Run(context.Background(), []string{"scan"}); err == nil {
		t.Fatalf("expected error for missing scan input")
	}
	if err := root.Run(context.Background(), []string{"align", "only-one"}); err == nil {
		t.Fatalf("expected error for insufficient align args")
	}
	if err := root.Run(context.Background(), []string{}); err != nil {
		t.Fatalf("expected nil for empty args showing usage, got %v", err)
	}
}

func TestRawCommandUsesProcessors(t *testing.T) {
	root, _, rawMgr, _ := newTestRoot(t)
	rawMgr.detect = []string{"stub"}
	proc := &stubProcessor{name: "stub", available: true}
	rawMgr.processors["stub"] = proc

	fileA := filepath.Join(t.TempDir(), "a.cr2")
	fileB := filepath.Join(t.TempDir(), "b.cr2")
	touch(t, fileA)
	touch(t, fileB)

	outDir := filepath.Join(t.TempDir(), "out")
	args := []string{"raw", "--output", outDir, "--format", "png", "--tool", "stub", fileA, fileB}

	if err := root.Run(context.Background(), args); err != nil {
		t.Fatalf("raw command failed: %v", err)
	}
	if proc.convertCalls != 2 {
		t.Fatalf("expected 2 conversions, got %d", proc.convertCalls)
	}
	for _, name := range []string{"a.png", "b.png"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("expected output file %s: %v", name, err)
		}
	}
}

func TestRawCommandFallbacks(t *testing.T) {
	root, _, rawMgr, _ := newTestRoot(t)
	rawMgr.detect = []string{"stub"}
	rawMgr.fallbackAvailable = true

	fileA := filepath.Join(t.TempDir(), "a.cr2")
	touch(t, fileA)

	if err := root.Run(context.Background(), []string{"raw", fileA}); err != nil {
		t.Fatalf("raw fallback failed: %v", err)
	}
	if rawMgr.fallbackCalls != 1 {
		t.Fatalf("expected ConvertWithFallback to run once, got %d", rawMgr.fallbackCalls)
	}
}

func TestListProcessorsUsesFactory(t *testing.T) {
	root, _, rawMgr, _ := newTestRoot(t)
	rawMgr.detect = []string{"stub-one", "stub-two"}
	rawMgr.processors["stub-one"] = &stubProcessor{name: "stub-one", available: true}
	rawMgr.processors["stub-two"] = &stubProcessor{name: "stub-two", available: true}

	output := captureOutput(t, func() {
		if err := root.cmdListProcessors(context.Background()); err != nil {
			t.Fatalf("cmdListProcessors failed: %v", err)
		}
	})
	for _, name := range rawMgr.detect {
		if !strings.Contains(output, name) {
			t.Fatalf("expected processor %s listed in output %q", name, output)
		}
	}
}

func TestToolStatusRespectsPath(t *testing.T) {
	root, _, rawMgr, _ := newTestRoot(t)
	rawMgr.detect = []string{"stub"}
	rawMgr.processors["stub"] = &stubProcessor{name: "stub", available: true}

	pathDir := t.TempDir()
	for _, name := range []string{"ffmpeg", "hugin", "enblend", "enfuse", "align_image_stack", "mencoder", "avconv", "siril", "ale", "pto_gen", "nona", "cpfind"} {
		createExecutable(t, pathDir, name)
	}
	prev := os.Getenv("PATH")
	t.Setenv("PATH", pathDir+string(os.PathListSeparator)+prev)

	output := captureOutput(t, func() {
		if err := root.cmdToolStatus(context.Background()); err != nil {
			t.Fatalf("cmdToolStatus failed: %v", err)
		}
	})
	if !strings.Contains(output, "Tool Availability Status") {
		t.Fatalf("expected status header in output")
	}
	if !strings.Contains(output, "ffmpeg") {
		t.Fatalf("expected ffmpeg availability in output")
	}
}

func TestToolsCommandUsesManager(t *testing.T) {
	root, _, _, toolMgr := newTestRoot(t)
	toolMgr.status = map[string]map[string]tasks.ToolStatus{
		"raw":       {"imagemagick": {Available: true, Version: "1.0", Path: "/bin/convert"}},
		"panoramic": {"hugin": {Available: true, Version: "2.0"}},
		"stacking":  {"ale": {Available: false, Error: io.EOF}},
		"timelapse": {"ffmpeg": {Available: true}},
		"alignment": {"align_image_stack": {Available: true}},
	}
	toolMgr.picks = map[string]string{
		"raw":       "imagemagick",
		"panoramic": "hugin",
		"stacking":  "",
		"timelapse": "ffmpeg",
		"alignment": "align_image_stack",
	}

	output := captureOutput(t, func() {
		if err := root.cmdTools(context.Background(), []string{"--verbose"}); err != nil {
			t.Fatalf("cmdTools failed: %v", err)
		}
	})
	if !strings.Contains(output, "Photonic Tool Status Report") {
		t.Fatalf("expected header in output")
	}
	if !strings.Contains(output, "RAW Processing: imagemagick") {
		t.Fatalf("expected recommended raw tool in output: %q", output)
	}
}

func TestServeCommandUsesInjectedFunction(t *testing.T) {
	root, _, _, _ := newTestRoot(t)
	var called bool
	root.serveFn = func(ctx context.Context, addr string, store *storage.Store, pipe pipelineClient, log *slog.Logger) error {
		called = true
		if addr != ":9999" {
			t.Fatalf("unexpected addr %s", addr)
		}
		return nil
	}
	if err := root.cmdServe(context.Background(), []string{"--addr", ":9999"}); err != nil {
		t.Fatalf("cmdServe failed: %v", err)
	}
	if !called {
		t.Fatalf("serve function was not invoked")
	}
}

func TestConfigCommands(t *testing.T) {
	root, _, rawMgr, _ := newTestRoot(t)
	rawMgr.detect = []string{"stub"}
	rawMgr.processors["stub"] = &stubProcessor{name: "stub", available: true}

	showOut := captureOutput(t, func() {
		if err := root.configShow(); err != nil {
			t.Fatalf("configShow failed: %v", err)
		}
	})
	if !strings.Contains(showOut, "Current configuration") {
		t.Fatalf("expected configuration output, got %q", showOut)
	}

	testOut := captureOutput(t, func() {
		if err := root.configTestProcessors(context.Background()); err != nil {
			t.Fatalf("configTestProcessors failed: %v", err)
		}
	})
	if !strings.Contains(testOut, "stub") {
		t.Fatalf("expected stub processor listed, got %q", testOut)
	}

	versionOut := captureOutput(t, func() {
		if err := root.cmdVersion(); err != nil {
			t.Fatalf("cmdVersion failed: %v", err)
		}
	})
	if !strings.Contains(versionOut, "Photonic v1.0.0-dev") {
		t.Fatalf("expected version string, got %q", versionOut)
	}
}

func TestEnqueueAndWaitPropagatesErrors(t *testing.T) {
	root, fakePipe, _, _ := newTestRoot(t)
	job := pipeline.Job{ID: "err-job", Type: pipeline.JobScan}
	fakePipe.jobErrors["err-job"] = context.DeadlineExceeded
	if err := root.enqueueAndWait(context.Background(), job); err == nil {
		t.Fatalf("expected error from pipeline result")
	}
}

// Test helpers

func newTestRoot(t *testing.T) (*Root, *fakePipeline, *stubRawManager, *stubToolManager) {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	tmp := t.TempDir()
	cfg.Paths.DefaultOutput = filepath.Join(tmp, "output")
	cfg.Paths.DatabasePath = filepath.Join(tmp, "photonic.db")
	cfg.Raw.TempDir = filepath.Join(tmp, "raw-temp")

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pipe := newFakePipeline()
	rawMgr := newStubRawManager(cfg)
	toolMgr := &stubToolManager{}

	root := &Root{
		pipeline:   pipe,
		cfg:        cfg,
		log:        logger,
		store:      nil,
		rawFactory: func(*config.RawProcessing) rawManager { return rawMgr },
		toolFactory: func(*config.Config) toolManager {
			return toolMgr
		},
		serveFn: defaultServe,
	}
	return root, pipe, rawMgr, toolMgr
}

type fakePipeline struct {
	mu        sync.Mutex
	jobs      []pipeline.Job
	subs      map[int]chan pipeline.Result
	nextSubID int
	jobErrors map[string]error
}

func newFakePipeline() *fakePipeline {
	return &fakePipeline{
		subs:      make(map[int]chan pipeline.Result),
		jobErrors: make(map[string]error),
	}
}

func (f *fakePipeline) Submit(job pipeline.Job) error {
	f.mu.Lock()
	f.jobs = append(f.jobs, job)
	subs := make([]chan pipeline.Result, 0, len(f.subs))
	for _, ch := range f.subs {
		subs = append(subs, ch)
	}
	err := f.errorFor(job)
	f.mu.Unlock()

	go func() {
		res := pipeline.Result{Job: job, Error: err, Meta: map[string]any{"ok": true}}
		for _, ch := range subs {
			ch <- res
		}
	}()
	return nil
}

func (f *fakePipeline) Subscribe() (<-chan pipeline.Result, func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextSubID
	f.nextSubID++
	ch := make(chan pipeline.Result, 2)
	f.subs[id] = ch
	unsub := func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if c, ok := f.subs[id]; ok {
			close(c)
			delete(f.subs, id)
		}
	}
	return ch, unsub
}

func (f *fakePipeline) errorFor(job pipeline.Job) error {
	if err, ok := f.jobErrors[job.ID]; ok {
		return err
	}
	if err, ok := f.jobErrors[string(job.Type)]; ok {
		return err
	}
	return nil
}

func (f *fakePipeline) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = nil
	f.jobErrors = make(map[string]error)
}

type stubRawManager struct {
	cfg               *config.RawProcessing
	processors        map[string]*stubProcessor
	detect            []string
	fallbackAvailable bool
	fallbackCalls     int
}

func newStubRawManager(cfg *config.Config) *stubRawManager {
	return &stubRawManager{
		cfg:        &cfg.Raw,
		processors: make(map[string]*stubProcessor),
	}
}

func (m *stubRawManager) DetectAvailable() []string {
	if m.detect != nil {
		return m.detect
	}
	var names []string
	for name, proc := range m.processors {
		if proc.available {
			names = append(names, name)
		}
	}
	return names
}

func (m *stubRawManager) Processors() map[string]tasks.RawProcessor {
	res := make(map[string]tasks.RawProcessor, len(m.processors))
	for k, v := range m.processors {
		res[k] = v
	}
	return res
}

func (m *stubRawManager) GetProcessor(name string) tasks.RawProcessor {
	return m.processors[name]
}

func (m *stubRawManager) ConvertWithFallback(ctx context.Context, inputFile, xmpFile, outputDir string) (tasks.RawConvertResult, error) {
	m.fallbackCalls++
	if !m.fallbackAvailable {
		return tasks.RawConvertResult{}, context.DeadlineExceeded
	}
	out := m.OutputPath(inputFile, outputDir)
	_ = os.MkdirAll(filepath.Dir(out), 0o755)
	_ = os.WriteFile(out, []byte("ok"), 0o644)
	return tasks.RawConvertResult{InputFile: inputFile, OutputFile: out, ToolUsed: "fallback", Success: true}, nil
}

func (m *stubRawManager) OutputPath(input string, outputDir string) string {
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	ext := m.cfg.OutputFormat
	if ext == "" {
		ext = "jpg"
	}
	if outputDir == "" {
		return filepath.Join(filepath.Dir(input), base+"."+ext)
	}
	return filepath.Join(outputDir, base+"."+ext)
}

type stubProcessor struct {
	name         string
	available    bool
	convertErr   error
	batchErr     error
	convertCalls int
	lastReq      tasks.RawConvertRequest
}

func (p *stubProcessor) Name() string { return p.name }

func (p *stubProcessor) IsAvailable() bool { return p.available }

func (p *stubProcessor) Convert(ctx context.Context, req tasks.RawConvertRequest) (tasks.RawConvertResult, error) {
	p.convertCalls++
	p.lastReq = req
	_ = os.MkdirAll(filepath.Dir(req.OutputFile), 0o755)
	_ = os.WriteFile(req.OutputFile, []byte("converted"), 0o644)
	if p.convertErr != nil {
		return tasks.RawConvertResult{InputFile: req.InputFile, OutputFile: req.OutputFile, ToolUsed: p.name, Success: false, Error: p.convertErr}, p.convertErr
	}
	return tasks.RawConvertResult{InputFile: req.InputFile, OutputFile: req.OutputFile, ToolUsed: p.name, Success: true}, nil
}

func (p *stubProcessor) BatchConvert(ctx context.Context, files []string, outputDir string) ([]string, error) {
	if p.batchErr != nil {
		return nil, p.batchErr
	}
	var outputs []string
	for _, f := range files {
		out := filepath.Join(outputDir, filepath.Base(f)+".jpg")
		_ = os.MkdirAll(filepath.Dir(out), 0o755)
		_ = os.WriteFile(out, []byte("batch"), 0o644)
		outputs = append(outputs, out)
	}
	return outputs, nil
}

type stubToolManager struct {
	status map[string]map[string]tasks.ToolStatus
	picks  map[string]string
}

func (m *stubToolManager) GetToolStatus() map[string]map[string]tasks.ToolStatus {
	if m.status != nil {
		return m.status
	}
	return map[string]map[string]tasks.ToolStatus{}
}

func (m *stubToolManager) GetAvailableRAWTool() (string, error) {
	return m.pick("raw")
}

func (m *stubToolManager) GetAvailablePanoramicTool() (string, error) {
	return m.pick("panoramic")
}

func (m *stubToolManager) GetAvailableStackingTool() (string, error) {
	return m.pick("stacking")
}

func (m *stubToolManager) GetAvailableTimelapseTool() (string, error) {
	return m.pick("timelapse")
}

func (m *stubToolManager) GetAvailableAlignmentTool() (string, error) {
	return m.pick("alignment")
}

func (m *stubToolManager) pick(key string) (string, error) {
	if m.picks == nil {
		return "", os.ErrNotExist
	}
	if val := m.picks[key]; val != "" {
		return val, nil
	}
	return "", os.ErrNotExist
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func touch(t *testing.T, path string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to touch %s: %v", path, err)
	}
}

func createExecutable(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to create stub executable %s: %v", path, err)
	}
}
