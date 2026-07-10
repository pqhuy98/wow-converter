package m2export

// GeosetGroups maps geoset group base IDs to labels.
var GeosetGroups = map[int]string{
	0: "Hair", 100: "FacialA", 200: "FacialB", 300: "FacialC", 400: "Gloves",
	500: "Boots", 600: "Tail", 700: "Ears", 800: "Wrists", 900: "Kneepads",
	1000: "Chest", 1100: "Pants", 1200: "Tabard", 1300: "Trousers", 1500: "Cloak",
	1600: "Chins", 1700: "Eyeglow", 1800: "Belt", 1900: "Bone/Tail", 2000: "Feet",
	2200: "Torso", 2300: "HandAttach", 2400: "HeadAttach", 2500: "DHBlindfolds",
	2700: "Head", 2800: "Chest", 2900: "MechagnomeArms", 3000: "MechagnomeLegs",
	3100: "MechagnomeFeet", 3200: "Face", 3300: "Eyes", 3400: "Eyebrows",
	3500: "Earrings", 3600: "Necklace", 3700: "Headdress", 3800: "Tails",
	3900: "Vines", 4000: "Chins/Tusks", 4100: "Noses", 4200: "HairDecoA",
	4300: "HairDecoB", 4400: "BodySize", 5100: "EyeGlowB",
}

// GetGeosetName returns a geoset label.
func GetGeosetName(index int, id int) string {
	if id == 0 {
		return "Geoset" + itoa(index)
	}
	base := (id / 100) * 100
	if group, ok := GeosetGroups[base]; ok {
		return group + itoa(id-base)
	}
	return "Geoset" + itoa(index) + "_" + itoa(base)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// GeosetMaskEntry controls submesh export.
type GeosetMaskEntry struct {
	ID      int
	Checked bool
}

// VariantTexture is a texture file data ID or data-texture key.
type VariantTexture = any

// TextureManifestEntry describes an exported texture slot.
type TextureManifestEntry struct {
	MatName         string
	MatPathRelative string
	MatPath         string
}
