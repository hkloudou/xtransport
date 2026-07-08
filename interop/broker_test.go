package interop

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	tcptransport "github.com/hkloudou/xtransport/transports/tcp"
	wstransport "github.com/hkloudou/xtransport/transports/ws"
)

// testBroker runs one Broker on both a TCP and a WebSocket listener and
// returns paho broker URLs for each.
type testBroker struct {
	broker  *Broker
	tcpURL  string
	wsURL   string
	closers []func()
}

func startBroker(t *testing.T) *testBroker {
	t.Helper()
	b := NewBroker()
	b.Logf = t.Logf

	tcpTran := tcptransport.NewTransport("tcp")
	tcpL, err := tcpTran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("tcp listen: %v", err)
	}
	go b.Serve(tcpL)

	wsTran := wstransport.NewTransport("/mqtt", wstransport.Subprotocols("mqtt"))
	wsL, err := wsTran.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("ws listen: %v", err)
	}
	go b.Serve(wsL)

	tb := &testBroker{
		broker: b,
		tcpURL: "tcp://" + tcpL.Addr(),
		wsURL:  "ws://" + wsL.Addr() + "/mqtt",
		closers: []func(){
			func() { tcpL.Close() },
			func() { wsL.Close() },
			b.Close,
		},
	}
	t.Cleanup(func() {
		for _, c := range tb.closers {
			c()
		}
	})
	return tb
}

func connect(t *testing.T, url, id string, keepalive time.Duration) paho.Client {
	t.Helper()
	opts := paho.NewClientOptions().
		AddBroker(url).
		SetClientID(id).
		SetKeepAlive(keepalive).
		SetConnectTimeout(5 * time.Second).
		SetAutoReconnect(false)
	c := paho.NewClient(opts)
	tok := c.Connect()
	if !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
		t.Fatalf("connect %s: %v", url, tok.Error())
	}
	t.Cleanup(func() { c.Disconnect(100) })
	return c
}

func urls(tb *testBroker) map[string]string {
	return map[string]string{"tcp": tb.tcpURL, "ws": tb.wsURL}
}

// TestPubSubQoS exercises publish/subscribe roundtrips at every QoS level
// over both the TCP and WebSocket transports with a real paho client.
func TestPubSubQoS(t *testing.T) {
	tb := startBroker(t)
	for name, url := range urls(tb) {
		for qos := 0; qos <= 2; qos++ {
			t.Run(fmt.Sprintf("%s/qos%d", name, qos), func(t *testing.T) {
				sub := connect(t, url, fmt.Sprintf("sub-%s-%d", name, qos), 30*time.Second)
				pub := connect(t, url, fmt.Sprintf("pub-%s-%d", name, qos), 30*time.Second)

				got := make(chan paho.Message, 1)
				tok := sub.Subscribe(fmt.Sprintf("t/%s/%d", name, qos), byte(qos), func(_ paho.Client, m paho.Message) {
					got <- m
				})
				if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
					t.Fatalf("subscribe: %v", tok.Error())
				}

				payload := []byte("hello-" + name)
				tok = pub.Publish(fmt.Sprintf("t/%s/%d", name, qos), byte(qos), false, payload)
				if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
					t.Fatalf("publish: %v", tok.Error())
				}

				select {
				case m := <-got:
					if !bytes.Equal(m.Payload(), payload) {
						t.Fatalf("payload = %q, want %q", m.Payload(), payload)
					}
					if int(m.Qos()) != qos {
						t.Fatalf("qos = %d, want %d", m.Qos(), qos)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("message never delivered")
				}
			})
		}
	}
}

// TestKeepalive verifies the ping/pong path: a client with a short keep
// alive stays connected through idle periods longer than the interval.
func TestKeepalive(t *testing.T) {
	tb := startBroker(t)
	c := connect(t, tb.tcpURL, "keepalive", 1*time.Second)

	time.Sleep(3500 * time.Millisecond)
	if !c.IsConnectionOpen() {
		t.Fatal("connection dropped during idle keepalive period")
	}
	tok := c.Publish("still/alive", 1, false, []byte("ping"))
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("publish after idle: %v", tok.Error())
	}
}

