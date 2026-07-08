# ws transport

WebSocket transport for [xtransport](https://github.com/hkloudou/xtransport),
built on gobwas/ws.

## Installation
``` sh
go get -u github.com/hkloudou/xtransport/transports/ws
```

## Notes
- `Subprotocols("mqtt")` enables RFC 6455 subprotocol negotiation, which
  standard MQTT-over-WebSocket clients (browsers, mqtt.js) require.
- `SetTimeOut` is an idle interval: only *data* frames count as
  activity; WebSocket pings alone do not keep a connection alive.
