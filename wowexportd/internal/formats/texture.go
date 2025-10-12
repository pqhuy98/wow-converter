package formats

import (
	"errors"
	"wowexportd/internal/casc"
	"wowexportd/internal/fileio"
)

// ExportTexturePNG loads a BLP by FileDataID via the active CASC source and writes
// a PNG to the given absolute path. Channel mask matches wow.export semantics:
// bit 0=R, 1=G, 2=B, 3=A. Currently supports BLP type 1 encodings as implemented
// in wowexportd/internal/casc/blp.go-equivalent logic below.
func ExportTexturePNG(src casc.Source, fileDataID uint32, exportPath string, channelMask uint8) error {
	if src == nil || !src.IsLoaded() {
		return errors.New("ERR_NO_CASC")
	}
	ekey, err := src.ResolveEncodingKeyByFileID(fileDataID)
	if err != nil {
		return err
	}
	raw, err := src.GetDataByEncodingKey(ekey)
	if err != nil {
		return err
	}
	img, err := NewBLP(raw)
	if err != nil {
		return err
	}
	w := fileio.NewPNGWriter(int(img.scaledWidth), int(img.scaledHeight))
	pixels := w.PixelData()
	switch img.encoding {
	case 1:
		img.writeUncompressed(pixels, channelMask)
	case 2:
		img.writeCompressed(pixels, channelMask)
	case 3:
		img.marshalBGRA(pixels, channelMask)
	default:
		return errors.New("unsupported BLP encoding")
	}
	buf := w.Buffer()
	return fileio.WriteFileAtomic(exportPath, buf, true)
}
