#!/usr/bin/env python3
"""Measure one OpenAI-compatible TurboQuant-MLX request.

The server is intentionally managed outside this script. Run the same command
once on an idle Mac and once with the usual Docker containers running, passing
the server PID so the request-time peak RSS can be sampled.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import threading
import time
import urllib.error
import urllib.request


def rss_kib(pid: int) -> int | None:
    result = subprocess.run(
        ["ps", "-o", "rss=", "-p", str(pid)],
        check=False,
        capture_output=True,
        text=True,
    )
    values = []
    for line in result.stdout.splitlines():
        value = line.strip()
        if value.isdigit():
            values.append(int(value))
    return max(values) if values else None


def sample_rss(pid: int, stop: threading.Event, samples: list[int]) -> None:
    while not stop.is_set():
        value = rss_kib(pid)
        if value is not None:
            samples.append(value)
        stop.wait(0.1)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Measure tokens/sec and peak server RSS for one request."
    )
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--server-pid", required=True, type=int)
    parser.add_argument("--label", required=True, help="idle or docker")
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--max-tokens", type=int, default=128)
    parser.add_argument("--model")
    parser.add_argument("--timeout", type=float, default=3600.0)
    args = parser.parse_args()

    payload = {
        "messages": [{"role": "user", "content": args.prompt}],
        "max_tokens": args.max_tokens,
        "stream": False,
    }
    if args.model:
        payload["model"] = args.model

    request = urllib.request.Request(
        args.endpoint,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    samples: list[int] = []
    stop = threading.Event()
    sampler = threading.Thread(
        target=sample_rss,
        args=(args.server_pid, stop, samples),
        daemon=True,
    )
    sampler.start()
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            response_body = response.read()
    except urllib.error.HTTPError as error:
        response_body = error.read()
        raise RuntimeError(
            f"endpoint returned HTTP {error.code}: {response_body.decode('utf-8', 'replace')}"
        ) from error
    finally:
        elapsed = time.perf_counter() - started
        stop.set()
        sampler.join(timeout=1.0)

    response = json.loads(response_body)
    usage = response.get("usage") or {}
    completion_tokens = usage.get("completion_tokens")
    if not isinstance(completion_tokens, int):
        raise RuntimeError(
            "response did not include usage.completion_tokens; "
            "no token-rate estimate was produced"
        )

    peak_rss = max(samples) if samples else None
    result = {
        "label": args.label,
        "endpoint": args.endpoint,
        "server_pid": args.server_pid,
        "completion_tokens": completion_tokens,
        "elapsed_seconds": round(elapsed, 6),
        "tokens_per_second": round(completion_tokens / elapsed, 6),
        "peak_server_rss_mib": round(peak_rss / 1024, 3) if peak_rss is not None else None,
        "rss_samples": len(samples),
    }
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
