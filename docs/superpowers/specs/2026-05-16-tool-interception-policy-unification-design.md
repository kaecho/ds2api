# Tool Interception Policy Unification Design

**Goal:** Unify the decision about when tool-call markup should be buffered, hidden from visible stream output, and parsed into tool calls, while keeping each compatibility surface's response protocol unchanged.

**Architecture:** Keep OpenAI, Claude, and Gemini response formatting in their own adapters, but route all tool-interception decisions through the same `promptcompat.ToolChoicePolicy` chain: request normalization -> stream runtime gating -> finalize-time parsing.

**Tech Stack:** Go backend (`internal/promptcompat`, `internal/httpapi/openai`, `internal/httpapi/claude`, `internal/httpapi/gemini`, `internal/assistantturn`), Go tests.

---

## Problem

The recent OpenAI and Claude fixes corrected the same class of bug: raw DSML/XML tool markup leaked into visible streaming output when the adapter decided whether to intercept tool content by looking at declared tool names instead of the actual tool-choice policy.

Gemini currently has two different paths:

1. **Proxy-via-OpenAI path** already inherits the OpenAI fix because it calls `h.OpenAI.ChatCompletions(...)`.
2. **Native Gemini path** still has the old shape:
   - stream buffering is gated by `len(toolNames) > 0`
   - finalize-time turn building does not pass `ToolChoice`
   - Gemini request normalization does not currently populate `StandardRequest.ToolChoice`

This makes Gemini native behavior inconsistent with OpenAI and Claude and leaves the same bug class available on a third surface.

---

## Design Goals

1. Make Gemini native streaming obey the same tool-interception policy as OpenAI and Claude.
2. Prevent future adapter work from reintroducing the same bug by centralizing the policy decision.
3. Keep protocol-specific stream emitters separate.
4. Avoid bundling this work with stricter declared-tool enforcement or parser hardening.

---

## Non-Goals

1. Do **not** merge OpenAI, Claude, and Gemini stream runtimes into one shared runtime.
2. Do **not** change the wire format of any adapter:
   - OpenAI keeps Chat/Responses SSE output
   - Claude keeps `content_block_*` events
   - Gemini keeps `candidates[].content.parts`
3. Do **not** implement strict filtering of parsed tool calls against declared available tools in this design.
4. Do **not** redesign tool prompting or prompt rewriting.

---

## Current State Summary

### Shared policy source already exists

`promptcompat.StandardRequest.ToolChoice` is already the right logical source of truth for whether tool use is allowed, forbidden, required, or forced.

### OpenAI and Claude already consume that policy

OpenAI and Claude stream handling now gate interception by policy rather than by declared tool-name count, and they pass `ToolChoice` into finalize-time turn building.

### Gemini native does not yet consume the full policy chain

The native Gemini path still relies on declared tool-name presence in stream runtime setup and omits `ToolChoice` when building the final turn snapshot.

---

## Proposed Design

### 1. Define a small shared policy helper

Create a neutral shared package for tool-interception policy decisions. The package should be outside `internal/httpapi/openai/shared` because Gemini and Claude should not depend on an OpenAI-named package for a cross-surface concern.

Recommended package shape:

- `internal/toolpolicy/policy.go`
- `internal/toolpolicy/policy_test.go`

Recommended functions:

```go
func ShouldBufferToolContent(policy promptcompat.ToolChoicePolicy) bool
func ShouldParseToolCalls(policy promptcompat.ToolChoicePolicy) bool
```

Behavior:

- `ToolChoiceNone` -> `false`, `false`
- `ToolChoiceAuto` -> `true`, `true`
- `ToolChoiceRequired` -> `true`, `true`
- `ToolChoiceForced` -> `true`, `true`

This helper intentionally stays thin. It does not own stream accumulation, protocol formatting, or tool parsing. It only answers the shared gating question.

### 2. Make `StandardRequest.ToolChoice` the required strategy input

All adapter request normalizers should treat `StandardRequest.ToolChoice` as required adapter state, even when the surface does not expose a rich tool-choice input yet.

For Gemini native, the first implementation may populate it with the effective supported policy for current request shapes. If Gemini later grows support for more explicit function-calling configuration, the mapping should still terminate at `StandardRequest.ToolChoice`.

### 3. Require each stream runtime to consume the shared helper

Each adapter remains responsible for its own visible streaming protocol, but must use the shared helper to decide whether tool markup is buffered or allowed to pass through as visible text.

That means:

- OpenAI runtime continues to emit OpenAI deltas
- Claude runtime continues to emit Claude content blocks
- Gemini runtime continues to emit Gemini candidate parts

But each runtime computes its buffering behavior from the same helper.

### 4. Require finalize-time parsing to receive the same policy

Every call into `assistantturn.BuildTurnFromCollected(...)` or `assistantturn.BuildTurnFromStreamSnapshot(...)` must receive the matching `ToolChoice`.

This keeps stream-time and finalize-time behavior aligned:

- if policy forbids tool use, visible output stays visible and tool parsing is disabled
- if policy allows tool use, raw DSML/XML can be reconstructed into tool calls without leaking markup

---

## Adapter Changes

### Gemini native request normalization

Modify `internal/httpapi/gemini/convert_request.go` so the returned `promptcompat.StandardRequest` explicitly includes `ToolChoice`.

