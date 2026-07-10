package directm2

import (
	"context"
	"fmt"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

const skeletonGraphCacheMax = 8

type skeletonGraph struct {
	Bones           []m2.BoneEntry
	Animations      []m2.AnimationEntry
	SkelAttachments []m2.AttachmentEntry
}

var (
	skeletonGraphMu    sync.Mutex
	skeletonGraphCache = map[string]skeletonGraph{}
	skeletonGraphOrder []string
	skeletonBuildKey   string
)

// ClearSkeletonGraphCache drops cached skeleton graphs (e.g. when the active CASC build changes).
func ClearSkeletonGraphCache() {
	skeletonGraphMu.Lock()
	defer skeletonGraphMu.Unlock()
	skeletonGraphCache = map[string]skeletonGraph{}
	skeletonGraphOrder = nil
	skeletonBuildKey = ""
}

func init() {
	runtimecache.RegisterConverterClearHook(ClearSkeletonGraphCache)
}

func skeletonGraphCacheGet(key string) (skeletonGraph, bool) {
	skeletonGraphMu.Lock()
	defer skeletonGraphMu.Unlock()
	g, ok := skeletonGraphCache[key]
	if !ok {
		return skeletonGraph{}, false
	}
	for i, k := range skeletonGraphOrder {
		if k == key {
			skeletonGraphOrder = append(skeletonGraphOrder[:i], skeletonGraphOrder[i+1:]...)
			break
		}
	}
	skeletonGraphOrder = append(skeletonGraphOrder, key)
	return g, true
}

func skeletonGraphCacheSet(key string, graph skeletonGraph) {
	skeletonGraphMu.Lock()
	defer skeletonGraphMu.Unlock()
	if skeletonBuildKey != "" {
		prefix := skeletonBuildKey + ":"
		if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
			return
		}
	}
	skeletonGraphCache[key] = graph
	for i, k := range skeletonGraphOrder {
		if k == key {
			skeletonGraphOrder = append(skeletonGraphOrder[:i], skeletonGraphOrder[i+1:]...)
			break
		}
	}
	skeletonGraphOrder = append(skeletonGraphOrder, key)
	for len(skeletonGraphOrder) > skeletonGraphCacheMax {
		evict := skeletonGraphOrder[0]
		skeletonGraphOrder = skeletonGraphOrder[1:]
		delete(skeletonGraphCache, evict)
	}
}

func skeletonGraphCacheKey(buildKey string, loader *m2.Loader, m2FileDataID int) string {
	if buildKey == "" {
		return ""
	}
	if loader.SkeletonFileID > 0 {
		return fmt.Sprintf("%s:skel:%d", buildKey, loader.SkeletonFileID)
	}
	if m2FileDataID > 0 {
		return fmt.Sprintf("%s:m2:%d", buildKey, m2FileDataID)
	}
	return ""
}

func loadSkeletonGraph(ctx context.Context, loader *m2.Loader, m2FileDataID int, buildKey string) (skeletonGraph, error) {
	if buildKey != "" {
		skeletonGraphMu.Lock()
		if skeletonBuildKey != "" && buildKey != skeletonBuildKey {
			skeletonGraphCache = map[string]skeletonGraph{}
			skeletonGraphOrder = nil
		}
		skeletonBuildKey = buildKey
		skeletonGraphMu.Unlock()
	}
	if key := skeletonGraphCacheKey(buildKey, loader, m2FileDataID); key != "" {
		if graph, ok := skeletonGraphCacheGet(key); ok {
			return graph, nil
		}
	}
	graph, err := loadSkeletonGraphUncached(ctx, loader)
	if err != nil {
		return skeletonGraph{}, err
	}
	if key := skeletonGraphCacheKey(buildKey, loader, m2FileDataID); key != "" {
		skeletonGraphCacheSet(key, graph)
	}
	return graph, nil
}

func loadSkeletonGraphUncached(ctx context.Context, loader *m2.Loader) (skeletonGraph, error) {
	getFile := loader.GetFile
	if loader.SkeletonFileID > 0 {
		raw, err := getFile(ctx, loader.SkeletonFileID)
		if err != nil {
			return skeletonGraph{}, err
		}
		skel := m2.NewSkelLoader(buffer.From(raw), getFile)
		skel.Load()
		if err := skel.LoadAnims(ctx); err != nil {
			return skeletonGraph{}, err
		}

		if skel.ParentSkelFileID > 0 {
			parentRaw, err := getFile(ctx, skel.ParentSkelFileID)
			if err != nil {
				return skeletonGraph{}, err
			}
			parentSkel := m2.NewSkelLoader(buffer.From(parentRaw), getFile)
			parentSkel.Load()
			if err := parentSkel.LoadAnims(ctx); err != nil {
				return skeletonGraph{}, err
			}

			type animIndexPair struct {
				childIdx  int
				parentIdx int
			}
			var animIndexMap []animIndexPair
			for i, anim := range skel.Animations {
				for j, parentAnim := range parentSkel.Animations {
					if parentAnim.ID == anim.ID && parentAnim.VariationIndex == anim.VariationIndex {
						animIndexMap = append(animIndexMap, animIndexPair{childIdx: i, parentIdx: j})
						break
					}
				}
			}

			for i := range skel.Bones {
				if i >= len(parentSkel.Bones) {
					break
				}
				bone := &skel.Bones[i]
				parentBone := &parentSkel.Bones[i]
				for _, pair := range animIndexMap {
					copyTrackSlice(&parentBone.Translation, &bone.Translation, pair.childIdx, pair.parentIdx)
					copyTrackSlice(&parentBone.Rotation, &bone.Rotation, pair.childIdx, pair.parentIdx)
					copyTrackSlice(&parentBone.Scale, &bone.Scale, pair.childIdx, pair.parentIdx)
				}
			}

			return skeletonGraph{
				Bones:           parentSkel.Bones,
				Animations:      parentSkel.Animations,
				SkelAttachments: skel.Attachments,
			}, nil
		}

		return skeletonGraph{
			Bones:           skel.Bones,
			Animations:      skel.Animations,
			SkelAttachments: skel.Attachments,
		}, nil
	}

	if err := loader.LoadAnims(ctx); err != nil {
		return skeletonGraph{}, err
	}
	return skeletonGraph{
		Bones:      loader.Bones,
		Animations: loader.Animations,
	}, nil
}

func copyTrackSlice(dst *m2.Track, src *m2.Track, srcIdx, dstIdx int) {
	if srcIdx >= len(src.Timestamps) || srcIdx >= len(src.Values) {
		return
	}
	if dstIdx >= len(dst.Timestamps) || dstIdx >= len(dst.Values) {
		return
	}
	// Match the TS path exactly: if the child skeleton has an entry for this
	// animation slot, it overrides the parent even when the child arrays are
	// empty. That clears inherited parent keys instead of keeping stale data.
	dst.Timestamps[dstIdx] = append([]uint32(nil), src.Timestamps[srcIdx]...)
	dst.Values[dstIdx] = append([][]float64(nil), src.Values[srcIdx]...)
}
