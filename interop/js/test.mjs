// Real mqtt.js client exercising the xtransport interop broker over both
// raw TCP and MQTT-over-WebSocket (the path browsers use). Exits 0 on
// success, 1 on any failure or timeout.
import mqtt from 'mqtt';

const TCP_URL = process.env.BROKER_TCP_URL ?? 'mqtt://127.0.0.1:1883';
const WS_URL = process.env.BROKER_WS_URL ?? 'ws://127.0.0.1:8083/mqtt';

const deadline = setTimeout(() => {
  console.error('FAIL: global timeout');
  process.exit(1);
}, 60_000);

function connect(url, id) {
  return new Promise((resolve, reject) => {
    const c = mqtt.connect(url, {
      clientId: id,
      protocolVersion: 4, // MQTT 3.1.1
      keepalive: 2,
      connectTimeout: 10_000,
      reconnectPeriod: 0,
    });
    c.once('connect', () => resolve(c));
    c.once('error', reject);
  });
}

function once(emitter, event) {
  return new Promise((resolve) => emitter.once(event, (...args) => resolve(args)));
}

async function roundtrip(name, url, qos) {
  const sub = await connect(url, `js-sub-${name}-${qos}`);
  const pub = await connect(url, `js-pub-${name}-${qos}`);
  try {
    const topic = `js/${name}/${qos}`;
    const payload = Buffer.from(`payload-${name}-${qos}-é中文`); // exercise UTF-8
    await sub.subscribeAsync(topic, { qos });

    const gotMessage = new Promise((resolve) => {
      sub.on('message', (t, p, packet) => resolve({ t, p, packet }));
    });
    await pub.publishAsync(topic, payload, { qos });

    const { t, p, packet } = await gotMessage;
    if (t !== topic) throw new Error(`topic ${t} != ${topic}`);
    if (!p.equals(payload)) throw new Error(`payload mismatch on ${name} qos${qos}`);
    if (packet.qos !== qos) throw new Error(`qos ${packet.qos} != ${qos}`);
    console.log(`ok: ${name} qos${qos} roundtrip`);
  } finally {
    await sub.endAsync();
    await pub.endAsync();
  }
}

async function keepalive(url) {
  // keepalive=2s; stay idle >2 intervals; mqtt.js kills the connection
  // itself if PINGRESP is missing, so surviving the idle window proves
  // the broker answers PINGREQ.
  const c = await connect(url, 'js-keepalive');
  try {
    const closed = once(c, 'close').then(() => {
      throw new Error('connection closed during keepalive idle window');
    });
    await Promise.race([new Promise((r) => setTimeout(r, 5_000)), closed]);
    await c.publishAsync('js/alive', Buffer.from('ok'), { qos: 1 });
    console.log('ok: keepalive survived idle window');
  } finally {
    await c.endAsync();
  }
}

try {
  for (const [name, url] of [['tcp', TCP_URL], ['ws', WS_URL]]) {
    for (const qos of [0, 1, 2]) {
      await roundtrip(name, url, qos);
    }
  }
  await keepalive(WS_URL);
  console.log('PASS: all mqtt.js interop checks');
  clearTimeout(deadline);
  process.exit(0);
} catch (err) {
  console.error('FAIL:', err);
  process.exit(1);
}
