package mdl

import (
	"fmt"

	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

var wowToWC3AttachmentMap = map[WoWAttachmentID]string{
	WoWAttachmentHead:          "Head",
	WoWAttachmentHandRight:     "Hand Right",
	WoWAttachmentHandLeft:      "Hand Left",
	WoWAttachmentShoulderRight: "Medium",
	WoWAttachmentShoulderLeft:  "Large",
	WoWAttachmentLeftFoot:      "Foot Left",
	WoWAttachmentRightFoot:     "Foot Right",
	WoWAttachmentChest:         "Chest",
	WoWAttachmentPlayerName:    "Overhead",
	WoWAttachmentBase:          "Origin",
}

func (mod *Modify) AddWc3AttachmentPoint() *Modify {
	for _, wowAttachment := range mod.MDL.WowAttachments {
		bone := wowAttachment.Bone
		wowAttachmentID := WoWAttachmentID(wowAttachment.WowAttachmentID)
		wc3Key, mapped := wowToWC3AttachmentMap[wowAttachmentID]
		attachmentName := wc3Key + " Ref"
		if !mapped {
			attachmentName = fmt.Sprintf("Wow:%d:%s", wowAttachmentID, animmap.GetWoWAttachmentName(int(wowAttachmentID)))
		}

		if len(mod.MDL.GlobalSequences) == 0 {
			mod.MDL.GlobalSequences = append(mod.MDL.GlobalSequences, &components.GlobalSequence{ID: -1, Duration: 1000})
		}
		globalSeq := mod.MDL.GlobalSequences[0]

		mod.MDL.Attachments = append(mod.MDL.Attachments, &components.AttachmentPoint{
			NodeBase: components.NodeBase{
				Name:       attachmentName,
				Type:       "AttachmentPoint",
				Parent:     bone,
				PivotPoint: wowAttachment.Bone.PivotPoint,
				Scaling: &components.Animation{
					Interpolation: components.InterpDontInterp,
					GlobalSeq:     globalSeq,
					KeyFrames:     map[int]any{0: imath.Vector3{1, 1, 1}},
					Type:          components.AnimTypeScaling,
				},
				Translation: &components.Animation{
					Interpolation: components.InterpDontInterp,
					GlobalSeq:     globalSeq,
					KeyFrames:     map[int]any{0: imath.Vector3{}},
					Type:          components.AnimTypeTranslation,
				},
			},
			AttachmentID: 0,
			Path:         "",
			Data:         &components.AttachmentPointData{WowAttachment: &wowAttachment},
		})
	}
	return mod
}
