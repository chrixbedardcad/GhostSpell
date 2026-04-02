# GhostSpell Adaptive Memory System — Design Document

**Project:** GhostSpell (v0.14.x target)  
**Author:** Chris / SlingMind  
**Date:** April 2026  
**License:** AGPL-3.0  
**Status:** Draft / RFC

---

## 1. Problem Statement

GhostSpell currently treats every text transformation as a stateless, isolated event. The user hits F7 (or Cmd+G), clipboard text gets sent to a local or cloud LLM, and the result is pasted back. There is no continuity between invocations.

This means:

- A user who always rejects formal rewrites will keep getting formal rewrites.
- A bilingual user switching between EN and FR gets no language-aware behavior.
- Repeated corrections (e.g., "don't use Oxford comma," "always use active voice") are lost.
- GhostSpell Cloud subscribers paying $4.99/month get no personalization advantage over the free Ollama tier.

**Goal:** Introduce a lightweight, privacy-respecting memory system inspired by the tiered architecture revealed in the Claude Code source leak (March 2026) — adapted to GhostSpell's scale and constraints.

---

## 2. Design Principles

### 2.1 Memory Is a Hint, Not a Rule

Borrowed directly from Claude Code's "skeptical memory" philosophy. Preferences loaded from memory are injected as **soft context** into the LLM prompt, never as hard system instructions. The model is told:

> "The user has historically preferred X. Consider this, but prioritize what makes sense for the current input."

A remembered preference for "formal tone" should not override context — if the user is transforming a Slack message, casual tone is appropriate regardless of stored preference.

### 2.2 Local-First, Privacy-Preserving

All memory lives on-device by default. GhostSpell Cloud users may opt into synced memory, but the default is local-only. Memory files are never sent to OpenRouter or any provider — only the **derived context string** is included in the prompt.

### 2.3 Bandwidth-Aware Context

The context window is a scarce resource, especially on local models (Ollama/llama.cpp with smaller context). The memory system must be designed so that:

- The hot index is **tiny** (< 500 tokens).
- Detail files are loaded **on-demand** based on relevance.
- Stale or low-confidence memories are pruned automatically.

### 2.4 Graceful Degradation

If memory files are missing, corrupted, or empty, GhostSpell operates exactly as it does today — stateless transformation. Memory is additive, never a dependency.

