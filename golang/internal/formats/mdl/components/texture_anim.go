package components

import "strings"

type TextureAnim struct {
	ID          int
	Translation *Animation
	Scaling     *Animation
	Rotation    *Animation
}

func TextureAnimsToString(textureAnims []TextureAnim) string {
	if len(textureAnims) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("TextureAnims ")
	b.WriteString(FVal(float64(len(textureAnims))))
	b.WriteString(" {\n")
	for _, texAnim := range textureAnims {
		b.WriteString("TVertexAnim {\n")
		b.WriteString(AnimationToString("Translation", texAnim.Translation))
		b.WriteString(AnimationToString("Rotation", texAnim.Rotation))
		b.WriteString(AnimationToString("Scaling", texAnim.Scaling))
		b.WriteString("}\n")
	}
	b.WriteString("}")
	return b.String()
}
