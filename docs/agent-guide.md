# Agent guide

How an AI coding agent runs the sealed-iteration loop. Every command below
uses `-o json`: read outcomes as data, and branch on exit statuses (`0`
success, `1` refused, `2` usage, `3` unreachable) rather than on text.

The loop assumes a registered sandbox host. See
[Cluster setup](cluster-setup.md).

## 1. Declare

Declare the application, the hosts it may reach, and its test suite.

```sh
cloudbox -o json app create web --allow api.github.com --suite smoke --tests 12
```

## 2. Create

```sh
cloudbox -o json sandbox create -a web
```

Read the sandbox name from `.name`. The sandbox is sealed before anything is
admitted. If `.sealed` is false, stop: the host failed its enforcement probe.

## 3. Apply

```sh
cloudbox -o json apply -a web -s web-sbx-7f3a-1 -f ./k8s/
```

A refusal (exit `1`) carries `findings`. Each one names the manifest and the
fix. Fix the manifests rather than retrying.

## 4. Test

```sh
cloudbox -o json test -s web-sbx-7f3a-1
```

## 5. Explain

When a test fails because something could not be reached, ask what was
blocked:

```sh
cloudbox -o json status -s web-sbx-7f3a-1 --explain
```

```json
{
  "sandbox": "web-sbx-7f3a-1",
  "state": "ready",
  "sealed": true,
  "blockedEgress": [
    {"destination": "api.stripe.com", "at": "2026-11-20T09:14:03Z", "workload": "web"}
  ],
  "proposal": {
    "app": "web",
    "addFqdns": ["api.stripe.com"],
    "submitTo": "admin review: audited application-policy change (N5)"
  }
}
```

Only attempts that reached the egress proxy are listed. A direct connection
is blocked without its destination being recorded. If a test fails and
`blockedEgress` is empty, check whether the client honours `http_proxy`.

## 6. Amend

Decide whether the dependency is legitimate. If it is, amend the declaration.
Never work around the seal.

```sh
cloudbox -o json app allow web api.stripe.com
```

The running sandbox picks up the amended allowlist once Kubernetes syncs it
into the egress proxy, usually within one to two minutes. Re-run the suite after that, and
expect attempts made before it to stay in `blockedEgress`.

## 7. Read evidence

```sh
cloudbox -o json evidence -s web-sbx-7f3a-1
```

Report what `.summary` says, not more. Evidence never says "verified
working". `.substrateParity` is `unverified` when no production substrate is
registered. `.source` is `user-controlled` on a cluster you registered.

## 8. Destroy

```sh
cloudbox -o json sandbox destroy -s web-sbx-7f3a-1
```

Evidence lives in the local control plane until it stops. Keep the output of
step 7 if you need it later.