Initial requirement:

- Do not leave `ToolChoice` as an implicit zero-value accident.
- When the Gemini-native request shape does not expose an explicit function-calling control, set `ToolChoice` to `promptcompat.DefaultToolChoicePolicy()` deliberately.
- If Gemini-native request normalization later learns an explicit "forbid tools" or "force tool" control, that mapping should still terminate at `StandardRequest.ToolChoice`.

This design does not require full Gemini-specific `functionCallingConfig` support yet. It only requires the normalized request to carry an explicit policy forward.

### Gemini native stream runtime

Modify `internal/httpapi/gemini/handler_generate.go` and `internal/httpapi/gemini/handler_stream_runtime.go` so the runtime constructor receives `ToolChoice` and derives buffering from the shared helper instead of `len(toolNames) > 0`.

Required effects:

- tool-allowed requests buffer tool markup during streaming
- `tool_choice:none` requests do not convert visible content into tool calls
- behavior matches the recent OpenAI and Claude fixes

### Gemini native finalize path

Modify Gemini finalize-time turn construction so `assistantturn.BuildTurnFromStreamSnapshot(...)` receives `ToolChoice`.

Required effects:

- `tool_choice:none` preserves visible output semantics
- tool-enabled requests can still produce Gemini `functionCall` parts at finalize time

### OpenAI and Claude call sites

OpenAI and Claude are already using policy-driven behavior. As part of this unification, their existing inline `!toolChoice.IsNone()` checks should switch to the shared helper as a mechanical cleanup, not a behavioral rewrite.

---

## Data Flow After This Change

### OpenAI / Claude / Gemini native

1. Surface request is normalized into `promptcompat.StandardRequest`
2. `ToolChoice` is set explicitly on the normalized request
3. Stream runtime receives `ToolChoice`
4. Shared tool-policy helper decides:
   - whether to buffer tool content in stream time
   - whether finalize-time tool parsing is enabled
5. Adapter-specific formatter emits its own protocol output
6. `assistantturn` receives the same `ToolChoice` and applies the same policy during final turn construction

This keeps one policy with multiple protocol-specific renderers.

---

## Why This Design

### Why not unify the entire stream runtime?

Because the repeated bug is not caused by three incompatible protocols. It is caused by the same policy decision being reimplemented in multiple adapters. The profitable shared layer is the decision layer, not the protocol layer.

### Why not keep the fixes adapter-local?

Because the same regression has already appeared across multiple surfaces. Leaving the logic inline in each adapter increases the chance that future compatibility work repeats the old gating mistake.

### Why not include strict declared-tool filtering now?

Because that is a deeper parser-hardening change with a different risk profile. It should be designed and tested separately rather than bundled into a policy unification pass.

---

## Testing Plan

### Shared helper tests

Add focused tests for the helper package to lock the gating matrix:

- `none` -> no buffering, no parsing
- `auto` -> buffering and parsing enabled
- `required` -> buffering and parsing enabled
- `forced` -> buffering and parsing enabled

### Gemini native normalization tests

Add or update tests for `internal/httpapi/gemini/convert_request.go` to verify that `StandardRequest.ToolChoice` is populated explicitly.

### Gemini native streaming tests

Add tests covering:

1. **Tool-allowed request with no declared tool names available to the old gate**
   - visible stream output must not leak raw DSML/XML tool markup
   - finalize output should produce Gemini `functionCall` parts

2. **Tool-forbidden request**
   - tool parsing must stay disabled
   - visible output must remain text-oriented rather than being converted into tool calls

### Regression alignment tests

Where practical, mirror the already-added OpenAI and Claude regression scenarios so the three surfaces are tested against the same logical behavior.

---

## File Impact

### New

- Create `internal/toolpolicy/policy.go`
- Create `internal/toolpolicy/policy_test.go`

### Modify

- Modify `internal/httpapi/gemini/convert_request.go`
- Modify `internal/httpapi/gemini/handler_generate.go`
- Modify `internal/httpapi/gemini/handler_stream_runtime.go`
- Modify relevant Gemini tests
- Optionally replace duplicated policy checks in:
  - `internal/httpapi/openai/...`
  - `internal/httpapi/claude/...`

---

## Rollout Order

1. Add the shared tool-policy helper and tests.
2. Populate `StandardRequest.ToolChoice` in Gemini native normalization.
3. Thread `ToolChoice` through Gemini native stream runtime construction.
4. Switch Gemini buffering/finalize gating to the shared helper.
5. Add Gemini-native regression tests.
6. Optionally replace equivalent OpenAI/Claude inline checks with the shared helper.

This order keeps the change incremental and makes each layer testable before the next one depends on it.

---

## Risks and Mitigations

### Risk: accidental protocol-level refactor

**Mitigation:** keep the shared helper thin and policy-only.

### Risk: Gemini-native policy mapping becomes over-scoped

**Mitigation:** only carry forward the currently supported behavior into `StandardRequest.ToolChoice`; do not expand request-surface semantics in the same change.

### Risk: mixing parser hardening into this task

**Mitigation:** keep declared-tool strict filtering out of scope and document it as separate follow-up work.

---

## Follow-Up Work

This design intentionally leaves one related hardening task separate:

- strict enforcement that parsed tool calls must match declared available tools

That follow-up belongs in parser/toolcall design, not in adapter policy unification.
