# RAW‑Hollow (Go) – PRD v2

> **Change log – 2025‑07‑18**\
> *Replaces Lambda/Streams fan‑out with a Temporal workflow that buffers WAL events and flushes batches every 30 s.  Ensures at‑least‑once processing with automatic retries and deterministic state replay.*

---

## 1  Overview

RAW‑Hollow augments Hollow with a replicated write‑ahead‑log (WAL) stored in DynamoDB.  Each WAL append must eventually result in a blob being materialised (snapshot or delta) and marked **Applied**.  Instead of Lambda triggered by DynamoDB Streams, we use a **long‑running Temporal workflow per dataset** that consumes WAL change‑events, buffers them for 30 s, and writes blobs in deterministic batches.

```
┌─────────┐    PutItem              Signal(NewWalEvent)             ┌────────────────────┐
│ Producer│──────────────▶ DDB WAL ───────────────────────────────▶│ RawHollowWorkflow  │
└─────────┘                            (bridge worker)             │  (Temporal)        │
                                                                    └───────┬────────────┘
                                                                            │Flush every 30 s
                                                  ┌─────────────────────────▼─────────┐
                                                  │  WriteBlobActivity (S3)           │
                                                  ├────────────────────────────────────┤
                                                  │ UpdateStatusActivity (DDB)        │
                                                  └────────────────────────────────────┘
```

### Event bridge

- **wal‑stream‑worker** is a Go service that tails DynamoDB Streams and emits `workflow.SignalWithStart` to Temporal.  One workflow key == one dataset id.  It carries the WAL primary key (SeqNo) and all item metadata so the workflow remains deterministic without needing extra reads.
- The worker at‑least‑once forwards events; duplicate signals are de‑duplicated inside the workflow.

### Workflow contract

| Responsibility  | Details                                                                                                                                                                                            |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Buffer**      | Keep an in‑memory slice of WAL events; keyed by `SeqNo`.  Deduplicate on insert.                                                                                                                   |
| **Flush Timer** | Use `workflow.NewTimer(ctx, 30*time.Second)` to trigger batch flush.                                                                                                                               |
| **Flush**       | In a single TX‑like activity chain: 1) compose delta, 2) `PutObject` to S3, 3) `UpdateItem` **StatusIdx=Applied#YYYY‑MM** for each event.  All activities have `RetryPolicy{MaximumAttempts:  ∞}`. |
| **Progress**    | Persist `LastAppliedSeq` in workflow state so that replay after crash resumes from correct point.                                                                                                  |
| **Completion**  | The workflow is **never** completed; it lives for the dataset lifetime.   A cron‑triggered “maintenance” workflow can compact history.                                                             |

---

## 2  Public API Additions

```go
// Bridge: emits Temporal signals when a new WAL row appears.
func RunWalStreamWorker(cfg WalWorkerConfig) error // blocking

type WalSignalPayload struct {
    DatasetID string
    SeqNo     uint64
    BlobKey   string
    Type      string // SNAPSHOT|DELTA
    Checksum  string
    CreatedAt int64  // epoch seconds
}
```

---

## 3  Temporal Workflow Skeleton

```go
// WorkflowID pattern: "RawHollow-<DatasetId>"
func RawHollowWorkflow(ctx workflow.Context, datasetID string) error {
    var (
        buf         = make(map[uint64]WalSignalPayload)
        lastApplied  uint64
    )

    // signal channel
    sigCh := workflow.GetSignalChannel(ctx, "wal")
    timer  := workflow.NewTimer(ctx, 30*time.Second)

    for {
        selector := workflow.NewSelector(ctx)
        selector.AddReceive(sigCh, func(c workflow.ReceiveChannel, _ bool) {
            var p WalSignalPayload; c.Receive(ctx, &p)
            buf[p.SeqNo] = p // dedup by SeqNo
        })
        selector.AddFuture(timer, func(_ workflow.Future) {
            if len(buf) == 0 { return }
            batch := toSortedSlice(buf) // deterministic order
            err := workflow.ExecuteActivity(ctx, WriteAndApplyBatch, batch).Get(ctx, nil)
            if err == nil {
                for _, e := range batch { delete(buf, e.SeqNo); lastApplied = e.SeqNo }
            }
            timer = workflow.NewTimer(ctx, 30*time.Second) // reset timer regardless
        })
        selector.Select(ctx)
    }
}
```

*`WriteAndApplyBatch`** internally calls two child‑activities: **`PutObject`** and **`UpdateStatus`** to keep the main workflow deterministic.*

---

## 4  Test Suite (Temporal Go SDK)

> Use `temporaltest.NewWorkflowEnvironment(t)` for unit‑style tests and `docker‑compose temporalite` for integration.

| # | Scenario                       | Kind          | Assertions                                                                                                                                          |
| - | ------------------------------ | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **HappyPath**                  | Workflow unit | 3 signals within 30 s → exactly 1 flush, S3 object contains 3 rows, DDB status updated.                                                             |
| 2 | **InterruptionDuringActivity** | Replay unit   | Inject activity failure (`WriteBlobActivity` retries then succeeds) → eventual success; `GetWorkflowHistory` shows contiguous events; no lost rows. |
| 3 | **WorkerCrashMidBuffer**       | Integration   | Send 10 signals, stop worker before 30 s, restart → workflow replays, flushes once; all 10 seqnos applied.                                          |
| 4 | **DuplicateSignals**           | Unit          | Send same `SeqNo` twice; flush still writes once; idempotent update verified via DDB stub.                                                          |
| 5 | **LargeBatchSplit**            | Unit          | Configure `MaxBatchSize=500`; send 1200 signals; expect 3 successive batch activities.                                                              |
| 6 | **TimerDrift**                 | Unit          | Advance virtual time to verify timer reschedules correctly without event loss.                                                                      |

**Test utilities**: fake S3 (`minio`), in‑memory DDB stub, `temporaltest.WorkflowTestSuite`, deterministic mock clock.

---

## 5  Failure Semantics & Idempotency

- **At‑least‑once**: Temporal guarantees signal delivery; deduplication by `SeqNo` avoids double‑apply.
- **No data loss on crashes**: workflow state is re‑loaded from history; buffered but un‑flushed events are re‑played into `buf` on recovery.
- **Activity retries**: infinite exponential back‑off (`MaxBackoff=5m`); alerts routed via metrics if age > 1 h.
- **Compaction**: optional `RawHollowCompactorWorkflow` runs daily to trim WAL history in DDB and archive older blobs.

---

## 6  Migration Notes

1. **Remove Lambda IAM roles** from Terraform modules.
2. Add `temporalite` helm chart + workers to deployment manifest.
3. Bridge worker requires DDB `streams:readRecords` + Temporal `workflow:signalWithStart` permissions.

---

## 7  Open Questions

- Should the bridge worker checkpoint Stream ARN + sequence id in Temporal search attributes or external store?
- Do we need multi‑region Temporal persistence for cross‑AWS‑region datasets? (can be deferred – use Temporal active/active clusters later).

---

*End of PRD v2 – ready for implementation.*

