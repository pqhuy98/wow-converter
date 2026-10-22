# Death audio decoder fixtures

Both files contain a generated 440 Hz sine wave (0.12 seconds, 22050 Hz, stereo).
They contain no game audio. Tests decode them without an external encoder.

To regenerate from the repository root:

```sh
ffmpeg -f lavfi -i sine=frequency=440:duration=0.12:sample_rate=22050 -ac 2 -c:a libvorbis -y golang/internal/converter/mapexporter/testdata/death-tone.ogg
ffmpeg -f lavfi -i sine=frequency=440:duration=0.12:sample_rate=22050 -ac 2 -c:a libmp3lame -y golang/internal/converter/mapexporter/testdata/death-tone.mp3
```
