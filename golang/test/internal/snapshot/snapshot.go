// Package snapshot provides directory snapshot, compare, and diff utilities
// ported from tests/compare-snapshots.ts.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// FileRecord holds size and SHA-256 for one file in a snapshot.
type FileRecord struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest is a directory snapshot with per-file hashes.
type Manifest struct {
	Root      string                `json:"root"`
	CreatedAt string                `json:"createdAt"`
	Files     map[string]FileRecord `json:"files"`
}

// PixelCompareResult is the outcome of a tolerance-based image compare.
type PixelCompareResult struct {
	OK          bool
	MaxDelta    int
	DiffPixels  int
	TotalPixels int
	Reason      string
}

// ToleranceEntry records a file that matched within pixel tolerance.
type ToleranceEntry struct {
	File       string `json:"file"`
	MaxDelta   int    `json:"maxDelta"`
	DiffPixels int    `json:"diffPixels"`
}

// DiffEntry records a file that differed beyond tolerance.
type DiffEntry struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// Summary aggregates a manifest-to-directory comparison.
type Summary struct {
	Identical         []string         `json:"identical"`
	WithinTolerance   []ToleranceEntry `json:"withinTolerance"`
	Different         []DiffEntry      `json:"different"`
	Missing           []string         `json:"missing"`
	Extra             []string         `json:"extra"`
	ToleranceRegex    string           `json:"toleranceRegex,omitempty"`
	MaxDeltaAllowed   int              `json:"maxDeltaAllowed,omitempty"`
}

// Pass reports whether the comparison found no missing or different files.
func (s Summary) Pass() bool {
	return len(s.Different) == 0 && len(s.Missing) == 0
}

// CompareOptions configures manifest comparison.
type CompareOptions struct {
	ToleranceRegex *regexp.Regexp
	MaxDelta       int
	BaselineDir    string
}

// Create walks dir and returns a snapshot manifest of all regular files.
func Create(dir string) (*Manifest, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}

	manifest := &Manifest{
		Root:      root,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Files:     map[string]FileRecord{},
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		rec, err := fileRecord(path)
		if err != nil {
			return err
		}
		manifest.Files[rel] = rec
		return nil
	})
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func fileRecord(path string) (FileRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileRecord{}, err
	}
	sum := sha256.Sum256(data)
	return FileRecord{
		Size:   int64(len(data)),
		SHA256: hex.EncodeToString(sum[:]),
	}, nil
}

// LoadManifest reads a snapshot manifest JSON file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Files == nil {
		manifest.Files = map[string]FileRecord{}
	}
	return &manifest, nil
}

// WriteManifest writes manifest JSON to path.
func WriteManifest(path string, manifest *Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// CompareManifestToDir compares baseline manifest files against targetDir.
func CompareManifestToDir(baseline *Manifest, targetDir string, opts CompareOptions) (*Summary, error) {
	maxDelta := opts.MaxDelta
	if maxDelta <= 0 {
		maxDelta = 2
	}

	root, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, err
	}

	targetFiles, err := listRelativeFiles(root)
	if err != nil {
		return nil, err
	}

	summary := &Summary{
		Identical:       []string{},
		WithinTolerance: []ToleranceEntry{},
		Different:       []DiffEntry{},
		Missing:         []string{},
		MaxDeltaAllowed: maxDelta,
	}
	if opts.ToleranceRegex != nil {
		summary.ToleranceRegex = opts.ToleranceRegex.String()
	}

	baselineDir := opts.BaselineDir
	if baselineDir == "" {
		baselineDir = baseline.Root
	}

	keys := make([]string, 0, len(baseline.Files))
	for rel := range baseline.Files {
		keys = append(keys, rel)
	}
	sort.Strings(keys)

	for _, rel := range keys {
		rec := baseline.Files[rel]
		if !targetFiles[rel] {
			summary.Missing = append(summary.Missing, rel)
			continue
		}
		delete(targetFiles, rel)

		targetPath := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(targetPath)
		if err != nil {
			summary.Different = append(summary.Different, DiffEntry{
				File:   rel,
				Reason: err.Error(),
			})
			continue
		}

		sum := sha256.Sum256(data)
		sha := hex.EncodeToString(sum[:])
		if sha == rec.SHA256 {
			summary.Identical = append(summary.Identical, rel)
			continue
		}

		if opts.ToleranceRegex != nil && opts.ToleranceRegex.MatchString(rel) {
			result, err := ComparePixels(filepath.Join(baselineDir, filepath.FromSlash(rel)), targetPath, maxDelta)
			if err != nil {
				summary.Different = append(summary.Different, DiffEntry{
					File:   rel,
					Reason: err.Error(),
				})
				continue
			}
			if result.OK {
				summary.WithinTolerance = append(summary.WithinTolerance, ToleranceEntry{
					File:       rel,
					MaxDelta:   result.MaxDelta,
					DiffPixels: result.DiffPixels,
				})
				continue
			}
			reason := result.Reason
			if reason == "" {
				reason = fmt.Sprintf("pixel maxDelta=%d > %d (%d/%d px differ)",
					result.MaxDelta, maxDelta, result.DiffPixels, result.TotalPixels)
			}
			summary.Different = append(summary.Different, DiffEntry{
				File:   rel,
				Reason: reason,
			})
			continue
		}

		summary.Different = append(summary.Different, DiffEntry{
			File:   rel,
			Reason: fmt.Sprintf("sha256 mismatch (size %d -> %d)", rec.Size, len(data)),
		})
	}

	for rel := range targetFiles {
		summary.Extra = append(summary.Extra, rel)
	}
	sort.Strings(summary.Extra)
	return summary, nil
}

