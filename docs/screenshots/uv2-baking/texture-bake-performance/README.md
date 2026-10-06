# Texture baking fixes and performance measurements

Measured on 2026-10-06 on the local Ryzen 9 5900X (24 logical CPUs), 32 GB RAM,
using the existing Retail wow-data-server and warm build-keyed source cache.
These measurements describe the cases below, not every model or cold-cache load.

| Case | Before | After | Scope |
| --- | ---: | ---: | --- |
| Void pylon | 115.151 s | 50.119 s | Direct still conversion, CPU/heap profiled |
| Void pylon | 116.15 s | 49.75 s | Earlier versus fresh API server export |
| Eagle temple | 249.48 s | 97.882 s | Earlier versus fresh API server export |

The pylon direct conversion is 56% faster. Its final MDL and all 33 baked PNGs
match the verified serial output exactly (full SHA-256 manifests). The temple's
MDX matches the earlier output exactly after normalizing only the model name;
all 44 texture references are unchanged. JSON evidence is saved alongside this
file. The final pylon converter heap peaked at 1,841,392,456 bytes in 100 ms
sampling; this excludes the separate CASC/data-server process and is not RSS.
The process allocated 69,692,349,456 bytes cumulatively and ran 143 GCs. Baseline
pprof sampled allocation volume was about 80 GiB; final volume was about 65 GiB.

## Fixes

Firehawk's native painted body consumed little of its assigned reserve, while
overlapping wing cards exhausted their much smaller effect allowance. Reclaiming
unused reference-budget pixels for subsequent batches raised one wing's median
triangle footprint from 1.49 to 20.87 texels. `firehawk-before.png` and
`firehawk-after.png` use the same camera and frozen pose and show the large grid
replaced by flame detail. The Firehawk integration now checks minimum wing texel
area, transparency, material flags and serialized animation tracks.

Equipment already inherited the export configuration, but global-only UV
tracks were discarded when no ordinary animation existed. Some replacements
also became available only after baking. The canonical track parser now keeps
global timestamps independently; items and collections pass exact texture-type
replacements before baking. Synthetic atlases cannot be overwritten by later
component replacement. The fresh troll has 13 active texture-animation bindings
(the original had none). `troll-shoulders-0ms.png` and
`troll-shoulders-700ms.png` freeze the skeleton and change only the global clock.
The real shoulder integration checks both animated and still modes and reload.

## Performance changes and checks

- Index actual chart overlaps instead of scanning every chart's sample map.
- Prepare still shader colours only when compared, and reuse per-face raster
  write buffers, WMO varyings and tangent bases.
- Expand padding through its boundary frontier, preserving the exact original
  neighbour order, halo and sample payload.
- Bake at most two independent WMO batches concurrently, with private mutable
  graphs and source-order merging. Resolve/decode shared textures once.
- Snapshot worker animation references before rebasing; resolve the final slice
  only after every merge, including multiple animated output passes.

An eager shader-preparation experiment took 145 s and was discarded. GPU baking
was not implemented: the profile identified chart construction and packing as
the larger costs, and these CPU changes already improve the end-to-end export.
More concurrent batches would raise peak working memory; the production limit
remains two.

Focused character, metadata, M2, WMO, MDL and WMO-loader tests pass. Worker tests
compare serial and parallel PNG bytes and optimized MDL/MDX, including animation
slice growth, existing animations and several output passes. Real Firehawk and
shoulder integration tests pass against the existing data server. No snapshot
goldens were rewritten. Actual viewer captures confirm the fixes and preserved
temple blending. In-game Warcraft III validation remains separate. Go's race
test could not run because this environment has no C/C++ compiler for cgo.

## Reproduce

With the dev/data server already running, from the repository root:

```powershell
.\scripts\profile-wmo-texture-bake.ps1 -Wmo 'world\wmo\expansion11\void\12vd_void_pylon01'
```

The script writes a test executable, `cpu.pprof`, `heap.pprof`, and `export/`
with `model.mdl`, `conversion-metrics.json`, generated PNGs and a sorted
`textures.json` SHA-256 manifest. Inspect a profile with:

```powershell
go -C golang tool pprof -top <absolute-path-to-cpu.pprof>
go -C golang tool pprof -alloc_space -top <absolute-path-to-heap.pprof>
```

Experiment scripts, CPU/heap profiles, requests, complete outputs and extra
captures remain under `tmp/texture-bake-performance/`. The temporary probe/camera
tests are archived in that directory's `scripts/`, not left in production tests.

Explicit FPS/window requests preserve spatial quality and every requested frame.
The detailed 12 FPS / 4000 ms Alysrazor preview has 181,497,660 texture bytes
(176.37 MiB including its MDX). The default adaptive preview has 27,699,440
texture bytes (29.69 MiB including MDX) and keeps the same wing spatial detail
while using fewer temporal samples. This is a size tradeoff of the quality fix,
not a performance saving or a GPU change. Preview URLs on the home LAN:

- `http://192.168.0.195:3001/viewer?model=alysrazor-fixed.mdx`
- `http://192.168.0.195:3001/viewer?model=alysrazor-compact-fixed.mdx`
- `http://192.168.0.195:3001/viewer?model=troll-animated-fixed.mdx`
- `http://192.168.0.195:3001/viewer?model=void-pylon-optimized.mdx`
- `http://192.168.0.195:3001/viewer?model=eagletemple-optimized.mdx`
