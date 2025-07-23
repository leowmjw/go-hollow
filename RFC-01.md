# RFC-01: Adopting Apache Arrow & Iceberg for Hollow-Go Data Representation

| Status | Draft |
|--------|-------|
| Author | Gemini (AI) |
| Stakeholders | Core Hollow-Go maintainers, Data Platform team, SRE, Performance SIG |
| Created | 2025-07-20 |
| Target Release | v0.5.0 |

---

## 1. Abstract
This RFC evaluates replacing the current JSON-encoded `map[string]any` snapshot/delta payloads with columnar Apache Arrow in memory and Apache Iceberg on S3 for long-term storage. The goal is to achieve sub-5 µs p99 read latency, minimise allocations, and provide strong schema typing while maintaining Hollow’s read-only semantics.

## 2. Background & Motivation
The existing implementation serialises snapshots with Go’s `encoding/json`, incurring:
* Excessive CPU from reflection & UTF-8 validation.
* Heap pressure from string keys and interface boxing.
* Lack of explicit schema → run-time type assertions.

Apache Arrow is a language-agnostic, in-memory columnar format offering:
* Zero-copy reads via contiguous buffers.
* Strongly-typed record batches.
* SIMD-friendly vector operations.

Persisting Arrow batches in Apache Iceberg tables enables:
* Schema evolution & partitioning.
* ACID data-lake semantics on S3.
* Interop with Spark/Trino.

## 3. Proposal
1. **In-Memory Layout**
   * Replace `readState` map with Arrow `RecordBatch` backed by Go memory mapped buffers.
   * Maintain an Arrow Schema generated from Hollow type definitions.
2. **Serialisation Path**
   * **Snapshot**: Arrow IPC (Feather V2) framed blobs.
   * **Delta**: Arrow Flight-like streaming of record batch diffs.
3. **Persistence**
   * Stage blobs directly to S3 as Iceberg `data` files using [`arrowport`](https://github.com/TFMV/arrowport) writers.
   * Metastore integration deferred; producer emits Iceberg `manifest` JSON per cycle.
4. **Libraries**
   * [`github.com/apache/arrow/go/v17/arrow`]
   * [`github.com/TFMV/archery`] – high-level builder APIs.
   * [`github.com/TFMV/porter`] – Arrow IPC streaming helpers.

## 4. Detailed Design
### 4.1 Producer Cycle
```
WriteState → Arrow Builder → RecordBatch → IPC → BlobStager
```
* Builders allocate once per cycle, re-used across records.
* Metrics: rows, batch size, build latency.

### 4.2 Consumer Refresh
```
BlobRetriever → mmap IPC file → arrow/ipc.Reader → atomic.Pointer[*RecordBatch]
```
* No JSON unmarshal; fields accessed via Arrow column vectors.
* Look-ups: binary search on dictionary-encoded columns or hash index (optional).

## 5. Expected Benefits
| Dimension | JSON (today) | Arrow (proposed) |
|-----------|--------------|------------------|
| p99 read latency | ~70 µs (est.) | ≤ 5 µs (vector load + SIMD) |
| Memory overhead | ~3× raw data | ≤ 1.1× (shared buffers) |
| Type safety | none | compile-time via schema |
| Interop | Go-only | Arrow Flight, Python, Java, Rust |

## 6. Drawbacks & Risks (Devil’s Advocate)
1. **Complexity Explosion** – Arrow & Iceberg introduce steep learning curves; contributors must understand columnar memory management.
2. **Binary Compatibility** – Arrow format upgrades (v12→v13) may break consumers.
3. **GC Edge Cases** – JNI-style CGo or unsafe pointers may cause leaks or seg-faults if reference counts mis-managed.
4. **Binary Size** – Arrow libs add ~15 MB to binaries; not ideal for serverless.
5. **Delta Semantics** – Hollow’s object graph diff is row-oriented; projecting to columnar deltas may require rewriting diff engine.
6. **Iceberg Overhead** – Manifest + metadata files per cycle increase small-file count on S3, impacting list operations.
7. **On-Disk Footprint** – Arrow IPC is not as storage-efficient as Parquet; may bloat snapshots.
8. **Vendor Lock-In** – TFMV libs are alpha; limited community support compared with core Apache repos.

## 7. Mitigations
* Start with **optional experimental path** behind feature flag.
* Retain JSON snapshot compatibility for rollback.
* Use pure-Go Arrow (`net/godoc`) avoiding CGO where possible.
* Ship memory-usage & latency SLO dashboards.
* Contribute fixes upstream to Arrow-Go project.

## 8. Alternatives Considered
1. **FlatBuffers** – fast but lacks columnar projection and analytics interop.
2. **Cap’n Proto** – zero-copy, strong typing, but poor ecosystem for analytics + S3 table formats.
3. **Protobuf w/ GZIP** – smaller on disk, still requires copy into structs.
4. **Stay with JSON + Code-gen** – easier, but unlikely to hit 5 µs target.

## 9. Decision Matrix
| Option | Read p99 | Writer CPU | Ecosystem | Complexity | Verdict |
|--------|----------|-----------|-----------|------------|---------|
| Arrow  | 🟢 | 🟢 | 🟢 | 🔴 | Candidate |
| FlatBuffers | 🟢 | 🟢 | 🟡 | 🟡 | Disfavoured |
| Cap’n Proto | 🟢 | 🟡 | 🔴 | 🟡 | Disfavoured |
| JSON (current) | 🔴 | 🔴 | 🟢 | 🟢 | Reject |

Green🟢 good, Yellow🟡 moderate, Red🔴 poor.

## 10. Recommendation
Proceed with a **prototype** Arrow snapshot path focused on read-latency benchmarks. Gate behind `ARROW_EXPERIMENT=1` env var. Re-evaluate after:
* 1 M record dataset shows ≤ 5 µs p99 on `c6i.large`.
* Memory overhead <1.2× raw data.
* Integration tests pass for fallback JSON path.

## 11. Unresolved Questions
* Best strategy for dictionary encoding dynamic strings?
* Influence on hot-reload developer loop (mmap semantics on macOS)?
* Iceberg commit atomicity without metastore?

## 12. References
* Archery: <https://github.com/TFMV/archery>
* Porter: <https://github.com/TFMV/porter>
* ArrowPort: <https://github.com/TFMV/arrowport>
* Apache Iceberg Go: <https://github.com/apache/iceberg-go>
* Arrow Flight + Iceberg article: <https://medium.com/@tglawless/apache-iceberg-apache-arrow-flight-7f95271b7a85>
* Arrow Columnar Format Spec: <https://arrow.apache.org/docs/format/Columnar.html>

---

*End of RFC-01*
