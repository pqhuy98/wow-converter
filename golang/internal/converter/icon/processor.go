package icon

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"

	pngfmt "github.com/pqhuy98/wow-converter/internal/formats/png"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

var frameCache sync.Map

func resolveFramePath(size Size, style Style, frame Frame) string {
	root := workspace.FindRepoRoot()
	sizeFolder := string(size)
	styleFolder := styleFolderMap[style]
	frameFile := frameFileMap[frame]
	switch frame {
	case FrameAtt, FrameUpg:
		return filepath.Join(root, "resources", "icon-frames", "custom_frames", sizeFolder, styleFolder, frameFile+".png")
	case FrameSSH, FrameSSP:
		return filepath.Join(root, "resources", "icon-frames", "custom_frames", "misc", frameFile+".png")
	default:
		return filepath.Join(root, "resources", "icon-frames", sizeFolder, styleFolder, frameFile+".png")
	}
}

func loadFrameImage(framePath string) ([]byte, error) {
	if _, err := os.Stat(framePath); err != nil {
		return nil, fmt.Errorf("frame image not found: %s", framePath)
	}
	return os.ReadFile(framePath)
}

func getCachedFrame(size Size, style Style, frame Frame) ([]byte, error) {
	key := fmt.Sprintf("%s-%s-%s", size, style, frame)
	if cached, ok := frameCache.Load(key); ok {
		return cached.([]byte), nil
	}
	data, err := loadFrameImage(resolveFramePath(size, style, frame))
	if err != nil {
		return nil, err
	}
	frameCache.Store(key, data)
	return data, nil
}

func optimalCropMargin(dim int, cropPercent float64) int {
	if cropPercent <= 0 || cropPercent >= 1 {
		return 0
	}
	target := float64(dim) * (cropPercent / 2)
	mFloor := int(math.Floor(target))
	mCeil := int(math.Ceil(target))
	if mFloor < 1 {
		mFloor = 1
	}
	if mCeil < 1 {
		mCeil = 1
	}
	if dim-2*mFloor <= 0 {
		mFloor = 1
	}
	if dim-2*mCeil <= 0 {
		mCeil = 1
	}
	errFloor := math.Abs(float64(dim-2*mFloor)/float64(dim) - (1 - cropPercent))
	errCeil := math.Abs(float64(dim-2*mCeil)/float64(dim) - (1 - cropPercent))
	if errFloor <= errCeil {
		return mFloor
	}
	return mCeil
}

func cropImage(data []byte, width, height int, cropPercent float64) ([]byte, int, int, error) {
	if cropPercent <= 0 || cropPercent >= 1 || width == 0 || height == 0 {
		return data, width, height, nil
	}
	marginW := optimalCropMargin(width, cropPercent)
	marginH := optimalCropMargin(height, cropPercent)
	newW := width - 2*marginW
	newH := height - 2*marginH
	if newW <= 0 || newH <= 0 {
		return data, width, height, nil
	}
	out := make([]byte, newW*newH*4)
	for y := 0; y < newH; y++ {
		srcY := y + marginH
		for x := 0; x < newW; x++ {
			srcX := x + marginW
			si := (srcY*width + srcX) * 4
			di := (y*newW + x) * 4
			copy(out[di:di+4], data[si:si+4])
		}
	}
	return out, newW, newH, nil
}

func applyDisabledFrameEffects(data []byte) {
	const saturation = 0.5
	const contrast = 0.82
	for i := 0; i < len(data); i += 4 {
		r := float64(data[i])
		g := float64(data[i+1])
		b := float64(data[i+2])
		gray := 0.299*r + 0.587*g + 0.114*b
		r = gray + (r-gray)*saturation
		g = gray + (g-gray)*saturation
		b = gray + (b-gray)*saturation
		data[i] = byte(clamp(r*contrast-(128*contrast)+128, 0, 255))
		data[i+1] = byte(clamp(g*contrast-(128*contrast)+128, 0, 255))
		data[i+2] = byte(clamp(b*contrast-(128*contrast)+128, 0, 255))
	}
}

func compositeOver(base, overlay []byte, baseW, baseH, left, top int) ([]byte, error) {
	overlayData, ow, oh, err := pngfmt.DecodeRGBA(overlay)
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), base...)
	for y := 0; y < oh; y++ {
		dy := top + y
		if dy < 0 || dy >= baseH {
			continue
		}
		for x := 0; x < ow; x++ {
			dx := left + x
			if dx < 0 || dx >= baseW {
				continue
			}
			si := (y*ow + x) * 4
			di := (dy*baseW + dx) * 4
			alpha := float64(overlayData[si+3]) / 255
			inv := 1 - alpha
			out[di] = byte(float64(out[di])*inv + float64(overlayData[si])*alpha)
			out[di+1] = byte(float64(out[di+1])*inv + float64(overlayData[si+1])*alpha)
			out[di+2] = byte(float64(out[di+2])*inv + float64(overlayData[si+2])*alpha)
			out[di+3] = byte(math.Min(255, float64(out[di+3])*inv+float64(overlayData[si+3])))
		}
	}
	return out, nil
}

