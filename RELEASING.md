# Releasing

This repository is a multi-module workspace. The transports depend on the
root module **by version tag**, not by path, so releases must happen in
order or consumers will mix old and new code:

1. Merge to `main`.
2. Tag the root module and the codec on the merge commit:

   ```sh
   git tag v1.1.8
   git tag packets/mqtt/v1.1.4
   git push origin v1.1.8 packets/mqtt/v1.1.4
   ```

3. Bump every `transports/*/go.mod` to require the root tag from step 2,
   merge that change, then tag the transports on it:

   ```sh
   git tag transports/tcp/v1.0.4 transports/ws/v1.0.6 transports/quic/v1.0.3
   git push origin transports/tcp/v1.0.4 transports/ws/v1.0.6 transports/quic/v1.0.3
   ```

Skipping step 3 means `go get github.com/hkloudou/xtransport/transports/...`
keeps resolving the previous root version — new transport code silently
runs against the old core. The `downstream` CI job builds every module
with `GOWORK=off` to surface exactly what a consumer would resolve.

`make` runs the build; version tagging via `git autotag` is only invoked
through the explicit `make default` / `make git` / `make retag` targets
from `include.mk`.
