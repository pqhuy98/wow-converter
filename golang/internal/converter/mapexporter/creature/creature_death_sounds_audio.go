package creature

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
)

// Warcraft positional sounds must be mono. Decode client audio in-process and
// write PCM WAV, independent of Ogg support or an external FFmpeg installation.
func deathSoundWAV(ctx context.Context, raw []byte) ([]byte, error) {
	var pcm []byte
	rate := 0
	if bytes.HasPrefix(raw, []byte("OggS")) {
		r, err := oggvorbis.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		rate = r.SampleRate()
		channels := r.Channels()
		if channels < 1 {
			return nil, fmt.Errorf("invalid Vorbis channel count")
		}
		samples := make([]float32, 4096*channels)
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			n, err := r.Read(samples)
			if n%channels != 0 {
				return nil, fmt.Errorf("incomplete Vorbis frame")
			}
			for i := 0; i < n; i += channels {
				sum := 0.0
				for _, s := range samples[i : i+channels] {
					sum += float64(s)
				}
				value := int16(math.Round(math.Max(-1, math.Min(1, sum/float64(channels))) * 32767))
				pcm = binary.LittleEndian.AppendUint16(pcm, uint16(value))
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
		}
	} else if bytes.HasPrefix(raw, []byte("RIFF")) {
		var err error
		pcm, rate, err = deathSoundPCM(raw)
		if err != nil {
			return nil, err
		}
	} else {
		r, err := mp3.NewDecoder(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		rate = r.SampleRate()
		samples := make([]byte, 16384)
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			n, err := io.ReadFull(r, samples)
			if n%4 != 0 {
				return nil, fmt.Errorf("incomplete MP3 frame")
			}
			// go-mp3 always returns stereo signed 16-bit little-endian PCM.
			for i := 0; i < n; i += 4 {
				left := int32(int16(binary.LittleEndian.Uint16(samples[i:])))
				right := int32(int16(binary.LittleEndian.Uint16(samples[i+2:])))
				pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16((left+right)/2)))
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				return nil, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if rate <= 0 || len(pcm) == 0 {
		return nil, fmt.Errorf("empty/invalid death audio")
	}
	out := append([]byte("RIFF"), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(out[4:], uint32(36+len(pcm)))
	out = append(out, []byte("WAVEfmt ")...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	for _, n := range []uint16{1, 1} {
		out = binary.LittleEndian.AppendUint16(out, n)
	}
	out = binary.LittleEndian.AppendUint32(out, uint32(rate))
	out = binary.LittleEndian.AppendUint32(out, uint32(rate*2))
	for _, n := range []uint16{2, 16} {
		out = binary.LittleEndian.AppendUint16(out, n)
	}
	out = append(out, []byte("data")...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(pcm)))
	return append(out, pcm...), nil
}

func deathSoundPCM(raw []byte) ([]byte, int, error) {
	if len(raw) < 12 || string(raw[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("invalid WAV")
	}
	channels, bits, rate, format := 0, 0, 0, 0
	var samples []byte
	for pos := 12; pos+8 <= len(raw); {
		size := int(binary.LittleEndian.Uint32(raw[pos+4:]))
		start := pos + 8
		if size > len(raw)-start {
			return nil, 0, fmt.Errorf("truncated WAV chunk")
		}
		chunk := raw[start : start+size]
		switch string(raw[pos : pos+4]) {
		case "fmt ":
			if len(chunk) < 16 {
				return nil, 0, fmt.Errorf("truncated WAV format")
			}
			format = int(binary.LittleEndian.Uint16(chunk))
			channels = int(binary.LittleEndian.Uint16(chunk[2:]))
			rate = int(binary.LittleEndian.Uint32(chunk[4:]))
			bits = int(binary.LittleEndian.Uint16(chunk[14:]))
		case "data":
			samples = chunk
		}
		pos = start + size + (size & 1)
	}
	if format != 1 || channels < 1 || (bits != 8 && bits != 16) {
		return nil, 0, fmt.Errorf("unsupported WAV format %d/%d-bit/%d channels", format, bits, channels)
	}
	frame := channels * (bits / 8)
	if len(samples)%frame != 0 {
		return nil, 0, fmt.Errorf("incomplete WAV frame")
	}
	pcm := make([]byte, 0, len(samples)*2/frame)
	for i := 0; i < len(samples); i += frame {
		sum := int64(0)
		for c := 0; c < channels; c++ {
			value := int16(0)
			if bits == 8 {
				value = int16(int(samples[i+c])-128) * 256
			} else {
				value = int16(binary.LittleEndian.Uint16(samples[i+c*2:]))
			}
			sum += int64(value)
		}
		pcm = binary.LittleEndian.AppendUint16(pcm, uint16(int16(sum/int64(channels))))
	}
	return pcm, rate, nil
}
