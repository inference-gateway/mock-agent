---
name: connectivity-check
description: Use this when the user wants to verify the agent is reachable and responding correctly. Invokes the echo tool with a known payload and confirms the round-trip succeeded.
tags:
  - mock
  - testing
  - connectivity
---

# connectivity-check

Use this when the user wants a quick health check - "are you there?", "ping",
"is the agent up?", or any request to confirm the A2A transport works end to
end before running a real workload.

## When to use

- The user says "ping", "are you up", "healthcheck", "smoke test", or similar.
- The user is debugging a client integration and wants to confirm the
  round-trip works before exercising more complex flows.
- The user explicitly asks for an echo round-trip.

Do NOT use this skill for general-purpose echo requests where formatting,
multi-line payloads, or domain-specific behavior matters - just call the
`echo` tool directly.

## Workflow

1. Call the `echo` tool with `message` set to a short, recognizable string
   (e.g. `"ping"` or a timestamp the user provided).
2. Inspect the JSON response - it should contain `"status": "success"` and an
   `"echo"` field that exactly matches the input.
3. Report back: confirm the round-trip succeeded, quote the response, and
   note the reported `length` so the user can see the payload was processed.

## Tools

- `echo` - sole tool used by this skill.
