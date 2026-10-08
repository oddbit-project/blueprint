package nats

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ackServer is a fake JetStream server: it answers PINGs and acks each publish once ack
// is closed; with a nil ack it never acks. published receives one value per publish read
func ackServer(t *testing.T, ack <-chan struct{}) (url string, published <-chan struct{}) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	pub := make(chan struct{}, 16)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		var wmu sync.Mutex
		write := func(s string) {
			wmu.Lock()
			defer wmu.Unlock()
			_, _ = c.Write([]byte(s))
		}
		write(fakeServerINFO)
		r := bufio.NewReader(c)
		sids := map[string]string{}
		seq := 0
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			f := strings.Fields(line)
			switch {
			case len(f) >= 3 && f[0] == "SUB":
				sids[strings.TrimSuffix(f[1], "*")] = f[len(f)-1]
			case len(f) == 4 && f[0] == "PUB":
				if _, err := r.ReadString('\n'); err != nil {
					return
				}
				pub <- struct{}{}
				if ack == nil {
					continue
				}
				reply := f[2]
				sid := sids[reply[:strings.LastIndex(reply, ".")+1]]
				seq++
				payload := fmt.Sprintf(`{"stream":"S","seq":%d}`, seq)
				go func() {
					<-ack
					write(fmt.Sprintf("MSG %s %s %d\r\n%s\r\n", reply, sid, len(payload), payload))
				}()
			case len(f) >= 1 && f[0] == "PING":
				write("PONG\r\n")
			}
		}
	}()
	return "nats://" + ln.Addr().String(), pub
}

// fakeJSProducer builds a JSProducer on a plain connection with the given drain timeout
func fakeJSProducer(t *testing.T, url string, drain time.Duration) *JSProducer {
	conn, err := nats.Connect(url, nats.DrainTimeout(drain))
	require.NoError(t, err)
	t.Cleanup(conn.Close)
	js, err := jetstream.New(conn)
	require.NoError(t, err)
	return &JSProducer{Subject: "issue114", Conn: conn, JS: js}
}

// publishAsync publishes one message and waits for the server to read it
func publishAsync(t *testing.T, p *JSProducer, published <-chan struct{}) jetstream.PubAckFuture {
	paf, err := p.PublishAsync(context.Background(), "issue114", []byte("m"))
	require.NoError(t, err)
	select {
	case <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("server never read the publish")
	}
	return paf
}

// disconnect runs Disconnect and returns a channel closed when it returns
func disconnect(p *JSProducer) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		p.Disconnect()
		close(done)
	}()
	return done
}

func TestIssue114_DisconnectResolvesUnackedFuture(t *testing.T) {
	const drain = 300 * time.Millisecond
	url, published := ackServer(t, nil)
	p := fakeJSProducer(t, url, drain)
	paf := publishAsync(t, p, published)

	select {
	case <-disconnect(p):
	case <-time.After(drain + 2*time.Second):
		t.Fatal("Disconnect did not return within the drain timeout")
	}

	select {
	case err := <-paf.Err():
		assert.ErrorIs(t, err, jetstream.ErrJetStreamPublisherClosed)
	case a := <-paf.Ok():
		t.Fatalf("future resolved with an ack the server never sent: %+v", a)
	case <-time.After(2 * time.Second):
		t.Fatal("future unresolved 2s after Disconnect")
	}
}

func TestIssue114_DisconnectResolvesUnackedFutureOnClosedConn(t *testing.T) {
	url, published := ackServer(t, nil)
	p := fakeJSProducer(t, url, 300*time.Millisecond)
	paf := publishAsync(t, p, published)
	p.Conn.Close()

	select {
	case <-disconnect(p):
	case <-time.After(5 * time.Second):
		t.Fatal("Disconnect did not return")
	}

	select {
	case err := <-paf.Err():
		assert.ErrorIs(t, err, jetstream.ErrJetStreamPublisherClosed)
	case a := <-paf.Ok():
		t.Fatalf("future resolved with an ack the server never sent: %+v", a)
	case <-time.After(2 * time.Second):
		t.Fatal("future unresolved 2s after Disconnect")
	}
}

// hookJS runs a hook after the JetStream calls Disconnect makes, to place an event at
// an exact point of Disconnect
type hookJS struct {
	jetstream.JetStream
	afterComplete func()
	afterCleanup  func()
}

func (h *hookJS) PublishAsyncComplete() <-chan struct{} {
	ch := h.JetStream.PublishAsyncComplete()
	if h.afterComplete != nil {
		h.afterComplete()
	}
	return ch
}

func (h *hookJS) CleanupPublisher() {
	h.JetStream.CleanupPublisher()
	if h.afterCleanup != nil {
		h.afterCleanup()
	}
}

