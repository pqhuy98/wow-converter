# ADR 0002: Bake unsupported WoW shader combinations in Go

**Status:** accepted

Classic WC3 material layers cannot directly express every WoW texture combiner or independently addressed UV sample. The direct M2 converter owns the shared M2/WMO baking implementation. Each export request can opt in using the texture baking options exposed in the UI and API.

Use native WC3 layers where the shader combination can be represented safely. Otherwise bake the combined result into deterministic atlases, retaining geometry, skinning and animation. Atlas packing, temporal sampling and spatial budgets belong in this shared implementation rather than per-model workarounds.

The implementation, request defaults, known limits and runnable checks live in [texture baking](../rendering/texture-baking.md).
