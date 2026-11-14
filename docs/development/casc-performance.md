# CASC metadata optimization

Measured 2026-11-14 on Windows amd64, Ryzen 9 5900X, warm local files.
Production hosting was not changed. Results include component benchmarks and
fresh-process local startup plus a first character conversion with warm caches.

## Changes

- Encoding lookup keeps Blizzard's decoded native pages with small record-offset
  indexes, using two binary searches instead of expanding every key into a Go map.
  Pages remain resident in memory; this is not disk-backed/mmap lookup yet.
- Archive lookup retains constant-time hashing, with binary 16-byte keys and
  12-byte locations referencing archive IDs. Temporary per-archive maps and hex
  strings are removed. Duplicate locations select CDN configuration order.
- Master listfile parsing streams CR/LF-delimited records, avoiding regexp line
  splitting and per-record token slices. Individual names no longer retain the
  entire original master text after selection of one build.
- No SQLite, new dependencies, generated lookup database or cache format change.
  Local-install CDN fallback uses the same compact archive lookup.

## Whole-process startup and first export

Measured on Windows amd64 with a local retail installation, build 12.1.5.70783,
using fresh processes and warm filesystem/download caches. The baseline for this
round already includes the native encoding/archive changes below and removal of
the unused preload reverse map. These are incremental savings over that baseline.

| Measurement | Before this round | Final |
|---|---:|---:|
| Startup peak working set | 2.86–2.99 GiB | 1.56 GiB |
| Idle after startup, without any export | 2.86–2.99 GiB | 0.98 GiB |
| Peak through first Lich King export | 2.97 GiB | 1.82 GiB |
| Idle after that export | 1.31 GiB | 1.17 GiB |
| Startup cumulative allocation, representative run | 4.83 GiB | 2.63 GiB |

Startup reached ready in 10.7–15.6 seconds before and 8.4–10.0 seconds after in
the recorded runs. TACT/network latency and machine load affect wall time. The
first export's handler completed in 5.35 seconds before and 3.61–3.95 seconds after;
these timings are observations, not isolated CPU benchmarks.

The 2,731,140-byte Lich King MDL and all seven BLP textures have identical SHA-256
hashes before/after. Browse counts remain 146,529 models and 790,306 textures.

Additional changes responsible for these savings:

- Store local journal index keys as nine binary bytes and locations as 12-byte
  values, removing per-key hex strings and machine-sized integer fields.
- Read the listfile from disk through a bounded scanner after root selection,
  skip other-build names before allocating strings, pre-size maps, and transfer
  the selected ID map instead of copying it. Refresh downloads stream to a
  temporary file and replace the cache only after success.
- Inflate framed BLTE blocks directly into the decoded destination. Keep the
  headerless grow-to-fit path and validate decoded lengths/checksums.
- Store locale collation keys in fixed-size chunks. The former growing buffer
  kept old allocations alive through earlier key slices. English collation
  rules and ordering are preserved.
- Reclaim temporary memory after root decoding and once both startup indexes
  finish, so idle memory no longer depends on an export triggering collection.
- Retain only fields consumed by model/texture/item/creature DB2 caches, and
  process creature tables sequentially. Default DB2 parsing still retains all
  fields; projected reads preserve IDs, copy rows and cursor decoding.

Validation includes all 2,129,238 local journal index entries, all encoding keys,
all master-listfile names, BLTE decoded-byte parity, collation parity, and every
selected field/ID in six real DB2 tables (732,046 rows total).

These are local Windows measurements for startup and one character export.
Remote-CDN startup, concurrent exports, large map exports and a fully uncached
download run still need target-host measurement before sizing a VPS.

## Initial component benchmarks

Real fixtures: 1,403 cached archive indexes (125,324,364 bytes, 5,047,365 unique
keys), one cached retail encoding file (2,882,697 content keys, 26,942 pages), and
a 153,916,679-byte master listfile (2,277,757 IDs/names). The index directory
contains a cached collection, not necessarily exactly one current build's set.

