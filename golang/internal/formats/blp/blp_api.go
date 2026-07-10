package blp

import (
	"fmt"
	"log"
	"os"

	"github.com/pqhuy98/wow-converter/internal/ansi"
)

// ConvertItem describes one PNG/BLP2 -> BLP1 conversion task.
type ConvertItem struct {
	PNGPath  string
	PNG      []byte
	BLP2     []byte
	ResizeTo *Size
	BlpPath  string
}

// BlpDimensions holds width and height read from a BLP header.
type BlpDimensions struct {
	Width  uint32
	Height uint32
}

// ReadBlpSizeSync reads BLP1/BLP2 width and height from a file path.
func ReadBlpSizeSync(blpPath string) *BlpDimensions {
	f, err := os.Open(blpPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	header := make([]byte, 20)
	n, err := f.Read(header)
	if err != nil || n < 20 {
		return nil
	}

	magic := string(header[0:4])
	if magic != "BLP1" && magic != "BLP2" {
		return nil
	}

	width := uint32(header[12]) | uint32(header[13])<<8 | uint32(header[14])<<16 | uint32(header[15])<<24
	height := uint32(header[16]) | uint32(header[17])<<8 | uint32(header[18])<<16 | uint32(header[19])<<24
	return &BlpDimensions{Width: width, Height: height}
}

// PngsToBlps converts a batch of textures to BLP1 using the inline encoder path.
func PngsToBlps(items []ConvertItem) error {
	log.Printf("Converting %s textures to BLPs (inline, no workers)", ansi.Yellowf("%d", len(items)))
	for _, item := range items {
		input, err := resolveItemInput(item)
		if err != nil {
			return err
		}
		encode := EncodeInput{ResizeTo: item.ResizeTo}
		switch input.kind {
		case "png":
			encode.PNG = input.data
		case "blp2":
			encode.BLP2 = input.data
		}
		if err := ConvertTextureToBlp(encode, item.BlpPath); err != nil {
			return err
		}
	}
	return nil
}

type resolvedInput struct {
	data []byte
	kind string
}

func resolveItemInput(item ConvertItem) (resolvedInput, error) {
	if len(item.BLP2) > 0 {
		return resolvedInput{data: item.BLP2, kind: "blp2"}, nil
	}
	if len(item.PNG) > 0 {
		return resolvedInput{data: item.PNG, kind: "png"}, nil
	}
	if item.PNGPath != "" {
		data, err := os.ReadFile(item.PNGPath)
		if err != nil {
			return resolvedInput{}, err
		}
		return resolvedInput{data: data, kind: "png"}, nil
	}
	return resolvedInput{}, fmt.Errorf("BLP convert item has no input: %s", item.BlpPath)
}
