# Gamedays

Records of deliberate failure drills. One file per drill:
`YYYY-MM-DD-<scenario>.md`.

Record the numbers even when — especially when — they are bad. The delta
between drills is the useful part.

## Template

```markdown
# <scenario>

**Date**       — YYYY-MM-DD
**Hypothesis** — what I expected to happen
**Target**     — RTO target for this scenario

## Timeline
| T+ | Event |
|----|-------|
| 00:00 | fault injected |
| ...   | ... |

## Measured
- Detection time:
- Recovery time:
- Requests lost:

## What broke that I did not expect

## Actions
- [ ] ...
```
