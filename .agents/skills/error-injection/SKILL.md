---
name: error-injection
description: Use this when the user wants to test how their client handles different failure modes. Invokes the error tool across the supported error_type values (validation, timeout, internal, not_found) so the caller can observe each error path.
tags:
  - mock
  - testing
  - error-handling
---

# error-injection

Use this when the user wants to exercise their client's error-handling paths
by deliberately triggering each kind of failure the agent can produce.

## When to use

- The user asks to "test error handling", "simulate failures", "exercise the
  retry path", or similar.
- The user names a specific error condition (validation / timeout / internal
  / not_found) and wants the agent to return it.
- The user is building or debugging a client that needs to behave correctly
  when the agent surfaces an error.

Do NOT use this skill when the user is reporting an actual problem - in that
case, investigate the real cause rather than injecting a synthetic error.

## Workflow

1. Determine scope:
   - If the user named a specific `error_type`, invoke `error` once with
     that value.
   - If the user asked for "all errors" or didn't specify, invoke `error`
     once per supported value, in this order:
     `validation` -> `timeout` -> `internal` -> `not_found`.
2. If the user supplied a custom message, pass it via the `message`
   parameter; otherwise let the tool's default message stand.
3. For each invocation, capture the returned error and report it back to
   the user verbatim so they can match it against their client's handling.
4. Summarize at the end: which error types were exercised and what the
   resulting error messages were.

## Tools

- `error` - sole tool used by this skill. Accepts `error_type` (required,
  one of `validation`, `timeout`, `internal`, `not_found`) and an optional
  `message`.
