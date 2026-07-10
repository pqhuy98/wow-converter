package components

import (
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type NodeFlag string

const (
	NodeFlagDontInheritTranslation NodeFlag = "DontInherit { Translation },"
	NodeFlagDontInheritScaling     NodeFlag = "DontInherit { Scaling },"
	NodeFlagDontInheritRotation    NodeFlag = "DontInherit { Rotation },"
	NodeFlagBillboarded            NodeFlag = "Billboarded,"
	NodeFlagBillboardLockX         NodeFlag = "BillboardedLockX,"
	NodeFlagBillboardLockY         NodeFlag = "BillboardedLockY,"
	NodeFlagBillboardLockZ         NodeFlag = "BillboardedLockZ,"
)

type WowAttachment struct {
	WowAttachmentID int
	Bone            *Bone
	PivotPoint      math.Vector3
}

type Node interface {
	NodeName() string
	NodeObjectID() int
	SetNodeObjectID(int)
	NodePivotPoint() math.Vector3
	SetNodePivotPoint(math.Vector3)
	NodeParent() Node
	SetNodeParent(Node)
	NodeFlags() []NodeFlag
	NodeTranslation() *Animation
	NodeRotation() *Animation
	NodeScaling() *Animation
	NodeType() string
}

type NodeBase struct {
	Name        string
	ObjectID    int
	PivotPoint  math.Vector3
	Parent      Node
	Flags       []NodeFlag
	Translation *Animation
	Scaling     *Animation
	Rotation    *Animation
	Type        string
}

func (n *NodeBase) NodeName() string                   { return n.Name }
func (n *NodeBase) NodeObjectID() int                  { return n.ObjectID }
func (n *NodeBase) SetNodeObjectID(id int)             { n.ObjectID = id }
func (n *NodeBase) NodePivotPoint() math.Vector3       { return n.PivotPoint }
func (n *NodeBase) SetNodePivotPoint(p math.Vector3)   { n.PivotPoint = p }
func (n *NodeBase) NodeParent() Node                   { return n.Parent }
func (n *NodeBase) SetNodeParent(p Node)               { n.Parent = p }
func (n *NodeBase) NodeFlags() []NodeFlag              { return n.Flags }
func (n *NodeBase) NodeTranslation() *Animation        { return n.Translation }
func (n *NodeBase) NodeRotation() *Animation           { return n.Rotation }
func (n *NodeBase) NodeScaling() *Animation            { return n.Scaling }
func (n *NodeBase) NodeType() string                   { return n.Type }

type Bone struct {
	NodeBase
	ParentBone  *Bone
	Geoset      *Geoset
	GeosetMulti bool
	GeosetAnim  *GeosetAnim
}

func NewBone(name string) *Bone {
	return &Bone{NodeBase: NodeBase{Name: name, Type: "Bone"}}
}

type AttachmentPointData struct {
	WowAttachment *WowAttachment
}

type AttachmentPoint struct {
	NodeBase
	Path         string
	AttachmentID int
	Data         *AttachmentPointData
}

func NewAttachmentPoint(name string) *AttachmentPoint {
	return &AttachmentPoint{NodeBase: NodeBase{Name: name, Type: "AttachmentPoint"}}
}

type EventTrackEntry struct {
	Sequence *Sequence
	Offset   int
}

type EventObject struct {
	NodeBase
	Track []EventTrackEntry
}

func NewEventObject(name string) *EventObject {
	return &EventObject{NodeBase: NodeBase{Name: name, Type: "EventObject"}}
}

type Helper struct {
	NodeBase
}

func NewHelper(name string) *Helper {
	return &Helper{NodeBase: NodeBase{Name: name, Type: "Helper"}}
}

type CollisionShape struct {
	NodeBase
	ShapeType   string
	Vertices    []math.Vector3
	BoundRadius float64
}

func NewCollisionShape(name, shapeType string) *CollisionShape {
	return &CollisionShape{NodeBase: NodeBase{Name: name, Type: shapeType}}
}

func BonesToString(bones []*Bone) string {
	var blocks []string
	for _, bone := range bones {
		var b strings.Builder
		b.WriteString("Bone \"")
		b.WriteString(bone.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&bone.NodeBase))
		if bone.GeosetMulti {
			b.WriteString("GeosetId Multiple,\n")
		} else if bone.Geoset != nil {
			b.WriteString("GeosetId ")
			b.WriteString(FVal(float64(bone.Geoset.ID)))
			b.WriteString(",\n")
		}
		if bone.GeosetAnim != nil {
			b.WriteString("GeosetAnimId ")
			b.WriteString(FVal(float64(bone.GeosetAnim.ID)))
			b.WriteString(",\n")
		} else {
			b.WriteString("GeosetAnimId None,\n")
		}
		b.WriteString(NodeAnimations(&bone.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func AttachmentPointsToString(attachmentPoints []*AttachmentPoint) string {
	var blocks []string
	for _, attachment := range attachmentPoints {
		var b strings.Builder
		b.WriteString("Attachment \"")
		b.WriteString(attachment.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&attachment.NodeBase))
		b.WriteString("AttachmentID ")
		b.WriteString(FVal(float64(attachment.AttachmentID)))
		b.WriteString(",\n")
		if attachment.Path != "" {
			b.WriteString("Path \"")
			b.WriteString(attachment.Path)
			b.WriteString("\",\n")
		}
		b.WriteString(NodeAnimations(&attachment.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func CollisionShapesToString(collisionShapes []*CollisionShape) string {
	var blocks []string
	for _, shape := range collisionShapes {
		var b strings.Builder
		b.WriteString("CollisionShape \"")
		b.WriteString(shape.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&shape.NodeBase))
		b.WriteString(shape.ShapeType)
		b.WriteString(",\n")
		b.WriteString("BoundsRadius ")
		b.WriteString(FVal(shape.BoundRadius))
		b.WriteString(",\n")
		b.WriteString("Vertices ")
		b.WriteString(FVal(float64(len(shape.Vertices))))
		b.WriteString(" {\n")
		for _, v := range shape.Vertices {
			b.WriteString("{ ")
			b.WriteString(FVector3(v))
			b.WriteString(" },\n")
		}
		b.WriteString("}\n\n")
		b.WriteString(NodeAnimations(&shape.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func EventObjectsToString(eventObjects []*EventObject) string {
	for _, e := range eventObjects {
		sort.Slice(e.Track, func(i, j int) bool {
			return e.Track[i].Sequence.Interval[0] < e.Track[j].Sequence.Interval[0]
		})
	}
	var blocks []string
	for _, event := range eventObjects {
		var b strings.Builder
		b.WriteString("EventObject \"")
		b.WriteString(event.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&event.NodeBase))
		b.WriteString("EventTrack ")
		b.WriteString(FVal(float64(len(event.Track))))
		b.WriteString(" {\n")
		for _, e := range event.Track {
			b.WriteString(FVal(float64(e.Sequence.Interval[0] + e.Offset)))
			b.WriteString(",\n")
		}
		b.WriteString("}\n")
		b.WriteString(NodeAnimations(&event.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func HelpersToString(helpers []*Helper) string {
	var blocks []string
	for _, helper := range helpers {
		var b strings.Builder
		b.WriteString("Helper \"")
		b.WriteString(helper.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&helper.NodeBase))
		b.WriteString(NodeAnimations(&helper.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func PivotPointsToString(nodes []Node) string {
	var b strings.Builder
	b.WriteString("PivotPoints ")
	b.WriteString(FVal(float64(len(nodes))))
	b.WriteString(" {\n")
	for _, node := range nodes {
		b.WriteString("{ ")
		b.WriteString(FVector3(node.NodePivotPoint()))
		b.WriteString(" },\n")
	}
	b.WriteString("}")
	return b.String()
}

func NodeHeaders(node *NodeBase) string {
	var b strings.Builder
	b.WriteString("ObjectId ")
	b.WriteString(FVal(float64(node.ObjectID)))
	b.WriteString(",\n\n")
	if node.Parent != nil {
		b.WriteString("Parent ")
		b.WriteString(FVal(float64(node.Parent.NodeObjectID())))
		b.WriteString(",\n\n")
	}
	for _, flag := range node.Flags {
		b.WriteString(string(flag))
		b.WriteString("\n")
	}
	return b.String()
}

func NodeAnimations(node *NodeBase) string {
	var b strings.Builder
	b.WriteString(AnimationToString("Translation", node.Translation))
	b.WriteString(AnimationToString("Rotation", node.Rotation))
	b.WriteString(AnimationToString("Scaling", node.Scaling))
	return b.String()
}
