package mdl

import (
	"log"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func (mod *Modify) AddPortraitCamera(standSequenceName string) *Modify {
	if standSequenceName == "" {
		standSequenceName = "Stand"
	}
	cameraName := "Portrait_Camera"

	var cameraBone *components.Bone
	for _, name := range []string{"Head", "Chest", "Root"} {
		for _, b := range mod.MDL.Bones {
			if b.Name == name {
				cameraBone = b
				break
			}
		}
		if cameraBone != nil {
			break
		}
	}

	if cameraBone == nil {
		mod.MDL.Cameras = append(mod.MDL.Cameras, components.Camera{
			Name:        cameraName,
			FieldOfView: 1,
			NearClip:    0.1,
			FarClip:     100000,
			Target: components.CameraTarget{
				Position: imath.V3Mean(mod.MDL.Model.MinimumExtent, mod.MDL.Model.MaximumExtent),
			},
			Position: imath.V3Scale(imath.Vector3{
				mod.MDL.Model.MinimumExtent[0],
				mod.MDL.Model.MaximumExtent[1],
				mod.MDL.Model.MaximumExtent[2],
			}, 1.1),
		})
		return mod
	}

	nodePos := cameraBone.PivotPoint
	var standSequence *components.Sequence
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Name == standSequenceName {
			standSequence = &mod.MDL.Sequences[i]
			break
		}
	}
	if standSequence != nil {
		IterateNodesAtTimestamp(mod.MDL, *standSequence, standSequence.Interval[0], func(node components.Node, value TransformValue) {
			if node == cameraBone {
				nodePos = value.Position
			}
		})
	}

	distanceScale := map[string]float64{"Head": 3, "Chest": 2, "Root": 1}[cameraBone.Name]
	rand := imath.SeededRandom("portrait-camera:" + mod.MDL.Model.Name)
	cameraPosition := imath.V3Sum(nodePos, imath.Vector3{
		distanceScale * mod.MDL.Model.MaximumExtent[0],
		0.5 * (rand() - 0.5) * mod.MDL.Model.MaximumExtent[1],
		(rand()*0.2 - 0.1) * mod.MDL.Model.MaximumExtent[2],
	})
	log.Printf("Generated portrait camera looking at bone %s", cameraBone.Name)

	mod.MDL.Cameras = append(mod.MDL.Cameras, components.Camera{
		Name:        cameraName,
		FieldOfView: 1,
		NearClip:    0.1,
		FarClip:     100000,
		Target:      components.CameraTarget{Position: nodePos},
		Position:    cameraPosition,
	})
	return mod
}
