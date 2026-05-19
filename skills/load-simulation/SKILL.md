---
name: load-simulation
description: Use this when the user wants to test client behavior under slow responses with realistic payloads. Combines the delay tool (to introduce latency) with the random_data tool (to produce a test payload of the requested shape).
tags:
  - mock
  - testing
  - performance
---

# load-simulation

Use this when the user wants to test how their client behaves under slow
responses carrying realistic test payloads - e.g. verifying timeouts,
streaming buffers, progress indicators, or retry logic.

## When to use

- The user asks to "simulate slow responses", "test timeout handling",
  "produce a delayed payload", or similar.
- The user wants a response that combines latency with a non-trivial body
  (rather than just an `echo` round-trip).
- The user is benchmarking client-side handling of slow upstreams.

Do NOT use this skill for pure latency tests (call `delay` directly) or for
pure data generation (call `random_data` directly).

## Workflow

1. Parse the user's request for two parameters:
   - **Delay**: seconds to wait before responding. Default to `2` if not
     specified.
   - **Payload shape**: the `data_type` and `count` to pass to
     `random_data`. Default to `data_type: json`, `count: 1` if not
     specified.
2. Call `random_data` first with the chosen `data_type` and `count` to
   build the payload. Capture the JSON result.
3. Call `delay` with `duration_seconds` set to the requested delay, and
   `message` set to a short summary of the payload (e.g.
   `"5 json records ready"`).
4. Return the user a single combined response that reports:
   - The actual delay observed (from the `delay` tool's response).
   - The full random payload (from step 2).

## Tools

- `random_data` - generates the payload.
- `delay` - introduces the configured latency.