func TestIssue114_DisconnectResolvesFutureOfPublishRacingWait(t *testing.T) {
	url, _ := ackServer(t, nil)
	p := fakeJSProducer(t, url, 300*time.Millisecond)
	var (
		paf jetstream.PubAckFuture
		err error
	)
	h := &hookJS{JetStream: p.JS}
	// a PublishAsync that passed the closed check before Disconnect, landing after the wait
	h.afterComplete = func() { paf, err = h.PublishAsync("issue114", []byte("m")) }
	p.JS = h

	select {
	case <-disconnect(p):
	case <-time.After(5 * time.Second):
		t.Fatal("Disconnect did not return")
	}
	require.NotNil(t, paf, "the racing publish never ran; err: %v", err)
	select {
	case err := <-paf.Err():
		assert.ErrorIs(t, err, jetstream.ErrJetStreamPublisherClosed)
	case a := <-paf.Ok():
		t.Fatalf("future resolved with an ack the server never sent: %+v", a)
	case <-time.After(2 * time.Second):
		t.Fatal("future of a publish racing Disconnect unresolved 2s after Disconnect")
	}
}

func TestIssue114_PublishAfterDisconnectCleanupIsRejected(t *testing.T) {
	url, _ := ackServer(t, nil)
	p := fakeJSProducer(t, url, 300*time.Millisecond)
	ran := false
	var err error
	h := &hookJS{JetStream: p.JS}
	// a racing PublishAsync landing after the cleanup must not register a future that
	// nothing will resolve
	h.afterCleanup = func() {
		ran = true
		_, err = h.PublishAsync("issue114", []byte("m"))
	}
	p.JS = h

	select {
	case <-disconnect(p):
	case <-time.After(5 * time.Second):
		t.Fatal("Disconnect did not return")
	}
	require.True(t, ran, "Disconnect never cleaned up the publisher")
	assert.Error(t, err, "a publish after Disconnect's cleanup was accepted, so its future is never resolved")
}

func TestIssue114_DisconnectStopsWaitingWhenConnCloses(t *testing.T) {
	url, published := ackServer(t, nil)
	p := fakeJSProducer(t, url, 10*time.Second)
	paf := publishAsync(t, p, published)
	h := &hookJS{JetStream: p.JS}
	h.afterComplete = p.Conn.Close
	p.JS = h

	select {
	case <-disconnect(p):
	case <-time.After(2 * time.Second):
		t.Fatal("Disconnect kept waiting for acks after the connection closed")
	}
	select {
	case err := <-paf.Err():
		assert.ErrorIs(t, err, jetstream.ErrJetStreamPublisherClosed)
	case a := <-paf.Ok():
		t.Fatalf("future resolved with an ack the server never sent: %+v", a)
	case <-time.After(2 * time.Second):
		t.Fatal("future unresolved 2s after Disconnect")
	}
}

func TestIssue114_DisconnectWaitBoundedByDrainTimeout(t *testing.T) {
	for _, drain := range []time.Duration{300 * time.Millisecond, 1500 * time.Millisecond} {
		t.Run(drain.String(), func(t *testing.T) {
			url, published := ackServer(t, nil)
			p := fakeJSProducer(t, url, drain)
			publishAsync(t, p, published)

			start := time.Now()
			select {
			case <-disconnect(p):
			case <-time.After(drain + 5*time.Second):
				t.Fatal("Disconnect did not return")
			}
			elapsed := time.Since(start)
			assert.GreaterOrEqual(t, elapsed, drain, "Disconnect stopped waiting before the drain timeout")
			assert.Less(t, elapsed, drain+time.Second, "Disconnect waited well past the drain timeout")
		})
	}
}

func TestIssue114_DisconnectOnClosedConnDoesNotWait(t *testing.T) {
	url, published := ackServer(t, nil)
	p := fakeJSProducer(t, url, 10*time.Second)
	publishAsync(t, p, published)
	p.Conn.Close()

	select {
	case <-disconnect(p):
	case <-time.After(2 * time.Second):
		t.Fatal("Disconnect waited for acks on a connection that was already closed")
	}
}

func TestIssue114_DisconnectWaitsForPendingAck(t *testing.T) {
	ack := make(chan struct{})
	url, published := ackServer(t, ack)
	p := fakeJSProducer(t, url, 10*time.Second)
	paf := publishAsync(t, p, published)

	done := disconnect(p)
	select {
	case <-done:
		t.Fatal("Disconnect returned before the pending ack arrived")
	case <-time.After(200 * time.Millisecond):
	}
	close(ack)

	select {
	case a := <-paf.Ok():
		assert.Equal(t, uint64(1), a.Sequence)
	case err := <-paf.Err():
		t.Fatalf("future failed although the ack arrived within the drain timeout: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("future unresolved 5s after the ack was sent")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Disconnect did not return once the ack arrived")
	}
}

