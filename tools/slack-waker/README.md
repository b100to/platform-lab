# slack-waker

Turns `/wake 3h deploying a hotfix` in a Slack channel into a
[`WakeRequest`](../../operators/idle-reaper) in the namespace that channel is
mapped to.

```
/wake 5m checking a fix

✅ lab-dev is awake for 5m (until 2026-08-25T05:57:38Z).
   Request wake-g5jjk expires on its own — nothing to undo.
   Reason: checking a fix
```

## What it deliberately does not do

The bot decides nothing. It does not know who may wake what, how long a
namespace may stay up, or when to put it back — the API server and the
idle-reaper controller own all three. What it does is translate a sentence
into an object in the right namespace and repeat back what the cluster said.

That is not tidiness. It is the security argument:

```yaml
kind: Role                    # not ClusterRole
namespace: lab-dev
rules:
  - apiGroups: ["finops.b100to.dev"]
    resources: ["wakerequests"]
    verbs: ["create", "get", "list"]
```

Note what is absent — no access to `idlewindows`, which is the policy; no
Deployments; no Secrets; no other namespace. **The worst a compromised bot can
do is ask a namespace to stay awake, for no longer than the IdleWindow allows,
and it cannot read or change that limit.** A bot that scaled Deployments
directly would need write access to workloads, and everyone who could talk to
it would inherit that.

## Channels are mapped, not inferred

```yaml
env:
  - name: CHANNEL_NAMESPACES
    value: "team-a-dev=lab-dev,team-b-dev=lab-b"
```

Deriving the namespace from the channel name would hand out access the moment
somebody created a channel with a convenient name. A command in an unmapped
channel is refused.

## Reporting the verdict, not the request

Creating the object is not the same as it being accepted. The API server
checks the shape; whether the duration is allowed is the controller's call,
made a moment later. An earlier version replied "awake for 24h" to a request
that was about to be refused for exceeding the 8h cap — the policy held, but
the human was told the opposite.

So the bot waits for the verdict and repeats that:

```
/wake 24h just for a while

⛔ request refused — duration 24h exceeds the 8h limit set by dev-nights
   Request wake-6zlbw is kept so the refusal is on the record.
```

Rejected requests are kept. The refusal is itself a record of who asked for
what.

## Socket Mode

Slash commands normally need a public URL. Socket Mode has the bot open an
outbound connection instead, so a cluster on a laptop needs no tunnel and no
inbound firewall rule.

One consequence shapes the deployment: Slack delivers each command to one
connected instance of its choice. During a rolling update both the old and new
pod are connected, and a command routed to the one shutting down is lost —
the user sees "the app didn't respond" and neither log explains it. The
Deployment therefore uses `strategy: Recreate`.

## Install

```sh
kubectl create secret generic wake-bot-slack -n idle-reaper-system \
  --from-literal=appToken=xapp-... \
  --from-literal=botToken=xoxb-...

kubectl apply -k deploy
```

The Slack app itself is defined by [`manifest.json`](manifest.json) rather than
configured by clicking, so the scopes and the slash command are reviewable and
reproducible:

```sh
slack login
slack manifest validate
slack app install
```

The app-level token for Socket Mode is a credential rather than configuration,
so it is still issued from the Slack UI.

## Identity

`spec.requestedBy` is filled from the Slack profile, and it is a label for
people reading the object later — not an authorization input. Anything the bot
writes is only as trustworthy as the bot; the record that counts is the API
server's audit log.

Making that record name a person rather than the bot means impersonation:
calling the API as the user via `Impersonate-User` so RBAC applies to them and
the audit entry carries their name. That needs a Slack-identity-to-cluster-user
mapping, and impersonation rights scoped with `resourceNames` — an unrestricted
`impersonate` on users is equivalent to cluster-admin, since group
impersonation reaches `system:masters`. Not done here; the mapping is the
missing piece.
