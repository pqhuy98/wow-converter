package animation

import (
	"fmt"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// WoWAttachmentID identifies WoW attachment points.
type WoWAttachmentID int

const (
	WoWAttachmentShield               WoWAttachmentID = 0
	WoWAttachmentHandRight            WoWAttachmentID = 1
	WoWAttachmentHandLeft             WoWAttachmentID = 2
	WoWAttachmentElbowRight           WoWAttachmentID = 3
	WoWAttachmentElbowLeft            WoWAttachmentID = 4
	WoWAttachmentShoulderRight        WoWAttachmentID = 5
	WoWAttachmentShoulderLeft         WoWAttachmentID = 6
	WoWAttachmentKneeRight            WoWAttachmentID = 7
	WoWAttachmentKneeLeft             WoWAttachmentID = 8
	WoWAttachmentHipRight             WoWAttachmentID = 9
	WoWAttachmentHipLeft              WoWAttachmentID = 10
	WoWAttachmentHelm                 WoWAttachmentID = 11
	WoWAttachmentBack                 WoWAttachmentID = 12
	WoWAttachmentShoulderFlapRight    WoWAttachmentID = 13
	WoWAttachmentShoulderFlapLeft     WoWAttachmentID = 14
	WoWAttachmentChestBloodFront      WoWAttachmentID = 15
	WoWAttachmentChestBloodBack       WoWAttachmentID = 16
	WoWAttachmentBreath               WoWAttachmentID = 17
	WoWAttachmentPlayerName           WoWAttachmentID = 18
	WoWAttachmentBase                 WoWAttachmentID = 19
	WoWAttachmentHead                 WoWAttachmentID = 20
	WoWAttachmentSpellLeftHand        WoWAttachmentID = 21
	WoWAttachmentSpellRightHand       WoWAttachmentID = 22
	WoWAttachmentSpecial1             WoWAttachmentID = 23
	WoWAttachmentSpecial2             WoWAttachmentID = 24
	WoWAttachmentSpecial3             WoWAttachmentID = 25
	WoWAttachmentSheathMainHand       WoWAttachmentID = 26
	WoWAttachmentSheathOffHand        WoWAttachmentID = 27
	WoWAttachmentSheathShield         WoWAttachmentID = 28
	WoWAttachmentPlayerNameMounted    WoWAttachmentID = 29
	WoWAttachmentLargeWeaponLeft      WoWAttachmentID = 30
	WoWAttachmentLargeWeaponRight     WoWAttachmentID = 31
	WoWAttachmentHipWeaponLeft        WoWAttachmentID = 32
	WoWAttachmentHipWeaponRight       WoWAttachmentID = 33
	WoWAttachmentChest                WoWAttachmentID = 34
	WoWAttachmentHandArrow            WoWAttachmentID = 35
	WoWAttachmentBullet               WoWAttachmentID = 36
	WoWAttachmentSpellHandOmni        WoWAttachmentID = 37
	WoWAttachmentSpellHandDirected    WoWAttachmentID = 38
	WoWAttachmentVehicleSeat1         WoWAttachmentID = 39
	WoWAttachmentVehicleSeat2         WoWAttachmentID = 40
	WoWAttachmentVehicleSeat3         WoWAttachmentID = 41
	WoWAttachmentVehicleSeat4         WoWAttachmentID = 42
	WoWAttachmentVehicleSeat5         WoWAttachmentID = 43
	WoWAttachmentVehicleSeat6         WoWAttachmentID = 44
	WoWAttachmentVehicleSeat7         WoWAttachmentID = 45
	WoWAttachmentVehicleSeat8         WoWAttachmentID = 46
	WoWAttachmentLeftFoot             WoWAttachmentID = 47
	WoWAttachmentRightFoot            WoWAttachmentID = 48
	WoWAttachmentShieldNoGlove        WoWAttachmentID = 49
	WoWAttachmentSpineLow             WoWAttachmentID = 50
	WoWAttachmentAlteredShoulderR     WoWAttachmentID = 51
	WoWAttachmentAlteredShoulderL     WoWAttachmentID = 52
	WoWAttachmentBeltBuckle           WoWAttachmentID = 53
	WoWAttachmentSheathCrossbow       WoWAttachmentID = 54
	WoWAttachmentHeadTop              WoWAttachmentID = 55
	WoWAttachmentVirtualSpellDirected WoWAttachmentID = 56
	WoWAttachmentBackpack             WoWAttachmentID = 57
	WoWAttachmentUnknown              WoWAttachmentID = 60
)

var wowAttachmentNames = map[WoWAttachmentID]string{
	WoWAttachmentShield: "Shield", WoWAttachmentHandRight: "HandRight", WoWAttachmentHandLeft: "HandLeft",
	WoWAttachmentElbowRight: "ElbowRight", WoWAttachmentElbowLeft: "ElbowLeft",
	WoWAttachmentShoulderRight: "ShoulderRight", WoWAttachmentShoulderLeft: "ShoulderLeft",
	WoWAttachmentKneeRight: "KneeRight", WoWAttachmentKneeLeft: "KneeLeft",
	WoWAttachmentHipRight: "HipRight", WoWAttachmentHipLeft: "HipLeft",
	WoWAttachmentHelm: "Helm", WoWAttachmentBack: "Back",
	WoWAttachmentShoulderFlapRight: "ShoulderFlapRight", WoWAttachmentShoulderFlapLeft: "ShoulderFlapLeft",
	WoWAttachmentChestBloodFront: "ChestBloodFront", WoWAttachmentChestBloodBack: "ChestBloodBack",
	WoWAttachmentBreath: "Breath", WoWAttachmentPlayerName: "PlayerName",
	WoWAttachmentBase: "Base", WoWAttachmentHead: "Head",
	WoWAttachmentSpellLeftHand: "SpellLeftHand", WoWAttachmentSpellRightHand: "SpellRightHand",
	WoWAttachmentSpecial1: "Special1", WoWAttachmentSpecial2: "Special2", WoWAttachmentSpecial3: "Special3",
	WoWAttachmentSheathMainHand: "SheathMainHand", WoWAttachmentSheathOffHand: "SheathOffHand",
	WoWAttachmentSheathShield: "SheathShield", WoWAttachmentPlayerNameMounted: "PlayerNameMounted",
	WoWAttachmentLargeWeaponLeft: "LargeWeaponLeft", WoWAttachmentLargeWeaponRight: "LargeWeaponRight",
	WoWAttachmentHipWeaponLeft: "HipWeaponLeft", WoWAttachmentHipWeaponRight: "HipWeaponRight",
	WoWAttachmentChest: "Chest", WoWAttachmentHandArrow: "HandArrow", WoWAttachmentBullet: "Bullet",
	WoWAttachmentSpellHandOmni: "SpellHandOmni", WoWAttachmentSpellHandDirected: "SpellHandDirected",
	WoWAttachmentVehicleSeat1: "VehicleSeat1", WoWAttachmentVehicleSeat2: "VehicleSeat2",
	WoWAttachmentVehicleSeat3: "VehicleSeat3", WoWAttachmentVehicleSeat4: "VehicleSeat4",
	WoWAttachmentVehicleSeat5: "VehicleSeat5", WoWAttachmentVehicleSeat6: "VehicleSeat6",
	WoWAttachmentVehicleSeat7: "VehicleSeat7", WoWAttachmentVehicleSeat8: "VehicleSeat8",
	WoWAttachmentLeftFoot: "LeftFoot", WoWAttachmentRightFoot: "RightFoot",
	WoWAttachmentShieldNoGlove: "ShieldNoGlove", WoWAttachmentSpineLow: "SpineLow",
	WoWAttachmentAlteredShoulderR: "AlteredShoulderR", WoWAttachmentAlteredShoulderL: "AlteredShoulderL",
	WoWAttachmentBeltBuckle: "BeltBuckle", WoWAttachmentSheathCrossbow: "SheathCrossbow",
	WoWAttachmentHeadTop: "HeadTop", WoWAttachmentVirtualSpellDirected: "VirtualSpellDirected",
	WoWAttachmentBackpack: "Backpack", WoWAttachmentUnknown: "Unknown",
}

// GetWoWAttachmentName returns the attachment name for an ID.
func GetWoWAttachmentName(attachmentID int) string {
	if name, ok := wowAttachmentNames[WoWAttachmentID(attachmentID)]; ok {
		return name
	}
	return "unknown"
}

// GetBoneName resolves a bone name from WoW identifiers.
func GetBoneName(boneID int, index int, crc uint32) string {
	if name, ok := boneNames[boneID]; ok {
		return name
	}
	if name, ok := crcBoneNames[crc]; ok {
		return name
	}
	if crc != 0 {
		return fmt.Sprintf("bone_crc_%d", crc)
	}
	if boneID != 0 {
		return fmt.Sprintf("bone_id_%d", boneID)
	}
	return fmt.Sprintf("bone_%d", index)
}

// IsUnknownBone reports whether a bone name is an unresolved index placeholder.
func IsUnknownBone(bone components.Bone) bool {
	if bone.Name == "" {
		return false
	}
	if !strings.HasPrefix(bone.Name, "bone_") {
		return false
	}
	for _, ch := range bone.Name[len("bone_"):] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return len(bone.Name) > len("bone_")
}