func listRelativeFiles(root string) (map[string]bool, error) {
	files := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = true
		return nil
	})
	return files, err
}

type rgbaImage struct {
	data   []uint8
	width  int
	height int
}

// ComparePixels compares two image files with a per-channel max delta tolerance.
func ComparePixels(fileA, fileB string, maxDeltaAllowed int) (PixelCompareResult, error) {
	a, err := decodeToRGBA(fileA)
	if err != nil {
		return PixelCompareResult{
			OK: false, MaxDelta: int(^uint(0) >> 1), Reason: "failed to decode A: " + err.Error(),
		}, nil
	}
	b, err := decodeToRGBA(fileB)
	if err != nil {
		return PixelCompareResult{
			OK: false, MaxDelta: int(^uint(0) >> 1), Reason: "failed to decode B: " + err.Error(),
		}, nil
	}
	if a.width != b.width || a.height != b.height {
		return PixelCompareResult{
			OK:     false,
			Reason: fmt.Sprintf("dimension mismatch %dx%d vs %dx%d", a.width, a.height, b.width, b.height),
		}, nil
	}

	maxDelta := 0
	diffPixels := 0
	totalPixels := a.width * a.height
	for i := 0; i < len(a.data); i += 4 {
		pixelDiff := 0
		for c := 0; c < 4; c++ {
			d := int(a.data[i+c]) - int(b.data[i+c])
			if d < 0 {
				d = -d
			}
			if d > pixelDiff {
				pixelDiff = d
			}
		}
		if pixelDiff > 0 {
			diffPixels++
		}
		if pixelDiff > maxDelta {
			maxDelta = pixelDiff
		}
	}

	return PixelCompareResult{
		OK:          maxDelta <= maxDeltaAllowed,
		MaxDelta:    maxDelta,
		DiffPixels:  diffPixels,
		TotalPixels: totalPixels,
	}, nil
}

func decodeToRGBA(path string) (*rgbaImage, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".blp") {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(data) >= 4 && string(data[:4]) == "BLP1" {
			return decodeBLP1(data)
		}
		return nil, fmt.Errorf("BLP2 decode not supported in Go harness")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	out := make([]uint8, width*height*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			i := (y-bounds.Min.Y)*width*4 + (x-bounds.Min.X)*4
			out[i] = uint8(r >> 8)
			out[i+1] = uint8(g >> 8)
			out[i+2] = uint8(b >> 8)
			out[i+3] = uint8(a >> 8)
		}
	}
	return &rgbaImage{data: out, width: width, height: height}, nil
}

func decodeBLP1(buf []byte) (*rgbaImage, error) {
	if len(buf) < 160 {
		return nil, fmt.Errorf("BLP1 too small")
	}
	content := uint32(buf[4]) | uint32(buf[5])<<8 | uint32(buf[6])<<16 | uint32(buf[7])<<24
	if content != 1 {
		return nil, fmt.Errorf("unsupported BLP1 content type %d", content)
	}
	width := int(uint32(buf[12]) | uint32(buf[13])<<8 | uint32(buf[14])<<16 | uint32(buf[15])<<24)
	height := int(uint32(buf[16]) | uint32(buf[17])<<8 | uint32(buf[18])<<16 | uint32(buf[19])<<24)
	mip0Offset := int(uint32(buf[28]) | uint32(buf[29])<<8 | uint32(buf[30])<<16 | uint32(buf[31])<<24)
	paletteOffset := 156
	pixelCount := width * height
	if mip0Offset+pixelCount*2 > len(buf) {
		return nil, fmt.Errorf("BLP1 mip data out of range")
	}

	out := make([]uint8, pixelCount*4)
	for i := 0; i < pixelCount; i++ {
		idx := int(buf[mip0Offset+i])
		p := paletteOffset + idx*4
		if p+3 >= len(buf) {
			return nil, fmt.Errorf("BLP1 palette out of range")
		}
		out[i*4] = buf[p+2]
		out[i*4+1] = buf[p+1]
		out[i*4+2] = buf[p]
		out[i*4+3] = buf[mip0Offset+pixelCount+i]
	}
	return &rgbaImage{data: out, width: width, height: height}, nil
}

// PrintSummary writes a human-readable comparison summary to w.
func PrintSummary(w io.Writer, summary *Summary) bool {
	fmt.Fprintf(w, "Identical: %d\n", len(summary.Identical))
	if len(summary.WithinTolerance) > 0 {
		fmt.Fprintf(w, "Within tolerance: %d\n", len(summary.WithinTolerance))
		for _, t := range summary.WithinTolerance {
			fmt.Fprintf(w, "  ~ %s (maxDelta=%d, diffPixels=%d)\n", t.File, t.MaxDelta, t.DiffPixels)
		}
	}
	for _, m := range summary.Missing {
		fmt.Fprintf(w, "  MISSING: %s\n", m)
	}
	for _, e := range summary.Extra {
		fmt.Fprintf(w, "  EXTRA: %s\n", e)
	}
	for _, d := range summary.Different {
		fmt.Fprintf(w, "  DIFF: %s — %s\n", d.File, d.Reason)
	}
	if summary.Pass() {
		fmt.Fprintln(w, "RESULT: PASS")
	} else {
		fmt.Fprintln(w, "RESULT: FAIL")
	}
	return summary.Pass()
}

// IsPlaceholderSHA reports whether a golden hash is unset.
func IsPlaceholderSHA(sha string) bool {
	sha = strings.TrimSpace(strings.ToLower(sha))
	return sha == "" || sha == "placeholder" || sha == "todo" || strings.HasPrefix(sha, "00000000")
}