// TestLargePayload pushes a payload well past the transports' internal
// buffer sizes through the full stack.
func TestLargePayload(t *testing.T) {
	tb := startBroker(t)
	for name, url := range urls(tb) {
		t.Run(name, func(t *testing.T) {
			sub := connect(t, url, "big-sub-"+name, 30*time.Second)
			pub := connect(t, url, "big-pub-"+name, 30*time.Second)

			payload := make([]byte, 256*1024)
			if _, err := rand.Read(payload); err != nil {
				t.Fatal(err)
			}

			got := make(chan []byte, 1)
			tok := sub.Subscribe("big/"+name, 1, func(_ paho.Client, m paho.Message) {
				got <- m.Payload()
			})
			if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
				t.Fatalf("subscribe: %v", tok.Error())
			}
			tok = pub.Publish("big/"+name, 1, false, payload)
			if !tok.WaitTimeout(10*time.Second) || tok.Error() != nil {
				t.Fatalf("publish: %v", tok.Error())
			}
			select {
			case p := <-got:
				if !bytes.Equal(p, payload) {
					t.Fatalf("payload corrupted: got %d bytes, want %d", len(p), len(payload))
				}
			case <-time.After(15 * time.Second):
				t.Fatal("large message never delivered")
			}
		})
	}
}

// TestConcurrentPublishers hammers one subscriber from several concurrent
// publishers; every message must arrive intact (this is what exercises
// concurrent Send frame-atomicity on the server socket).
func TestConcurrentPublishers(t *testing.T) {
	tb := startBroker(t)
	const publishers = 8
	const perPublisher = 50

	sub := connect(t, tb.tcpURL, "conc-sub", 30*time.Second)
	var mu sync.Mutex
	seen := map[string]bool{}
	done := make(chan struct{})
	tok := sub.Subscribe("conc/#", 1, func(_ paho.Client, m paho.Message) {
		mu.Lock()
		seen[string(m.Payload())] = true
		n := len(seen)
		mu.Unlock()
		if n == publishers*perPublisher {
			close(done)
		}
	})
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("subscribe: %v", tok.Error())
	}

	var wg sync.WaitGroup
	for p := 0; p < publishers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			c := connect(t, tb.tcpURL, fmt.Sprintf("conc-pub-%d", p), 30*time.Second)
			for i := 0; i < perPublisher; i++ {
				tok := c.Publish(fmt.Sprintf("conc/%d", p), 1, false, []byte(fmt.Sprintf("msg-%d-%d", p, i)))
				if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
					t.Errorf("publish %d/%d: %v", p, i, tok.Error())
					return
				}
			}
		}(p)
	}
	wg.Wait()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		mu.Lock()
		n := len(seen)
		mu.Unlock()
		t.Fatalf("only %d/%d messages delivered", n, publishers*perPublisher)
	}
}

// TestUnsubscribe verifies UNSUBSCRIBE stops delivery.
func TestUnsubscribe(t *testing.T) {
	tb := startBroker(t)
	sub := connect(t, tb.tcpURL, "unsub-sub", 30*time.Second)
	pub := connect(t, tb.tcpURL, "unsub-pub", 30*time.Second)

	got := make(chan struct{}, 8)
	tok := sub.Subscribe("un/x", 1, func(_ paho.Client, m paho.Message) { got <- struct{}{} })
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("subscribe: %v", tok.Error())
	}
	tok = pub.Publish("un/x", 1, false, []byte("1"))
	tok.WaitTimeout(5 * time.Second)
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("first message not delivered")
	}

	tok = sub.Unsubscribe("un/x")
	if !tok.WaitTimeout(5*time.Second) || tok.Error() != nil {
		t.Fatalf("unsubscribe: %v", tok.Error())
	}
	tok = pub.Publish("un/x", 1, false, []byte("2"))
	tok.WaitTimeout(5 * time.Second)
	select {
	case <-got:
		t.Fatal("message delivered after unsubscribe")
	case <-time.After(1 * time.Second):
	}
}

func TestMatchFilter(t *testing.T) {
	cases := []struct {
		filter, topic string
		want          bool
	}{
		{"a/b/c", "a/b/c", true},
		{"a/b/c", "a/b/d", false},
		{"a/+/c", "a/b/c", true},
		{"a/+/c", "a/b/d/c", false},
		{"a/#", "a", true},
		{"a/#", "a/b/c", true},
		{"#", "a/b", true},
		{"+", "a", true},
		{"+", "a/b", false},
		{"#", "$SYS/x", false},
		{"+/x", "$SYS/x", false},
		{"a/b/+", "a/b", false},
		{"/", "/", true},
		{"a//b", "a//b", true},
	}
	for _, c := range cases {
		if got := MatchFilter(c.filter, c.topic); got != c.want {
			t.Errorf("MatchFilter(%q, %q) = %v, want %v", c.filter, c.topic, got, c.want)
		}
	}
}
