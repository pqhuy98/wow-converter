package reportshot

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// RunCLI keeps the two go-run entry points on the server's capture implementation.
func RunCLI(source string) {
	if err := runCLI(source, os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type shotOptions struct {
	request  Request
	indices  []int
	out      string
	sheet    string
	aimSheet string
	cameras  string
}

func runCLI(source string, args []string) error {
	opts, err := parseShotArgs(source, args, os.Stderr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	var frames [2][][]byte
	sequence := opts.request.Sequence
	if opts.request.Model != "" && opts.request.WowheadURL != "" {
		frames, sequence, err = captureFrames(ctx, opts.request, opts.indices)
	} else {
		b, openErr := openBrowser(ctx)
		if openErr != nil {
			return openErr
		}
		defer b.close()
		if err = prepareViewport(ctx, b); err != nil {
			return err
		}
		if err = b.setCacheDisabled(ctx, true); err != nil {
			return err
		}
		if source == "converter" {
			if err = loadShotCameras(&opts); err != nil {
				return err
			}
			frames[1], sequence, err = captureConverter(ctx, b, opts.request, opts.indices)
		} else {
			if err = loadWowheadAims(&opts); err != nil {
				return err
			}
			var cams []cameraReference
			if err = prepareWowheadPage(ctx, b, opts.request); err == nil {
				frames[0], cams, err = shootWowheadViews(ctx, b, opts.request, opts.indices)
			}
			if err == nil && opts.cameras != "" {
				err = writeShotCameras(opts.cameras, opts.indices, cams)
			}
		}
	}
	if err != nil {
		return err
	}
	if opts.sheet != "" {
		if err = writeSheet(opts.sheet, pickSheetFrames(source, frames)); err != nil {
			return err
		}
		if sequence != "" {
			fmt.Println("sequence " + sequence)
		}
		return nil
	}
	if err = os.MkdirAll(opts.out, 0755); err != nil {
		return err
	}
	names := [2]string{wowheadSlug(opts.request.WowheadURL), strings.TrimSuffix(filepath.Base(opts.request.Model), filepath.Ext(opts.request.Model))}
	sequences := [2]string{opts.request.WowSequence, sequence}
	for source, shots := range frames {
		for i, raw := range shots {
			view := views[opts.indices[i]]
			img, err := renderFrame(raw, 1440, 900, view)
			if err != nil {
				return err
			}
			data, err := encodePNG(img)
			if err != nil {
				return err
			}
			name := fmt.Sprintf("%s-%s-%s.png", names[source], safeSequence(sequences[source]), view)
			if opts.request.Model != "" && opts.request.WowheadURL != "" {
				name = strings.TrimSuffix(name, ".png") + "-" + []string{"wowhead", "converter"}[source] + ".png"
			}
			path := filepath.Join(opts.out, name)
			if err = os.WriteFile(path, data, 0644); err != nil {
				return err
			}
			absolute, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			fmt.Println(absolute)
		}
	}
	return nil
}

func parseShotArgs(source string, args []string, output io.Writer) (shotOptions, error) {
	var opts shotOptions
	var sequence, view string
	flags := flag.NewFlagSet("shot-"+source, flag.ContinueOnError)
	flags.SetOutput(output)
	base := os.Getenv("WOW_CONVERTER_URL")
	if base == "" {
		base = "http://127.0.0.1:3001"
	}
	flags.StringVar(&opts.request.BaseURL, "base", base, "converter server URL")
	flags.StringVar(&sequence, "seq", "Stand", "animation name (prefix accepted)")
	flags.StringVar(&view, "view", "", "one view or comma-separated views; default all six")
	flags.StringVar(&opts.out, "out", "", "output directory")
	flags.StringVar(&opts.sheet, "sheet", "", "write a 1920x800 2x3 sheet PNG instead of per-view files")
	flags.StringVar(&opts.aimSheet, "aim-sheet", "", "1920x800 converter sheet whose side-view silhouettes aim Wowhead")
	flags.StringVar(&opts.cameras, "cameras", "", "JSON camera map written by Wowhead and read by the converter")
	flags.StringVar(&opts.request.Model, "model", "", "exported model for a paired Wowhead shot")
	flags.StringVar(&opts.request.WowheadURL, "wowhead", "", "Wowhead URL for a paired converter shot")
	flags.StringVar(&opts.request.WowSequence, "wow-seq", "Stand", "Wowhead animation for a paired converter shot")
	flags.StringVar(&opts.request.Sequence, "converter-seq", "Stand", "WC3 animation for a paired Wowhead shot")
	flags.IntVar(&opts.request.WowVariant, "variant", 0, "Wowhead animation variant")
	flags.Float64Var(&opts.request.ModelScale, "model-scale", 0, "exported units per source unit (default 56); use export metadata for resized models")
	flags.Usage = func() {
		fmt.Fprintf(output, "usage: go run ./cmd/shot-%s <model-or-url> [--seq Stand] [--view front] [--out dir]\n", source)
		flags.PrintDefaults()
	}
	// flag stops at the first positional argument; existing shot commands allow flags after it.
	var options, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		options = append(options, arg)
		if arg != "-h" && arg != "--help" && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	if err := flags.Parse(options); err != nil {
		return opts, err
	}
	if len(positional) == 0 || len(positional) > 2 || (source == "converter" && len(positional) > 1) {
		flags.Usage()
		return opts, errors.New("a model path or Wowhead URL is required")
	}
	if source == "converter" {
		opts.request.Model, opts.request.Sequence = positional[0], sequence
	} else {
		opts.request.WowheadURL, opts.request.WowSequence = positional[0], sequence
	}
	if len(positional) == 2 && opts.out == "" {
		opts.out = positional[1]
	}
	if opts.out == "" {
		opts.out = workspace.ResolveRepoPath("tmp/shots")
	}
	opts.out = workspace.ResolveRepoPath(opts.out)
	if opts.aimSheet != "" {
		opts.aimSheet = workspace.ResolveRepoPath(opts.aimSheet)
	}
	if opts.cameras != "" {
		opts.cameras = workspace.ResolveRepoPath(opts.cameras)
	}
	opts.request.Model = assetPath(opts.request.Model)
	opts.request.BaseURL = strings.TrimRight(opts.request.BaseURL, "/")
	opts.request.sequencePrefix = true
	if opts.sheet != "" {
		opts.sheet = workspace.ResolveRepoPath(opts.sheet)
		opts.indices = []int{0, 1, 2, 3, 4, 5}
	} else if view == "" {
		opts.indices = []int{0, 1, 2, 3, 4, 5}
	} else {
		for _, name := range strings.Split(view, ",") {
			index := slices.Index(views, strings.TrimSpace(name))
			if index < 0 || slices.Contains(opts.indices, index) {
				return opts, fmt.Errorf("invalid or duplicate view %q; use %s", name, strings.Join(views, ", "))
			}
			opts.indices = append(opts.indices, index)
		}
	}
	if opts.request.WowVariant < 0 {
		return opts, errors.New("variant must be nonnegative")
	}
	if math.IsNaN(opts.request.ModelScale) || math.IsInf(opts.request.ModelScale, 0) || opts.request.ModelScale < 0 {
		return opts, errors.New("model-scale must be finite and nonnegative")
	}
	return opts, nil
}

func pickSheetFrames(source string, frames [2][][]byte) [][]byte {
	if source == "converter" {
		return frames[1]
	}
	return frames[0]
}

func writeShotCameras(dest string, indices []int, cams []cameraReference) error {
	out := make(map[string]cameraReference, len(indices))
	for i, index := range indices {
		if i >= len(cams) {
			break
		}
		out[views[index]] = cams[i]
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, append(data, '\n'), 0644)
}

func loadShotCameras(opts *shotOptions) error {
	if opts.cameras == "" {
		return nil
	}
	data, err := os.ReadFile(opts.cameras)
	if err != nil {
		return err
	}
	var byView map[string]cameraReference
	if err = json.Unmarshal(data, &byView); err != nil {
		return err
	}
	opts.request.cameras = make([]chan cameraReference, len(views))
	for _, index := range opts.indices {
		cam, ok := byView[views[index]]
		if !ok {
			continue
		}
		if opts.request.ModelScale > 0 {
			cam.ModelScale = opts.request.ModelScale
		}
		opts.request.cameras[index] = make(chan cameraReference, 1)
		opts.request.cameras[index] <- cam
	}
	return nil
}

func writeSheet(dest string, frames [][]byte) error {
	data, err := composeSheet(frames, false)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(dest, data, 0644); err != nil {
		return err
	}
	absolute, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	fmt.Println(absolute)
	return nil
}

func assetPath(input string) string {
	input = strings.ReplaceAll(input, `\`, "/")
	if at := strings.Index(strings.ToLower(input), "exported-assets/"); at >= 0 {
		return input[at+len("exported-assets/"):]
	}
	return strings.TrimPrefix(input, "./")
}

func wowheadSlug(raw string) string {
	page, err := url.Parse(raw)
	if err != nil || page.Path == "" {
		return "model"
	}
	slug := filepath.Base(strings.TrimRight(page.Path, "/"))
	if at := strings.Index(slug, "="); at >= 0 {
		slug = slug[at+1:]
	}
	return slug
}

func safeSequence(sequence string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, sequence)
}

func loadWowheadAims(opts *shotOptions) error {
	if opts.aimSheet != "" {
		aims, err := loadAimsFromSheet(opts.aimSheet, opts.indices)
		if err != nil {
			return err
		}
		opts.request.aims = aims
		return nil
	}
	return loadConverterAims(opts)
}

func loadAimsFromSheet(path string, indices []int) ([]chan frameAim, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 800 {
		return nil, fmt.Errorf("aim sheet must be 1920x800, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	aims := make([]chan frameAim, len(views))
	for _, index := range indices {
		if index < 0 || index >= 4 {
			continue
		}
		origin := image.Pt(bounds.Min.X+(index%3)*640, bounds.Min.Y+(index/3)*400)
		tile := image.NewRGBA(image.Rect(0, 0, 640, 400))
		draw.Draw(tile, tile.Bounds(), img, origin, draw.Src)
		raw, err := encodePNG(tile)
		if err != nil {
			return nil, err
		}
		aim, err := measureFrame(raw)
		if err != nil {
			return nil, fmt.Errorf("aim sheet %s: %w", views[index], err)
		}
		aims[index] = make(chan frameAim, 1)
		aims[index] <- aim
	}
	return aims, nil
}

// Preserve standalone Wowhead alignment to existing comparison-folder converter shots.
func loadConverterAims(opts *shotOptions) error {
	opts.request.aims = make([]chan frameAim, len(views))
	for _, index := range opts.indices {
		if index >= 4 {
			continue
		}
		ordinal := []int{1, 3, 2, 4}[index]
		name := wowheadSlug(opts.request.WowheadURL) + "-" + opts.request.WowSequence + "-0" + strconv.Itoa(ordinal) + "-" + views[index] + "-converter.png"
		file, err := os.Open(filepath.Join(opts.out, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		img, err := png.Decode(file)
		file.Close()
		if err != nil {
			return err
		}
		unlabeled := image.NewRGBA(img.Bounds())
		draw.Draw(unlabeled, img.Bounds(), img, img.Bounds().Min, draw.Src)
		draw.Draw(unlabeled, image.Rect(0, 0, 64, 26), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
		raw, err := encodePNG(unlabeled)
		if err != nil {
			return err
		}
		aim, err := measureFrame(raw)
		if err != nil {
			return err
		}
		opts.request.aims[index] = make(chan frameAim, 1)
		opts.request.aims[index] <- aim
	}
	return nil
}
