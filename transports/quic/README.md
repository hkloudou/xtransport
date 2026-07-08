# quic transport

QUIC transport for [xtransport](https://github.com/hkloudou/xtransport).

## Installation
``` sh
go get -u github.com/hkloudou/xtransport/transports/quic
```

## Notes
- A `tls.Config` is required on the listener side; when the config
  carries no ALPN protocols, `xtransport` is used.
- QUIC opens streams lazily: the server's Accept handler is not invoked
  until the client's first Send, so the protocol spoken over the socket
  must be client-speaks-first (as MQTT is).