func compositeFrameOnTop(imageData []byte, width, height int, frameData []byte, canvasW, canvasH int) ([]byte, error) {
	framePNG, err := pngfmt.ResizePngFill(frameData, canvasW, canvasH)
	if err != nil {
		return nil, err
	}
	return compositeOver(imageData, framePNG, width, height, 0, 0)
}

func effectiveSizeForOriginal(width, height int, frame Frame) Size {
	if frame == FrameNone {
		if width <= 64 && height <= 64 {
			return Size64
		}
		if width <= 128 && height <= 128 {
			return Size128
		}
		return Size256
	}
	imageSize := width
	if height < imageSize {
		imageSize = height
	}
	if imageSize <= 64 {
		return Size64
	}
	if imageSize <= 128 {
		return Size128
	}
	return Size256
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

// ProcessIconImage applies frames, styles, and extras to a PNG buffer.
func ProcessIconImage(inputPNG []byte, options MergedOptions) ([]byte, error) {
	data, width, height, err := pngfmt.DecodeRGBA(inputPNG)
	if err != nil {
		return nil, err
	}
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}

	var frameSize struct{ Width, Height int }
	var canvasSize struct{ Width, Height int }
	if options.Size == SizeOrig {
		if options.Frame == FrameNone {
			frameSize.Width, frameSize.Height = width, height
			canvasSize = frameSize
		} else {
			frameSize = sizeMapping[effectiveSizeForOriginal(width, height, options.Frame)]
			canvasSize = frameSize
		}
	} else {
		frameSize = sizeMapping[options.Size]
		canvasSize = frameSize
	}

	effectiveSize := options.Size
	if options.Size == SizeOrig {
		effectiveSize = effectiveSizeForOriginal(width, height, options.Frame)
	}

	custom := getCustomFrameData(options.Frame, effectiveSize, options.Style)
	targetW, targetH := frameSize.Width, frameSize.Height
	var customPos *[2]int
	if custom != nil {
		targetW, targetH = custom.Size[0], custom.Size[1]
		customPos = &custom.Pos
	}

	if options.Extras.Crop {
		var err error
		data, width, height, err = cropImage(data, width, height, 0.1)
		if err != nil {
			return nil, err
		}
	}

	if !(options.Size == SizeOrig && options.Frame == FrameNone) {
		pngData, err := pngfmt.EncodeRGBA(data, width, height)
		if err != nil {
			return nil, err
		}
		pngData, err = pngfmt.ResizePngFill(pngData, targetW, targetH)
		if err != nil {
			return nil, err
		}
		data, width, height, err = pngfmt.DecodeRGBA(pngData)
		if err != nil {
			return nil, err
		}
	}

	if options.Frame != FrameNone {
		frameData, err := getCachedFrame(effectiveSize, options.Style, options.Frame)
		if err != nil {
			return nil, err
		}
		if customPos != nil {
			canvas := make([]byte, canvasSize.Width*canvasSize.Height*4)
			imagePNG, err := pngfmt.EncodeRGBA(data, width, height)
			if err != nil {
				return nil, err
			}
			composited, err := compositeOver(canvas, imagePNG, canvasSize.Width, canvasSize.Height, customPos[0], customPos[1])
			if err != nil {
				return nil, err
			}
			data, err = compositeFrameOnTop(composited, canvasSize.Width, canvasSize.Height, frameData, canvasSize.Width, canvasSize.Height)
			if err != nil {
				return nil, err
			}
			width, height = canvasSize.Width, canvasSize.Height
		} else {
			imagePNG, err := pngfmt.EncodeRGBA(data, width, height)
			if err != nil {
				return nil, err
			}
			if width != canvasSize.Width || height != canvasSize.Height {
				imagePNG, err = pngfmt.ResizePngFill(imagePNG, canvasSize.Width, canvasSize.Height)
				if err != nil {
					return nil, err
				}
				data, width, height, err = pngfmt.DecodeRGBA(imagePNG)
				if err != nil {
					return nil, err
				}
			}
			data, err = compositeFrameOnTop(data, width, height, frameData, canvasSize.Width, canvasSize.Height)
			if err != nil {
				return nil, err
			}
			width, height = canvasSize.Width, canvasSize.Height
		}
	}

	if options.Style == StyleReforgedHD {
		if _, ok := hdDesaturationFrames[options.Frame]; ok {
			applyDisabledFrameEffects(data)
		}
	}

	pngfmt.RemoveAlphaRGB(data)
	return pngfmt.EncodeRGBA(data, width, height)
}