---

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                   GhostSpell Core                    │
│                                                      │
│  ┌──────────┐    ┌──────────┐    ┌───────────────┐  │
│  │ Clipboard │───▶│ Transform│───▶│  Paste-back   │  │
│  │  Capture  │    │  Engine  │    │               │  │
│  └──────────┘    └────┬─────┘    └───────────────┘  │
│                       │                              │
│                       ▼                              │
│              ┌────────────────┐                      │
│              │ Memory Context │                      │
│              │   Assembler    │                      │
│              └───────┬────────┘                      │
│                      │                               │
│         ┌────────────┼────────────┐                  │
│         ▼            ▼            ▼                  │
│   ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│   │  Layer 1 │ │  Layer 2 │ │  Layer 3 │           │
│   │  Index   │ │  Topics  │ │  Journal │           │
│   │ (always) │ │(on-demand)│ │ (never   │           │
│   │          │ │          │ │  loaded) │           │
│   └──────────┘ └──────────┘ └──────────┘           │
│                                                      │
│              ┌────────────────┐                      │
│              │  Consolidator  │ (background goroutine)│
│              │  (mini-dream)  │                      │
│              └────────────────┘                      │
└─────────────────────────────────────────────────────┘
```

---

## 4. The Three Memory Layers

### Layer 1 — Index (`memory/index.json`)

**Always loaded. Always tiny.**

A flat JSON file containing one-line preference summaries. Target size: **< 500 tokens** (roughly 30-40 entries at ~12 tokens each). This is the only layer injected into every LLM call.

```json
{
  "version": 1,
  "updated_at": "2026-04-02T14:30:00Z",
  "entries": [
    {
      "key": "tone_preference",
      "value": "casual-professional, avoids overly formal language",
      "confidence": 0.85,
      "source": "consolidator",
      "last_seen": "2026-04-02T14:22:00Z"
    },
    {
      "key": "language_primary",
      "value": "en",
      "confidence": 0.95,
      "source": "explicit",
      "last_seen": "2026-04-02T14:30:00Z"
    },
    {
      "key": "language_secondary",
      "value": "fr",
      "confidence": 0.90,
      "source": "inferred",
      "last_seen": "2026-04-01T09:15:00Z"
    },
    {
      "key": "style_active_voice",
      "value": "strongly prefers active voice over passive",
      "confidence": 0.92,
      "source": "consolidator",
      "last_seen": "2026-04-02T13:00:00Z"
    },
    {
      "key": "reject_pattern_oxford_comma",
      "value": "user consistently removes Oxford commas from output",
      "confidence": 0.78,
      "source": "consolidator",
      "last_seen": "2026-03-30T18:45:00Z"
    }
  ]
}
```

**Index rules:**

- Max 40 entries. If full, lowest-confidence entry is evicted.
- Each `value` field must be ≤ 120 characters (enforced on write).
- `confidence` is a float 0.0–1.0, decays over time if not reinforced.
- `source` is one of: `explicit` (user told us), `inferred` (pattern detected), `consolidator` (from mini-dream).

### Layer 2 — Topic Files (`memory/topics/*.json`)

**Loaded on-demand. Richer detail.**

Topic files contain supporting evidence for index entries. They are only loaded when the Memory Context Assembler determines they are relevant to the current transformation.

```
memory/topics/
├── tone.json           # detailed tone observations
├── formatting.json     # comma, spacing, punctuation prefs
├── languages.json      # EN/FR switching patterns, code-switching
├── domains.json        # tech writing vs casual vs academic
└── rejections.json     # transforms the user undid or rejected
```

Example `tone.json`:

```json
{
  "topic": "tone",
  "observations": [
    {
      "timestamp": "2026-04-01T10:30:00Z",
      "input_context": "slack_message",
      "observation": "User rejected formal rewrite, re-ran with casual prompt",
      "weight": 1.0
    },
    {
      "timestamp": "2026-03-29T15:00:00Z",
      "input_context": "email",
      "observation": "User accepted formal rewrite for email context",
      "weight": 1.0
    }
  ],
  "max_observations": 50,
  "pruned_at": "2026-04-01T00:00:00Z"
}
```

**Topic file rules:**

- Max 50 observations per file (FIFO eviction).
- Total topic directory size capped at 500 KB.
- Files older than 90 days with no reinforcement are archived.

### Layer 3 — Transform Journal (`memory/journal/`)

**Never loaded into context. Source of truth for the Consolidator.**

Raw log of every transformation event. Stored as append-only JSONL.

```jsonl
{"ts":"2026-04-02T14:22:00Z","input_hash":"a3f2...","input_lang":"en","output_lang":"en","model":"llama3.2:3b","prompt_type":"rewrite","accepted":true,"undo":false,"duration_ms":340}
{"ts":"2026-04-02T14:25:00Z","input_hash":"b7c1...","input_lang":"fr","output_lang":"fr","model":"openrouter/claude-haiku","prompt_type":"translate","accepted":true,"undo":false,"duration_ms":1200}
{"ts":"2026-04-02T14:28:00Z","input_hash":"c9d4...","input_lang":"en","output_lang":"en","model":"llama3.2:3b","prompt_type":"rewrite","accepted":false,"undo":true,"duration_ms":280}
```

**Journal rules:**

- Input text is **never stored** — only a truncated hash for deduplication.
- Rotated daily. Files older than 30 days are deleted.
- The Consolidator reads the journal; nothing else does.

---

## 5. Memory Context Assembler

The assembler runs on every F7 invocation, between clipboard capture and the LLM call. Its job is to construct a memory context string that gets appended to the transformation prompt.

### Assembly Algorithm

```
1. Load Layer 1 index (always — it's tiny)
2. Analyze current clipboard input:
   a. Detect language (EN/FR/mixed)
   b. Estimate domain (casual/formal/technical/academic)
   c. Estimate length class (short/medium/long)
3. Select relevant Layer 2 topics based on (2):
   - If language != primary → load languages.json
   - If domain is ambiguous → load tone.json + domains.json
   - If input contains patterns matching known rejections → load rejections.json
4. Assemble context string (budget: max 800 tokens total):
   a. Index entries → ~500 tokens
   b. Selected topic summaries → ~300 tokens
5. Inject as soft context after the system prompt, before the user input
```

### Prompt Injection Format

```
<ghost_memory confidence="soft">
User preferences (treat as hints, not rules):
- Tone: casual-professional, avoids overly formal language (high confidence)
- Strongly prefers active voice (high confidence)
- Removes Oxford commas from output (medium confidence)
- Primary language: English, also writes in French
- Context: input appears to be a casual message (inferred)

Recent relevant pattern:
- User rejected formal rewrites for short messages (3 times this week)
</ghost_memory>
```

---

## 6. The Consolidator (Mini-Dream)

A background goroutine that periodically reviews the Transform Journal and updates Layers 1 and 2. Inspired by Claude Code's `autoDream` but scaled for a lightweight desktop app.

### Trigger Conditions

The Consolidator runs when **any** of these are true:

- 20 new journal entries since last consolidation
- 6 hours have elapsed since last consolidation
- User explicitly runs `ghostspell memory consolidate` (CLI)
- App is idle for > 10 minutes with pending journal entries

### Consolidation Process

```
1. Read journal entries since last consolidation
2. Compute statistics:
   a. Acceptance rate by prompt_type
   b. Undo rate (signals bad transforms)
   c. Language distribution
   d. Time-of-day patterns
3. For LOCAL tier (Ollama/llama.cpp):
   - Use rule-based heuristics (no LLM call needed):
     - Undo rate > 40% for a prompt_type → flag for review
     - Language switch detected → update language entries
     - Consistent rejection pattern → add to rejections topic
4. For CLOUD tier (OpenRouter):
   - Fire a single cheap model call (Haiku-class) with:
     - Recent journal summary (anonymized, no raw text)
     - Current index.json
     - Prompt: "Given these transformation patterns, update the
       preference index. Remove contradictions. Increase confidence
       for reinforced patterns. Decrease confidence for stale ones."
   - Parse structured response back into index.json
5. Write discipline (atomic):
   a. Write updated topic files first
   b. Write updated index.json via atomic rename
   c. Update consolidation checkpoint
```

### Confidence Decay

Every consolidation cycle applies decay to entries not reinforced:

```go
const (
    DecayPerCycle   = 0.02  // lose 2% confidence per cycle
    MinConfidence   = 0.30  // below this, entry is evicted
    ReinforcementBoost = 0.10 // observed again → +10%
)

func decayConfidence(entry *IndexEntry, reinforced bool) {
    if reinforced {
        entry.Confidence = min(1.0, entry.Confidence + ReinforcementBoost)
    } else {
        entry.Confidence -= DecayPerCycle
    }
    if entry.Confidence < MinConfidence {
        // mark for eviction
        entry.Evict = true
    }
}
```

---

## 7. Context Compaction Strategies

For GhostSpell Cloud users who maintain longer conversational sessions (e.g., via Telegram integration), context will grow. Implement three compaction strategies (inspired by Claude Code's five):

### Strategy 1 — Sliding Window

Keep the last N transform pairs (input/output) in full fidelity. Older pairs are dropped entirely. Default N = 5.

**Use when:** Context is approaching 70% of model's window. Simplest, always safe.

### Strategy 2 — Summary Compaction

Older transforms are replaced with a one-line summary:

```
[Previous: 3 casual EN rewrites, all accepted, avg 45 words]
```

Recent transforms kept verbatim. The summary is generated locally via template — no LLM call required.

**Use when:** Session has 10+ transforms and user is on a small-context local model (e.g., 4K context llama).

### Strategy 3 — Preference-Weighted Compaction

Keep transforms that **disagree** with current memory (rejections, undos, unusual patterns). Drop transforms that confirm existing preferences.

The logic: confirming transforms have already been absorbed into the memory index. Disagreeing transforms are more valuable for context because they represent edge cases or preference shifts.

**Use when:** Long Telegram sessions, Cloud tier, models with 8K+ context.

---

## 8. File Layout

```
~/.ghostspell/
├── config.toml              # existing GhostSpell config
├── models/                  # existing model registry
│   └── registry.json
└── memory/
    ├── index.json           # Layer 1 — always loaded
    ├── topics/              # Layer 2 — on-demand
    │   ├── tone.json
    │   ├── formatting.json
    │   ├── languages.json
    │   ├── domains.json
    │   └── rejections.json
    ├── journal/             # Layer 3 — append-only logs
    │   ├── 2026-04-01.jsonl
    │   └── 2026-04-02.jsonl
    └── consolidation.json   # checkpoint for the Consolidator
```

### Config Integration (`config.toml`)

```toml
[memory]
enabled = true
index_max_entries = 40
index_max_token_budget = 500
topic_max_observations = 50
topic_max_size_kb = 500
journal_retention_days = 30
consolidation_interval_hours = 6
consolidation_min_entries = 20

[memory.cloud_sync]
enabled = false  # opt-in for GhostSpell Cloud subscribers
encrypt = true   # AES-256-GCM before upload
```

---

## 9. Rejection & Undo Tracking

The most valuable signal for memory is **negative feedback**: when the user undoes a transformation (Ctrl+Z after paste-back) or re-invokes F7 on the same text.

### Detection Methods

1. **Undo detection:** After paste-back, monitor clipboard for 5 seconds. If the original text reappears on the clipboard, record `undo: true`.
2. **Re-invocation detection:** If F7 is pressed within 10 seconds and the input hash matches the previous output hash, the user is retrying — record `accepted: false` for the previous transform.
3. **Explicit feedback (future):** A small toast notification after transform with 👍/👎 buttons. Only if user enables it in settings.

### Rejection → Memory Pipeline

```
Rejection detected
  → journal entry with accepted=false / undo=true
    → Consolidator picks up pattern after N occurrences
      → Topic file updated (rejections.json)
        → Index entry created/updated with rejection pattern
          → Next transform avoids the rejected pattern
```

---

## 10. Privacy & Security Considerations

| Concern | Mitigation |
|---|---|
| Raw text in memory | **Never stored.** Journal uses truncated SHA-256 hashes only. |
| Memory sent to LLM providers | Only the derived context string (preferences, not content) is sent. |
| Cloud sync exposure | Encrypted (AES-256-GCM) before upload. Key derived from user passphrase. |
| Memory poisoning | Confidence scores + decay prevent a single bad session from dominating. |
| Sensitive content in topics | Topic observations describe *patterns*, not content. E.g., "rejected formal tone" not "rewrote email about salary negotiation." |
| AGPL-3.0 compliance | Memory system is part of the AGPL-licensed core. Cloud sync protocol is documented. |

---

## 11. Implementation Phases

### Phase 1 — Foundation (v0.14.0)

- [ ] `memory/` directory structure and config integration
- [ ] Layer 3: Transform Journal (append-only JSONL writer)
- [ ] Basic undo/rejection detection
- [ ] `ghostspell memory status` CLI command

### Phase 2 — Index & Assembly (v0.14.1)

- [ ] Layer 1: Index file read/write with atomic rename
- [ ] Memory Context Assembler (inject into transform prompts)
- [ ] Confidence decay on read
- [ ] `ghostspell memory show` / `ghostspell memory reset` CLI commands

### Phase 3 — Consolidator (v0.14.2)

- [ ] Rule-based consolidation for local tier
- [ ] LLM-based consolidation for Cloud tier (Haiku-class call)
- [ ] Background goroutine with trigger conditions
- [ ] Layer 2: Topic file management

### Phase 4 — Compaction & Cloud (v0.15.0)

- [ ] Context compaction strategies 1-3
- [ ] Cloud sync with E2E encryption
- [ ] Telegram session memory integration
- [ ] `ghostspell memory export` / `ghostspell memory import`

---

## 12. Open Questions

1. **Should the Consolidator run in-process or as a separate daemon?** In-process is simpler but dies with the app. A systemd/launchd service survives but adds deployment complexity.

2. **Token budget allocation:** 500 tokens for index + 300 for topics = 800 total. Is this too much for 4K-context local models? May need a `memory.budget = "minimal" | "standard" | "generous"` config option.

3. **Model-specific formatting:** Should the memory context string be formatted differently for different model families? (e.g., ChatML vs. Llama prompt format). The model registry already knows the model — could use this.

4. **Cross-device memory:** If a user runs GhostSpell on Mac (local) and also uses GhostSpell Cloud from their phone, should memories merge? Conflict resolution gets tricky.

5. **GDPR/privacy:** The `ghostspell memory export` command should produce a human-readable dump. `ghostspell memory nuke` should be a hard delete with no recovery.

---

## References

- Claude Code source leak analysis (March 31, 2026) — tiered memory architecture, autoDream, skeptical memory pattern
- GhostSpell architecture docs (internal) — model registry, OpenRouter integration, hotkey pipeline
- [Anthropic: How Claude remembers your project](https://code.claude.com/docs/en/memory) — CLAUDE.md + auto memory design
