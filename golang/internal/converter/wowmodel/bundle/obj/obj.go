package obj

// Vertex is an OBJ vertex position.
type Vertex struct {
	X, Y, Z float64
}

// TextureVertex is an OBJ texture coordinate.
type TextureVertex struct {
	U, V, W float64
}

// Group names a face group / geoset.
type Group struct {
	Name string
	ID   int
}

// FaceVertex references OBJ vertex attributes (1-based in file format).
type FaceVertex struct {
	VertexIndex        int
	TextureCoordsIndex int
	VertexNormalIndex  int
}

// Face is a triangle face.
type Face struct {
	Group          Group
	Material       string
	SmoothingGroup int
	Vertices       [3]FaceVertex
}

// Model is a parsed OBJ model.
type Model struct {
	Name           string
	Group          []Group
	Vertices       []Vertex
	TextureCoords  []TextureVertex
	TextureCoords2 []TextureVertex
	VertexNormals  []Vertex
	Faces          []Face
}

// Result is the full parsed OBJ document.
type Result struct {
	Models            []Model
	MaterialLibraries []string
}
