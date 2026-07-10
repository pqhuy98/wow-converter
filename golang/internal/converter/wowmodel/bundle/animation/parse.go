package animation

import (
	"encoding/json"
	"os"

	"github.com/pqhuy98/wow-converter/internal/converter/convertlog"
)

type bonesJSON struct {
	Bones        []BoneData        `json:"bones"`
	Animations   []AnimationData   `json:"animations"`
	BoneWeights  []float64         `json:"boneWeights"`
	BoneIndices  []float64         `json:"boneIndices"`
	Attachments  []AttachmentData  `json:"attachments"`
}

// Parse reads _bones.json from disk when present.
func (f *File) Parse() error {
	convertlog.Loading(f.Config, f.FilePath)
	data, err := os.ReadFile(f.FilePath)
	if err != nil {
		f.IsLoaded = false
		return nil
	}
	var parsed bonesJSON
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	f.Bones = parsed.Bones
	f.Animations = parsed.Animations
	f.BoneWeights = parsed.BoneWeights
	f.BoneIndices = parsed.BoneIndices
	f.Attachments = parsed.Attachments
	f.IsLoaded = true
	return nil
}