// waitingHook makes p signal on the returned channel when Disconnect starts waiting
// for acks
func waitingHook(p *JSProducer) <-chan struct{} {
	waiting := make(chan struct{})
	h := &hookJS{JetStream: p.JS}
	h.afterComplete = func() { close(waiting) }
	p.JS = h
	return waiting
}

// awaitWaiting fails the test unless the first Disconnect starts waiting for acks
func awaitWaiting(t *testing.T, waiting, first <-chan struct{}) {
	t.Helper()
	select {
	case <-waiting:
	case <-first:
		t.Fatal("the first Disconnect returned without waiting for the pending ack")
	case <-time.After(5 * time.Second):
		t.Fatal("the first Disconnect never started waiting for the pending ack")
	}
}

func TestIssue114_SecondDisconnectWaitsForFirst(t *testing.T) {
	const drain = time.Second
	url, published := ackServer(t, nil)
	p := fakeJSProducer(t, url, drain)
	paf := publishAsync(t, p, published)
	waiting := waitingHook(p)

	first := disconnect(p)
	awaitWaiting(t, waiting, first)
	// the first call keeps the connection open until its drain timeout, so a second call
	// that returns before the first has closed it finds it open
	select {
	case <-disconnect(p):
	case <-time.After(drain + 5*time.Second):
		t.Fatal("the second Disconnect did not return")
	}
	assert.True(t, p.Conn.IsClosed(), "the second Disconnect returned before the first closed the connection")
	select {
	case err := <-paf.Err():
		assert.ErrorIs(t, err, jetstream.ErrJetStreamPublisherClosed)
	case <-time.After(2 * time.Second):
		t.Fatal("future unresolved 2s after Disconnect")
	}
	select {
	case <-first:
	case <-time.After(5 * time.Second):
		t.Fatal("the first Disconnect did not return")
	}
}

func TestIssue114_SecondDisconnectDoesNotCutFirstWaitShort(t *testing.T) {
	ack := make(chan struct{})
	url, published := ackServer(t, ack)
	p := fakeJSProducer(t, url, 10*time.Second)
	paf := publishAsync(t, p, published)
	waiting := waitingHook(p)

	first := disconnect(p)
	awaitWaiting(t, waiting, first)
	second := disconnect(p)
	assert.Never(t, p.Conn.IsClosed, 300*time.Millisecond, 5*time.Millisecond,
		"the second Disconnect closed the connection while the first waited for the ack")
	close(ack)

	select {
	case <-paf.Ok():
	case err := <-paf.Err():
		t.Fatalf("future failed although its ack arrived within the drain timeout: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("future unresolved 5s after the ack was sent")
	}
	for _, done := range []<-chan struct{}{first, second} {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Disconnect did not return once the ack arrived")
		}
	}
}

func TestIssue114_DisconnectFromPublishErrHandlerReturns(t *testing.T) {
	url, published := ackServer(t, nil)
	conn, err := nats.Connect(url, nats.DrainTimeout(300*time.Millisecond))
	require.NoError(t, err)
	t.Cleanup(conn.Close)
	p := &JSProducer{Subject: "issue114", Conn: conn}
	called := make(chan struct{}, 1)
	p.JS, err = jetstream.New(conn, jetstream.WithPublishAsyncErrHandler(
		func(jetstream.JetStream, *nats.Msg, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			p.Disconnect()
		}))
	require.NoError(t, err)
	publishAsync(t, p, published)

	select {
	case <-disconnect(p):
	case <-time.After(3 * time.Second):
		t.Fatal("Disconnect called from a PublishAsyncErrHandler never returned")
	}
	select {
	case <-called:
	default:
		t.Fatal("Disconnect never ran the PublishAsyncErrHandler for the pending future")
	}
}

func TestIssue114_DisconnectWithNothingPendingReturnsPromptly(t *testing.T) {
	tests := []struct {
		name    string
		publish bool
	}{
		{"never published", false},
		{"all acked", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ack := make(chan struct{})
			close(ack)
			url, published := ackServer(t, ack)
			p := fakeJSProducer(t, url, 10*time.Second)
			if tc.publish {
				paf := publishAsync(t, p, published)
				select {
				case <-paf.Ok():
				case err := <-paf.Err():
					t.Fatalf("publish failed: %v", err)
				case <-time.After(5 * time.Second):
					t.Fatal("ack never arrived")
				}
			}

			select {
			case <-disconnect(p):
			case <-time.After(2 * time.Second):
				t.Fatal("Disconnect waited with no acks pending")
			}
			assert.True(t, p.Conn.IsClosed())
		})
	}
}
