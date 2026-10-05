# Neck investigation checkpoint

Paused at the user's request on 2026-10-05. The neck fix is incomplete.
The previously approved chest/base conversion behavior is restored. No Git
index changes, commit, or release were made during this cleanup.

The original neck capture shows fragmented horizontal glow strips. Interpolating
both vertex positions and different bone weights introduces skinning cross terms;
the inserted vertices need not stay on the original posed triangle. Disabling
depth testing hides the gaps but draws glow through armor, so that diagnostic
was not adopted.

The experimental hybrid preserved original triangles where skin weights differed,
using a 128-pixel raster floor for the combined bake. It smoothed the collar but
lost sharp galaxy detail and increased the 21 FPS / 4000 ms export to about
316 MiB. It is unsuitable as a completed fix. The source and cached texture
6789823 palette agree visually, so a different source palette does not explain
the remaining mismatch.

Experiment source, tests, ADR copy, scripts, and matched closeups are retained
under `tmp/uv2-dimensius-detail/neck-experiment/` and its parent. Experimental
viewer assets `uv2-dimensius-neck-v3.mdx` through `-v5.mdx` remain available.
The frozen closeups use source and converter Stand at 2917 ms with transferred
camera data. The source reference is
<https://www.wowhead.com/npc=233824/dimensius>.

Resume by preserving the approved full-resolution stars while finding a bounded
representation for the neck's varying mask. Validate neck, chest, and skull base
together and measure MDX plus uniquely referenced BLP bytes before enabling it.

Wrap-up validation: the converter binary builds and the direct M2, metadata,
MDL, and BLP test packages pass. The API suite encountered Windows Access denied
in `TestReportDetectsOverwrittenExport`. The regenerated native-detail preview is
`uv2-dimensius-detail-preview.mdx`: 195.625 MiB including 42 unique referenced
BLPs. Experimental v5 is 326.771 MiB and is not the default conversion path.
