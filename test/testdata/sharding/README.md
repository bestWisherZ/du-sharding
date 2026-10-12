# Sharding Helper Contracts

- `poke_block`: explicit transactions used to advance preparation heights.
- `sharding_heartbeat`: proposer-injected heartbeat transactions for scheduling.

Each directory contains source, an independent Go module, a Linux amd64 binary
and its 7z deployment package. Both modules use contract-sdk-go v2.3.9.

To rebuild, run these commands inside the corresponding contract directory:

```bash
name=$(go list -m)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o "$name" .
7z a "$name.7z" "$name"
```

These contracts are system helpers, not the benchmark workload.
