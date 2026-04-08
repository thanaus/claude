# Technical Debt

## NATS connection lifecycle shared across probe and provision

`internal/nats/client.go` and `internal/nats/provisioner.go` both establish and close
their own NATS connection through the shared `connect()` helper.

This is not a bug in the current `sync` flow because `sync` only uses
`natsclient.Provisioner` and does not call `natsclient.Client.Probe()`.
However, the design leaves room for a future `probe -> provision` workflow to open
two sequential NATS connections for the same command execution.

### Current state

- `Client.Probe()` opens a connection, validates NATS and JetStream, then closes it.
- `Provisioner.Provision()` opens another connection, prepares streams and KV, then closes it.
- `cmd/sync.go` currently wires only `natsclient.Provisioner`.

### Risk

- A future command that composes probe and provision may duplicate connection setup,
  authentication, and JetStream initialization work.
- The current separation makes connection reuse harder because the provisioning API
  owns connection creation internally.

### Direction

- Prefer introducing a shared internal NATS session or connection owner that can be
  reused across probe and provision steps in the same workflow.
- Avoid pushing raw `*nats.Conn` too high into service-level interfaces unless that
  trade-off becomes necessary.
