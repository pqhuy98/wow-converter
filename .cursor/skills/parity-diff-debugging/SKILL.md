---
name: parity-diff-debugging
description: Debug parity mismatches between TypeScript and Go exports by reproducing one failing case, adding matched stage logs on both sides, and identifying the earliest divergent stage. Use when TS and Go artifacts differ, parity loops fail, or the user asks to trace where a conversion mismatch begins.
disable-model-invocation: true
---
# Parity Diff Debugging

## Goal

Find the first stage where TypeScript and Go stop matching.

Do not guess from the final `.mdl` or `.mdx` diff alone. Work backward from the artifact difference into the pipeline and prove the earliest divergent stage.

## Workflow

1. Reproduce one failing case only.

   Prefer a single focused case over the whole parity loop:

   ```powershell
   bun scripts/retail-mdl-parity-loop.ts --case-idx=<N> --debug --diff-lines=80
   ```

   Use the full loop only after the focused case is fixed:

   ```powershell
   bun run parity:mdl
   ```

2. Inspect the actual artifact diff first.

   Compare the generated files under:

   - `.parity-artifacts/loop-ts-retail-mdl`
   - `.parity-artifacts/loop-go-retail-mdl`

   Identify the first meaningful mismatch:

   - missing keyframes
   - extra keyframes
   - changed texture or material IDs
   - reordered sections
   - changed transforms
   - changed counts

3. Reduce the problem to the smallest comparable object.

   Do not log entire models unless necessary. Target the exact object that differs:

   - one emitter
   - one bone
   - one material
   - one sequence
   - one animation track

4. Add matched logs on both TS and Go.

   Log the same object at the same named stage in both implementations.

   Recommended stage labels:

   - `extract`
   - `after-attack-filter`
   - `after-walk-scale`
   - `after-decay`
   - `after-portrait`
   - `after-concat`
   - `before-opt`
   - `after-opt`

   Keep the log structure identical across languages:

   - same stage name
   - same object identifier
   - same field names
   - same ordering

5. Gate debug logs behind env vars.

   Never leave unconditional parity logs in the code.

   Use env vars such as:

   - `WOW_DEBUG_<AREA>=1`
   - `WOW_DEBUG_<AREA>=substring`

   That allows focusing on one model or node without flooding the output.

6. Compare stage by stage.

   For each stage:

   - if TS and Go still match, move later
   - if they differ, the bug is in the step that just ran

   The question is always:

   "What is the first stage where TS and Go no longer have the same data?"

7. Classify the mismatch before fixing it.

   Put it in one of these buckets:

   - raw extraction mismatch
   - conversion mismatch
   - modification mismatch
   - optimization mismatch
   - serialization mismatch

   This keeps the fix targeted.

## High-Value Transition Points

In this repo, these transitions often cause parity drift:

- sequence filtering or remapping
- sequence duration scaling
- decay insertion
- portrait cloning
- attack sequence concatenation
- material and texture dedupe
- keyframe optimization
- final MDL formatting

## Logging Guidelines

Log only what explains the diff.

Good examples:

- raw source timestamps and values
- converted MDL keyframe summaries
- material or texture ID summaries
- sequence intervals
- keyframe counts per node

Bad examples:

- full JSON dumps of the entire model
- repeated noisy logs without stage labels
- logs enabled by default

## Fix Strategy

1. Prove the earliest bad stage.
2. Patch only that behavior.
3. Re-run the focused failing case.
4. Re-run the broader parity loop.
5. Remove temporary debug hooks you added, unless they are generic, reusable, and env-gated.

## Verification

After the fix, run:

```powershell
go test ./internal/formats/mdl ./internal/converter/character
bun scripts/retail-mdl-parity-loop.ts --case-idx=<N> --diff-lines=40
bun run parity:mdl
```

## Rules Of Thumb

- Never debug from the final diff alone.
- Never compare whole models before isolating the smallest bad object.
- Never add ungated logs.
- Always prove the first divergent stage.
- Fix one stage at a time.
