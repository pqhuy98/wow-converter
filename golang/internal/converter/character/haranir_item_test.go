package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestPlayableHaranirUsesHaranirArmorVariants(t *testing.T) {
	for gender := 0; gender < 2; gender++ {
		files := []wowhead.ItemFile{
			{FileDataID: 6122337 + gender, Race: 86, Gender: gender, ExtraData: -1},
			{FileDataID: 100 + gender, Race: 4, Gender: gender, ExtraData: -1},
		}
		if got := wowheadSelectBestModel(files, -1, gender, 1, 91); got != 6122337+gender {
			t.Fatalf("race 91 gender %d selected model %d instead of its Haranir variant", gender, got)
		}
		for i := range files {
			files[i].ExtraData = 0
		}
		if got := wowheadSelectBestTexture(files, gender, 1, 91); got != 6122337+gender {
			t.Fatalf("race 91 gender %d selected texture %d instead of its Haranir variant", gender, got)
		}
	}
}
