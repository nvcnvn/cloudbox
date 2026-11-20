# Containment

What a sealed sandbox stops, what it does not, and what it records. This page
carries the control plane's own published statement
(`GET /v1/containment-statement`) item by item, and the acceptance suite
checks the two against each other. The page never claims more than the
product does.

## Cooperative guarantee

For a workload that is not trying to escape: closed-world reproducibility:
any egress not on the allowlist is denied (which denials are recorded by
destination is stated under recordingLimits); a workload that ran correctly
sealed exercised zero undeclared external dependencies on the paths it ran.

In practice, if your code ran sealed and passed, it does not depend on a
host you did not declare, on the paths it exercised.

## Adversarial limits

For a workload that *is* trying to escape, the seal is strong but bounded.

### Blocked

- all direct egress, including IP-literal connections (default-deny NetworkPolicy admits only cluster DNS and the egress proxy)
- any FQDN not on the allowlist
- all ingress
- all writes to production (the sandbox holds no production credentials at any adoption level)

### Residual channels

These paths stay open, and CloudBox names them rather than hiding them:

- low-bandwidth exfiltration through DNS queries to cluster DNS (tunneling)
- exfiltration through allowlisted endpoints (the seal proves where traffic went, not what it carried)

## Recording limits

Blocking and recording are different guarantees, and recording is the weaker
one:

- egress from proxy-aware HTTP clients reaches the product's egress proxy, and a denied attempt there is recorded by destination (FQDN), time, and workload
- all other egress — direct connections, IP literals, non-HTTP protocols, and clients that ignore the injected proxy settings — is denied by NetworkPolicy at the CNI without its destination being recorded
- the blocked-attempt record is therefore a floor, not a complete account of the egress the seal denied

A short blocked-attempt list does not mean the workload attempted little. It
may have attempted a great deal over protocols the record cannot name.

## When the egress proxy is not running

Default-deny holds without the egress proxy, but the allowlist does not.
While a sandbox's proxy is unavailable, allowlisted egress is denied, and the
sandbox reports `allowlistEnforced: false` with the proxy's state and the
image it could not provision.

## On the roadmap

Not in this release: DNS query logging with per-sandbox rate limits and
anomaly detection, and per-FQDN egress volume metering in evidence.
