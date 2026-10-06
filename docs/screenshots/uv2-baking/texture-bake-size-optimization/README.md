# Advanced texture size optimization

Measured 2026-10-06 using the local Retail build 12.1.0.69933, the existing bundled CASC server, warm source/texture caches, and uncommitted converter changes. MiB means 1,048,576 bytes. Every total counts one model plus each distinct referenced BLP path once; unrelated files accumulated in exported-assets are excluded. MDX previews are serialized from the exported MDL using mdx-m3-viewer. Both use version 1000 in these measurements.

| Model | Resolution | FPS / window ms | MDX MiB | MDL MiB | BLP MiB | MDX + BLP MiB | MDL + BLP MiB | API seconds |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| firehawk | full | 12 / 4000 | 3.30 | 8.37 | 96.16 | 99.46 | 104.54 | 21.24 |
| firehawk | half | 12 / 4000 | 3.30 | 8.37 | 24.66 | 27.96 | 33.04 | 18.13 |
| troll | full | 15 / 4000 | 14.16 | 36.73 | 101.68 | 115.85 | 138.42 | 24.26 |
| troll | half | 15 / 4000 | 14.16 | 36.73 | 34.21 | 48.37 | 70.94 | 20.24 |
| dressing | full | 15 / 1000 | 31.96 | 87.09 | 64.58 | 96.54 | 151.67 | 33.40 |
| dressing | half | 15 / 1000 | 31.96 | 87.09 | 28.20 | 60.16 | 115.29 | 20.17 |

API timings include queue polling but exclude preview serialization and screenshot capture. They describe these warm-cache runs, not cold-cache performance.

## Changes without reducing raster detail or temporal samples

- Crop repeated frame cells to the padded chart bounds. Cell strides are aligned to 8 pixels for the first three mip levels; each final page remains a power-of-two rectangle. Search uniform page dimensions to minimize allocated page pixels, then update normalized UVs and frame/page tracks. Preserve existing spatial budget decisions, chart samples and three-pixel gutters. The empty horizontal bands were repeated power-of-two cell margins, not a required WC3 gap.
- Share identical generated image files across attachments while keeping their texture objects, wrap flags, material states and animation tracks independent. Match PNG bytes and encoding flags, not just a short hash.
- Select customization/equipment collection geosets before baking, using the existing selection and missing-variant fallback rules. A file shared by multiple equipment slots gets the union of their selections. Unknown/non-collection files retain the prior conversion path.
- Compact BLP1 alpha planes only when all mip alpha values can be represented exactly in 0, 1 or 4 bits. Preserve palette and index bytes. For explicitly known opaque baked base passes, alpha is unused and can be removed after quantization without changing any mip RGB. The alpha-sensitive attenuation pass is retained.
- Entirely zero-RGB secondary additive images can become one transparent black texel; do not apply this to black Blend attenuation images, whose alpha still affects the result.

## Opt-in half resolution

The shared Advanced Textures dialog in Character and Browse exports offers Full resolution (default) and Half resolution (50% width and height). The API field is textureBaking.resolutionScale: 1 or 0.5; omitted values preserve full resolution. The same request configuration reaches attached items and collections and applies to M2, WMO and generated particle textures. Animated and still modes both support it.

Half resolution downsamples only generated images; it keeps FPS, window length, geometry and normalized animation tracks. Straight-alpha outputs filter premultiplied color before returning to straight alpha, avoiding dark edges. RGB-only material modes filter independently. Baked image pixel count is roughly quartered; native textures and model data remain, so the whole package does not shrink to one quarter.

## Historical comparisons

The immediately preceding detailed Firehawk used 12 FPS / 4000 ms and occupied 176.37 MiB including MDX. The new full variant keeps the same spatial triangle footprints (within MDL rounding), geometry, FPS and window. The reported troll's earlier MDL plus textures occupied 182.21 MiB; compare that with the MDL + BLP column, not the binary MDX column.

The older cached 30 FPS / 4000 ms result at tmp/uv2-seven/benchmark/uv2-alysrazor-fps30-4000ms.json recorded 13 baked pages, 10,485,760 baked pixels and 27,977,366 baked bytes. Its baked images are no longer available, so identical visual quality cannot be established. Later spatial detail fixes raised crowded wing chart coverage substantially; FPS alone cannot explain or predict package size. The new packing removes waste without reverting the detail fix.

The second character remains geometry-heavy: native-detail mask subdivisions preserve the shader result but create many faces. Its black pages carry nonzero alpha and use Blend, so deleting their geometry would change rendering. They cannot be treated as invisible additive passes.

## Verification and evidence

Relevant M2, WMO, character, common, BLP and API option tests pass. The real Firehawk and troll shoulder integration tests use the existing server, covering wing detail, transparency, animated/still global UV tracks and serialization. Alpha tests compare decoded mip RGB/RGBA and preserve quantizer palette/index data. Planner tests check chart containment, frame translations and mip alignment. Collection tests check exact selection, missing variants and shared-file unions. Full/half UI controls were checked in Character and Browse. No snapshot goldens were rewritten.

Actual frozen-camera viewer captures: [Firehawk full](firehawk-full.png), [half](firehawk-half.png); troll full at [0 ms](troll-full-0ms.png) / [700 ms](troll-full-700ms.png), half at [0 ms](troll-half-0ms.png) / [700 ms](troll-half-700ms.png); second character [full](dressing-full.png) / [half](dressing-half.png). UI: [resolution selector](ui-resolution-options.png), [Browse still mode](ui-browse-half-still.png). In-game WC3 validation remains separate.

## Reproduce

Exact API requests are under requests/. With bun dev serving port 3001, POST a request to /api/export/character and poll /api/export/character/status/{id} until done. Scratch helpers are under tmp/texture-bake-performance/scripts/: size-variants.ts exports all six variants, make-preview-mdx.ts serializes previews, preserve-model.ts measures only referenced files, and texture_bake_closeup_tmp_test.go archives the fixed-camera capture probe. No capture-only test remains in the production package. The six mobile previews use http://192.168.0.195:3001/viewer?model={firehawk,troll,dressing}-size-{full,half}.mdx.
