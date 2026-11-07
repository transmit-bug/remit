# ADR 0006 — Extension substrate: an in-process JS sandbox beside the process protocol

- **Status:** accepted
- **Date:** 2026-11-07
- **Supersedes:** the two rulings in #9 that read *"no in-process plugins (Go
  plugin ABI is brittle)"* and *"no sandbox in v1 (a local process is trusted
  as the user trusts it)"*. ADR 0007 supersedes the rest of #9's extension
  surface where it disagrees.
- **Related:** #9, #67 (spike), #68, #69, ADR 0007

## Context

#9 fixed Remit's extension form as an out-of-process program speaking
newline-delimited JSON (`internal/ext`), and ruled out two alternatives:
in-process plugins, because Go's plugin ABI is brittle; and a sandbox, because
a local process is trusted as its user is.

Both rulings reasoned about **Go plugins and local processes**. Neither covers a
script host: the language ABI is the host API, and an engine that limits time,
memory and reachable globals supplies the sandbox cheaply.

#67 measured `moejs` (`github.com/MoeForge/moejs`), a pure-Go JavaScript engine,
at v0.1.0-alpha.11:

- `CGO_ENABLED=0` cross-compiles for darwin/arm64, linux/amd64 and
  windows/amd64 (7.3 / 7.6 / 7.9 MB binaries; +6.2 MB over a hello world).
- A card-derivation call costs 1.06–1.2 µs; `NewRuntime` + `Load` 1.0–1.35 ms.
- `Interrupt` stops an endless loop in 19–25 µs; `MemoryLimit` trips with
  `ErrMemoryLimit`; a panic inside a Go host function returns `*InternalError`
  and the process survives.
- `go.sum` grows by 6 lines and one direct requirement.

Not measured: per-runtime resident memory (the machine's `HeapAlloc` delta was
unreliable; the upstream claim of 81 KiB/runtime is treated as unverified).

## Decision

**Two substrates, one contract.** `extension.json` gains `runtime`, which is
`"exec"` when absent and `"js"` for a JavaScript extension loaded into an
in-process `moejs` runtime.

- The contract of #9 does not change: a per-library explicit enable, `describe`
  and `on_event`, one-second per-call budget, failure disables rather than
  retries, reads through the agent's read-only surface, writes through a token,
  no UI injection, no direct SQLite. *Amended (#142):* a plugin reaches the
  host in process through `ext.Host` — the same read/write layer
  `internal/agentapi` serves, not an HTTP round trip and not a token — and the
  six named capabilities of ADR 0007 are the door.
- New extensions are written in JavaScript. `exec` remains for an extension that
  needs a native library or a long computation, and for the operating systems'
  own tooling.
- Limits are part of the substrate, not a tuning knob: 1 s per call (a timer
  plus `Interrupt`), `MemoryLimit` of 64 MiB per runtime, a pool of two
  runtimes per extension, three consecutive failures disable it.

  *Amended (2026-11-08):* the ruling above also said no plugin ever runs on the
  UI thread. Today a contributed command and a page's data run off the frame,
  but `deriveBlock` is asked from the path that renders a card — on a cache
  miss, once per block text, with the failure cached too. The bound is the 1 s
  budget and the cache, not the thread; #141 and later screens keep the rule
  for everything that can wait.

## Known risk, stated plainly

The in-process substrate is **not equivalent** to #9's process isolation. The
capability surface, `Interrupt` and `MemoryLimit` bound what a plugin may do and
how long it may take; they do not protect the app from a memory-safety bug
inside the engine itself, which would take the process down. #9's isolation
bought that protection by paying a process, a wire protocol and a per-platform
binary. We trade that for one-file distribution.

`moejs` is an alpha (0.1.0-alpha.11). The version is pinned; upgrading it is a
change that re-runs #67's probes rather than a routine bump.

## Consequences

- An extension ships as one file instead of three platform binaries.
- The app grows by ~6 MB and accepts a dependency whose stability is younger
  than the rest of the tree.
- `internal/ext` grows a second loader; the protocol types are shared, so a
  JavaScript extension and a process extension see the same JSON.
- The capability model (ADR 0007) is what keeps this trade honest: without it,
  an in-process plugin would be strictly worse than what #9 had.
