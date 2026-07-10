package mdl

// WoWAttachmentID constants used by attachment mapping.
type WoWAttachmentID int

const (
	WoWAttachmentHandRight     WoWAttachmentID = 1
	WoWAttachmentHandLeft      WoWAttachmentID = 2
	WoWAttachmentShoulderRight WoWAttachmentID = 5
	WoWAttachmentShoulderLeft  WoWAttachmentID = 6
	WoWAttachmentPlayerName    WoWAttachmentID = 18
	WoWAttachmentBase          WoWAttachmentID = 19
	WoWAttachmentHead          WoWAttachmentID = 20
	WoWAttachmentChest         WoWAttachmentID = 34
	WoWAttachmentLeftFoot      WoWAttachmentID = 47
	WoWAttachmentRightFoot     WoWAttachmentID = 48
	WoWAttachmentUnknown       WoWAttachmentID = 60
)

var wowAttachmentNames = map[WoWAttachmentID]string{
	WoWAttachmentHandRight:     "HandRight",
	WoWAttachmentHandLeft:      "HandLeft",
	WoWAttachmentShoulderRight: "ShoulderRight",
	WoWAttachmentShoulderLeft:  "ShoulderLeft",
	WoWAttachmentPlayerName:    "PlayerName",
	WoWAttachmentBase:          "Base",
	WoWAttachmentHead:          "Head",
	WoWAttachmentChest:         "Chest",
	WoWAttachmentLeftFoot:      "LeftFoot",
	WoWAttachmentRightFoot:     "RightFoot",
	WoWAttachmentUnknown:       "Unknown",
}

func GetWoWAttachmentName(attachmentID int) string {
	if name, ok := wowAttachmentNames[WoWAttachmentID(attachmentID)]; ok {
		return name
	}
	return "unknown"
}

// WowAnimName is a minimal subset of WoW animation names used by modify helpers.
type WowAnimName string

const (
	WowAnimWalk   WowAnimName = "Walk"
	WowAnimRun    WowAnimName = "Run"
	WowAnimSprint WowAnimName = "Sprint"
)
