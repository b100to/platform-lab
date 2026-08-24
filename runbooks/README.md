# Runbooks

One file per failure mode. Written *before* the drill as a hypothesis, then
corrected against what actually happened.

A runbook is finished when someone else could follow it under pressure without
asking a question.

## Template

```markdown
# <failure mode>

**Detection**  — what signal shows this is happening
**Blast radius** — what is affected, what is not
**Target**     — RTO / RPO for this scenario

## Steps
1. ...

## Verification
- ...

## Known pitfalls
- ...
```
