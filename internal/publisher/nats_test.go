package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/taagomag/m-generator/internal/config"
	"github.com/taagomag/m-generator/internal/generator"
)

var testMetrics = []generator.Metric{{
	Name:    "requests_total",
	Type:    "counter",
	Help:    "Total requests",
	Samples: []generator.Sample{{Name: "requests_total", Value: 1, Timestamp: 1}},
}}

type fakeJetStream struct {
	jetstream.JetStream
	errs  []error
	calls int
}

func (f *fakeJetStream) Publish(_ context.Context, _ string, _ []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	f.calls++
	if f.calls <= len(f.errs) && f.errs[f.calls-1] != nil {
		return nil, f.errs[f.calls-1]
	}
	return &jetstream.PubAck{}, nil
}

func newTestPublisher(js jetstream.JetStream, attempts int, backoff time.Duration) (*NATSPublisher, *bytes.Buffer) {
	var logs bytes.Buffer
	p := newPublisher(nil, js, "metrics.batch")
	p.attempts = attempts
	p.backoff = backoff
	p.timeout = time.Second
	p.logger = slog.New(slog.NewTextHandler(&logs, nil))
	return p, &logs
}

func TestPublishRetriesTransientFailure(t *testing.T) {
	js := &fakeJetStream{errs: []error{errors.New("ack timeout")}}
	p, logs := newTestPublisher(js, 3, time.Millisecond)

	if err := p.Publish(context.Background(), testMetrics); err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
	if js.calls != 2 {
		t.Fatalf("publish calls = %d, want 2", js.calls)
	}
	out := logs.String()
	if !strings.Contains(out, "publish attempt failed") || !strings.Contains(out, "will_retry=true") {
		t.Errorf("logs missing failed attempt entry:\n%s", out)
	}
	if !strings.Contains(out, "published batch after retry") {
		t.Errorf("logs missing retry success entry:\n%s", out)
	}
}

func TestPublishGivesUpAfterMaxAttempts(t *testing.T) {
	publishErr := errors.New("no responders")
	js := &fakeJetStream{errs: []error{publishErr, publishErr, publishErr}}
	p, logs := newTestPublisher(js, 3, time.Millisecond)

	err := p.Publish(context.Background(), testMetrics)
	if !errors.Is(err, publishErr) {
		t.Fatalf("Publish() error = %v, want wrapped %v", err, publishErr)
	}
	if js.calls != 3 {
		t.Fatalf("publish calls = %d, want 3", js.calls)
	}
	if strings.Count(logs.String(), "will_retry=true") != 2 || !strings.Contains(logs.String(), "will_retry=false") {
		t.Errorf("unexpected retry logs:\n%s", logs.String())
	}
}

func TestPublishStopsWaitingWhenContextIsCancelled(t *testing.T) {
	publishErr := errors.New("no responders")
	js := &fakeJetStream{errs: []error{publishErr, publishErr, publishErr}}
	p, _ := newTestPublisher(js, 3, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(20*time.Millisecond, cancel)

	done := make(chan error, 1)
	go func() { done <- p.Publish(ctx, testMetrics) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Publish() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Publish() did not return after context cancellation")
	}
}

func startNATSServer(t *testing.T) string {
	t.Helper()
	srv, err := natsserver.NewServer(&natsserver.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	})
	if err != nil {
		t.Fatalf("create NATS server: %v", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(srv.Shutdown)
	return srv.ClientURL()
}

func TestNATSPublisherStoresBatchInStream(t *testing.T) {
	ctx := context.Background()
	url := startNATSServer(t)
	p, err := NewNATSPublisher(ctx, config.NATSConfig{URL: url, Stream: "METRICS", SubjectPrefix: "metrics"})
	if err != nil {
		t.Fatalf("NewNATSPublisher() error = %v", err)
	}
	t.Cleanup(p.Close)

	if err := p.Publish(ctx, testMetrics); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	stream, err := p.js.Stream(ctx, "METRICS")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	msg, err := stream.GetLastMsgForSubject(ctx, "metrics.batch")
	if err != nil {
		t.Fatalf("get last message: %v", err)
	}
	want, err := json.Marshal(testMetrics)
	if err != nil {
		t.Fatal(err)
	}
	if string(msg.Data) != string(want) {
		t.Errorf("stored data = %s, want %s", msg.Data, want)
	}
	if msg.Header.Get(nats.MsgIdHdr) == "" {
		t.Errorf("stored message has no %s header", nats.MsgIdHdr)
	}
}

func TestPublishOnceDeduplicatesSameMessageID(t *testing.T) {
	ctx := context.Background()
	url := startNATSServer(t)
	p, err := NewNATSPublisher(ctx, config.NATSConfig{URL: url, Stream: "METRICS", SubjectPrefix: "metrics"})
	if err != nil {
		t.Fatalf("NewNATSPublisher() error = %v", err)
	}
	t.Cleanup(p.Close)

	data := []byte(`[{"name":"requests_total"}]`)
	for range 2 {
		if err := p.publishOnce(ctx, data, "batch-1"); err != nil {
			t.Fatalf("publishOnce() error = %v", err)
		}
	}

	stream, err := p.js.Stream(ctx, "METRICS")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("stream holds %d messages, want 1 after duplicate publish", info.State.Msgs)
	}
}

func TestNewNATSPublisherRejectsStreamWithoutSubject(t *testing.T) {
	ctx := context.Background()
	url := startNATSServer(t)
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: "OTHER", Subjects: []string{"other.>"}}); err != nil {
		t.Fatalf("create stream: %v", err)
	}

	_, err = NewNATSPublisher(ctx, config.NATSConfig{URL: url, Stream: "OTHER", SubjectPrefix: "metrics"})
	if err == nil || !strings.Contains(err.Error(), "does not capture subject") {
		t.Fatalf("NewNATSPublisher() error = %v, want subject mismatch", err)
	}
}
