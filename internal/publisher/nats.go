package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/taagomag/m-generator/internal/config"
	"github.com/taagomag/m-generator/internal/generator"
)

type NATSPublisher struct {
	conn    *nats.Conn
	js      jetstream.JetStream
	subject string
}

func NewNATSPublisher(ctx context.Context, cfg config.NATSConfig) (*NATSPublisher, error) {
	conn, err := nats.Connect(cfg.URL, nats.Name("metrics-generator"), nats.Timeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("initialize JetStream: %w", err)
	}
	subjectPattern := cfg.SubjectPrefix + ".>"
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     cfg.Stream,
		Subjects: []string{subjectPattern},
		Storage:  jetstream.FileStorage,
	})
	if err != nil && !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		conn.Close()
		return nil, fmt.Errorf("create JetStream stream %q: %w", cfg.Stream, err)
	}
	if err == nil {
		return &NATSPublisher{
			conn:    conn,
			js:      js,
			subject: cfg.SubjectPrefix + ".batch",
		}, nil
	}

	stream, err := js.Stream(ctx, cfg.Stream)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("get existing JetStream stream %q: %w", cfg.Stream, err)
	}
	info := stream.CachedInfo()
	for _, subject := range info.Config.Subjects {
		if subject == subjectPattern {
			return &NATSPublisher{
				conn:    conn,
				js:      js,
				subject: cfg.SubjectPrefix + ".batch",
			}, nil
		}
	}
	conn.Close()
	return nil, fmt.Errorf("JetStream stream %q exists but does not capture subject %q", cfg.Stream, subjectPattern)
}

func (p *NATSPublisher) Publish(ctx context.Context, metrics []generator.Metric) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("encode metrics as JSON: %w", err)
	}
	publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := p.js.Publish(publishCtx, p.subject, data); err != nil {
		return fmt.Errorf("publish to NATS subject %q: %w", p.subject, err)
	}
	return nil
}

func (p *NATSPublisher) Close() {
	p.conn.Close()
}
