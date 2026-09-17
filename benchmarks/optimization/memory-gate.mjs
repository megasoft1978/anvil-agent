// The real constraint on this host is not "is the whole system under memory
// pressure" (normal apps running is the expected, permanent condition, not
// contamination) -- it is "does the model server's own dirty footprint fit
// in what's actually free." `footprint -p <pid> -j <file>` reports that
// per-process dirty footprint directly; RSS and wired-system totals do not
// (MODELS.md in the sibling research repo: RSS varied 2-10 GB for one
// unchanged configuration).

// Parse footprint's -j output for one process's dirty footprint in bytes.
// Returns null rather than 0 for "not found" or malformed JSON, so a missing
// measurement is never mistaken for a measured zero.
export function parseFootprintJSON(text) {
  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch {
    return null;
  }
  const processes = parsed?.processes;
  if (!Array.isArray(processes) || processes.length === 0) return null;
  const footprint = processes[0]?.footprint;
  return typeof footprint === "number" && Number.isFinite(footprint) && footprint >= 0 ? footprint : null;
}

// Parse footprint's plain-text "Auxiliary data: phys_footprint: <n> <unit>"
// line into bytes. This avoids a temp file per one-second sample (the -j
// flag only writes a file, never stdout). Units observed: B, KB, MB, GB.
// Returns null, never 0, for a missing or unparsable line.
//
// `footprint -p <arg>` accepts a name OR a pid, and its `-p` matching falls
// back to a partial-name search when an exact pid lookup fails (observed:
// `footprint -p 1` silently matched an unrelated process by name rather than
// reporting pid 1 not found). So this also requires `expectedPid` to appear
// in the report's own `[<pid>]` banner -- if the report is for a different
// process than the one asked about, that is treated the same as no
// measurement, never attributed to the wrong process.
export function parseFootprintText(text, expectedPid) {
  if (expectedPid != null && !new RegExp(`\\[${expectedPid}\\]`).test(text)) return null;
  const match = text.match(/phys_footprint:\s*([0-9.]+)\s*(B|KB|MB|GB)\b/i);
  if (!match) return null;
  const value = Number(match[1]);
  if (!Number.isFinite(value) || value < 0) return null;
  const unit = { B: 1, KB: 1024, MB: 1024 ** 2, GB: 1024 ** 3 }[match[2].toUpperCase()];
  return value * unit;
}

// Evaluate the server-footprint gate against a configured ceiling. This is
// deliberately independent of system-wide "pressure": a host with other apps
// open and a small, well-behaved server footprint should pass; a host that's
// idle but running a server whose footprint has grown past the ceiling
// should not.
export function evaluateFootprintGate(serverFootprintBytes, maxServerFootprintBytes) {
  if (typeof maxServerFootprintBytes !== "number" || !(maxServerFootprintBytes > 0)) {
    return { pass: true, reason: "no server-footprint ceiling configured" };
  }
  if (serverFootprintBytes == null) {
    return { pass: false, reason: "server footprint unavailable; cannot verify it fits the configured ceiling" };
  }
  if (serverFootprintBytes > maxServerFootprintBytes) {
    return {
      pass: false,
      reason: `server footprint ${serverFootprintBytes} bytes exceeds the configured ceiling ${maxServerFootprintBytes} bytes`,
    };
  }
  return { pass: true, reason: null };
}
