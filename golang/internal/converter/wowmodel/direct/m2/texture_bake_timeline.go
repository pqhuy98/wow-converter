package directm2

import (
	"log"
	"math"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type bakeMoment struct {
	time     float64
	sequence *components.Sequence
}
type bakeTimeline struct {
	moments []bakeMoment
	keys    map[int]int
	period  int
}

// A native detail loop and a baked cutout using the same source clock must
// repeat the same window. Otherwise scrolling heads pass through old holes.
func boundNativeUVLoops(result *ConvertResult, baked map[*components.GlobalSequence]bool, window int) {
	if window <= 0 {
		window = bakeWindowMS
	}
	copies := map[*components.TextureAnim]*components.TextureAnim{}
	for _, material := range result.MDL.Materials {
		for li := range material.Layers {
			layer := &material.Layers[li]
			source := layer.TVertexAnim
			if source == nil || source.Translation == nil || source.Rotation != nil || source.Scaling != nil {
				continue
			}
			a := source.Translation
			if a.GlobalSeq == nil || !baked[a.GlobalSeq] || a.GlobalSeq.Duration <= window || (a.Interpolation != components.InterpLinear && a.Interpolation != components.InterpDontInterp) {
				continue
			}
			if existing := copies[source]; existing != nil {
				layer.TVertexAnim = existing
				continue
			}
			anim := *a
			anim.KeyFrames = map[int]any{}
			for t, value := range a.KeyFrames {
				if t > 0 && t < window {
					anim.KeyFrames[t] = value
				}
			}
			for _, t := range []int{0, window} {
				value := sampleBakeAnimation(a, float64(t), []float64{0, 0, 0})
				anim.KeyFrames[t] = imath.Vector3{value[0], value[1], value[2]}
			}
			gs := components.NewGlobalSequence(len(result.MDL.GlobalSequences), window)
			gs.HasRawID = false
			anim.GlobalSeq = &gs
			result.MDL.GlobalSequences = append(result.MDL.GlobalSequences, &gs)
			copyTA := *source
			copyTA.ID = len(result.MDL.TextureAnims)
			copyTA.Translation = &anim
			result.MDL.TextureAnims = append(result.MDL.TextureAnims, copyTA)
			layer.TVertexAnim = &result.MDL.TextureAnims[len(result.MDL.TextureAnims)-1]
			copies[source] = layer.TVertexAnim
		}
	}
}

// Pure global tracks retain an independent WC3 global sequence. Local tracks
// use the model's native sequence intervals, including the one-ms separators.
// When independent global periods are incommensurate, an atlas is a bounded
// approximation: it samples each original track, then repeats a declared window.
func makeBakeTimeline(transforms [2]*components.TextureAnim, weights [4]components.AnimatedOrStatic[float64], pixel int, sequences []components.Sequence, options ...config.TextureBakingOptions) bakeTimeline {
	fps, windowMS, explicit := bakeFPS, bakeWindowMS, false
	if len(options) > 0 {
		explicit = options[0].FPS > 0 || options[0].WindowMS > 0
		if options[0].FPS > 0 {
			fps = options[0].FPS
		}
		if options[0].WindowMS > 0 {
			windowMS = options[0].WindowMS
		}
	}
	var tracks []*components.Animation
	for _, ta := range transforms {
		if ta != nil {
			tracks = append(tracks, ta.Translation, ta.Rotation, ta.Scaling)
		}
	}
	usedWeights := [4]bool{}
	switch pixel {
	case 15, 25:
		usedWeights[2] = true
	case 20, 23, 34:
		usedWeights[1] = true
	case 24:
		usedWeights[0] = true
	case 26, 28:
		usedWeights[1], usedWeights[2] = true, true
	}
	for k, w := range weights {
		if usedWeights[k] && !w.Static {
			tracks = append(tracks, w.Anim)
		}
	}
	period, longest := 0, 0
	local, approx := false, false
	for _, track := range tracks {
		if track == nil || len(track.KeyFrames) <= 1 || constantBakeTrack(track) {
			continue
		}
		if track.GlobalSeq == nil {
			local = true
			continue
		}
		duration := track.GlobalSeq.Duration
		if duration <= 0 {
			continue
		}
		longest = max(longest, duration)
		if period == 0 {
			period = duration
		} else {
			a, b := period, duration
			for b != 0 {
				a, b = b, a%b
			}
			candidate := int64(period/a) * int64(duration)
			if candidate > 60000 {
				approx = true
				period = longest
			} else {
				period = int(candidate)
			}
		}
	}
	if approx {
		period = longest
		log.Printf("M2 bake: independent UV/weight loops use %d ms, with a phase reset at the boundary", period)
	}
	if period > windowMS {
		log.Printf("Shader bake: repeating the first %d ms of the native %d ms loop", windowMS, period)
		period = windowMS
	}
	plan := bakeTimeline{keys: map[int]int{}}
	if !local || len(sequences) == 0 {
		plan.period = period
		frames := 1
		if period > 0 {
			frames = bakeFrameCount(period)
			if explicit {
				frames = max(2, int(math.Ceil(float64(period)*float64(fps)/1000)))
			}
		}
		for frame := range frames {
			time := float64(frame*period) / float64(frames)
			plan.moments = append(plan.moments, bakeMoment{time: time})
			plan.keys[int(math.Round(time))] = frame
		}
		if period > 0 {
			plan.keys[period] = 0
		}
		return plan
	}
	total := 0
	for _, s := range sequences {
		total += min(windowMS, max(1, s.Interval[1]-s.Interval[0]))
	}
	sampling := float64(fps) / 1000
	if !explicit && float64(total)*sampling > 128 {
		sampling = 128 / float64(total)
		log.Printf("M2 bake: sequence-local atlas uses %.2f samples/second over %d sequences", sampling*1000, len(sequences))
	}
	if period > 0 {
		log.Printf("M2 bake: global UV phase starts at each sequence boundary when combined with local tracks")
	}
	for si := range sequences {
		seq := &sequences[si]
		duration := max(1, seq.Interval[1]-seq.Interval[0])
		window := min(duration, windowMS)
		count := max(2, int(math.Ceil(float64(window)*sampling)))
		start := len(plan.moments)
		for frame := range count {
			time := float64(seq.Interval[0]) + float64(frame*window)/float64(count)
			plan.moments = append(plan.moments, bakeMoment{time: time, sequence: seq})
		}
		for offset := 0; offset < duration; offset += window {
			for frame := range count {
				key := seq.Interval[0] + offset + int(math.Round(float64(frame*window)/float64(count)))
				if key <= seq.Interval[1] {
					plan.keys[key] = start + frame
				}
			}
		}
		plan.keys[seq.Interval[1]] = start + min(count-1, (duration%window)*count/window)
	}
	return plan
}

func bakeFrameCount(duration int) int {
	requested := max(2, duration*bakeFPS/1000)
	count := 2
	for count*2 <= requested && count < 32 {
		count *= 2
	}
	return count
}

// Remap keys as well as samples so local sequence boundaries stay independent.
func reduceBakeTimeline(plan bakeTimeline) bakeTimeline {
	out := bakeTimeline{keys: map[int]int{}, period: plan.period}
	remap := make([]int, len(plan.moments))
	within := 0
	for i, moment := range plan.moments {
		if i == 0 || moment.sequence != plan.moments[i-1].sequence {
			within = 0
		}
		if within%2 == 0 {
			out.moments = append(out.moments, moment)
		}
		remap[i] = len(out.moments) - 1
		within++
	}
	for key, frame := range plan.keys {
		out.keys[key] = remap[frame]
	}
	return out
}

func bakeAnimationAt(anim *components.Animation, moment bakeMoment, fallback []float64) []float64 {
	if anim == nil {
		return fallback
	}
	if moment.sequence == nil {
		return sampleBakeAnimation(anim, moment.time, fallback)
	}
	if anim.GlobalSeq != nil {
		return sampleBakeAnimation(anim, moment.time-float64(moment.sequence.Interval[0]), fallback)
	}
	local := *anim
	local.KeyFrames = map[int]any{}
	for key, value := range anim.KeyFrames {
		if key >= moment.sequence.Interval[0] && key <= moment.sequence.Interval[1] {
			local.KeyFrames[key] = value
		}
	}
	return sampleBakeAnimation(&local, moment.time, fallback)
}

func bakeTransformAt(anim *components.TextureAnim, moment bakeMoment) [6]float64 {
	if anim == nil || moment.sequence == nil {
		return bakeTransform(anim, moment.time)
	}
	copyAnim := *anim
	copyTrack := func(track *components.Animation) *components.Animation {
		if track == nil {
			return nil
		}
		copyTrack := *track
		if track.GlobalSeq != nil {
			return &copyTrack
		}
		copyTrack.KeyFrames = map[int]any{}
		for k, v := range track.KeyFrames {
			if k >= moment.sequence.Interval[0] && k <= moment.sequence.Interval[1] {
				copyTrack.KeyFrames[k-moment.sequence.Interval[0]] = v
			}
		}
		return &copyTrack
	}
	copyAnim.Translation, copyAnim.Rotation, copyAnim.Scaling = copyTrack(anim.Translation), copyTrack(anim.Rotation), copyTrack(anim.Scaling)
	return bakeTransform(&copyAnim, moment.time-float64(moment.sequence.Interval[0]))
}