| Component | Previous load | Optimized load | Allocated bytes, previous / optimized |
|---|---:|---:|---:|
| Archive collection | 2.44–2.62 s | 1.20–1.25 s | 1,588 / 663 MB |
| Encoding table | 1.04–1.34 s | 0.26–0.29 s | 873 / 485 MB |
| Master listfile | 7.20–7.78 s | 1.62–1.66 s | 1,107 / 597 MB |

Allocated bytes count all allocations during loading, not live RAM. Explicit
post-GC retained-heap measurement:

| Component | Previous | Optimized |
|---|---:|---:|
| Archive collection | 602.35 MiB | 256.33 MiB |
| Encoding table | 192.15 MiB | 118.50 MiB |

Those two fixture tables retain about 420 MiB less heap (53% reduction).
The whole-process measurements above are separate; these component figures
alone do not establish a hosting size.

Rotating 1,024-hit-key lookup microbenchmark:

| Lookup | Previous | Optimized | Allocations |
|---|---:|---:|---:|
| Archive | 15.87 ns | 32.24 ns | 0 / 0 |
| Encoding | 48.68 ns | 317.2 ns | 1 / 1 (16 bytes) |

Encoding adds about 0.27 microseconds per lookup. This is an intentional
time-for-memory tradeoff. Network downloads,
DB2 initialization, root parsing and conversion working memory still remain.

## Reproduce

Set `CASC_INDEX_BENCH_DIR`, `CASC_ENCODING_BENCH_FILE`, and
`CASC_LISTFILE_BENCH_FILE` to trusted existing cache files. No fixture downloads
or production access happen inside tests.
Set `CASC_LOCAL_INDEX_BENCH_DIR` to a trusted local installation's `Data/data`
directory for exhaustive local-journal parity.

From `golang/`:

```sh
go test ./internal/wow/archive/casc -v
go test ./internal/wow/archive/casc -run '^$' -bench 'BenchmarkReal(ArchiveLoad|EncodingLoad|MasterListfile)' -benchtime=1x -count=3 -benchmem
go test ./internal/wow/archive/casc -run TestRealIndexRetainedHeap -v
go test ./internal/wow/archive/casc -run '^$' -bench BenchmarkRealIndexLookups -benchmem
go test ./internal/wow/... ./internal/server/... ./internal/stringsort/... ./internal/converter/character -skip '^TestLocalLichKingCapture$'
```

To compare all projected DB2 fields with full parsing, set
`WOW_DB_PROJECTION_VALIDATION=1` and `CASC_LOCAL_WOW`, then run:

```sh
go test -tags integration_tests ./internal/wow/db -run '^TestRealProjectedModelCacheTables$' -v
```

Real-data tests exhaustively compare every fixture lookup with the previous
implementation. Unit cases cover native page hits/misses, multi-encoding entries,
reset, truncated records, native archive page padding and listfile line endings.
Legacy parsers remain test-only reference implementations.

The broad regression command above passed. `TestLocalLichKingCapture` depends
on a separately running external server and remains excluded; the isolated
before/after MDL export comparison exercises real conversion without that test.
Native visual screenshots and race tests were not run.

## Startup allocation follow-up

The preloaded reverse filename map had no readers except memory diagnostics.
Removed it: build selection still creates the actual reverse lookup from selected
ID/name pairs. Final lookup behavior, duplicate IDs and build filtering are tested.
On the same 2,277,757-entry listfile, cumulative parser allocations fall further
from 596.5 MB to 372.8 MB, and parsing from 1.62–1.66 s to 0.958–0.965 s.
These are incremental improvements over the streaming parser above.

The subsequent whole-process work above removes the master byte buffer/map
overlap, extra inflated block buffers, and growing collation arena. The encoding
fixture still occupies 179.35 MiB encoded and declares 190.96 MiB decoded;
encoded/decoded buffers and copied resident encoding pages can still overlap.
Root records, result caches and conversion data also remain resident as needed.
