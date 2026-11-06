// The built-in scheduler plugin: it decides policy, not arithmetic. The
// schedule's maths stays the host's — the plugin says how much retention to aim
// for and what to call the scheduler, so that every review it schedules can be
// stamped with a name (ADR 0007).
//
// It also answers what a finished session is worth saying. Lines it returns are
// markdown the host renders; a plugin that returns none lets the host word the
// numbers itself.

export function policy() {
  return {
    family: "fsrs",
    version: "1",
    retention: 0.9,
  };
}

// identity is what a review record carries: family and version, and the flags
// that would change an interval.
export function identity() {
  const p = policy();
  return [p.family + "-" + p.version].join("");
}

// summary answers a finished session. The built-in one is quiet and says
// nothing of its own: the host words the numbers, in the user's language.
export function summary(session) {
  return {
    tone: "quiet",
    lines: [],
  };
}
