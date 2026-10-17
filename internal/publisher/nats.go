package publisher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/taagomag/m-generator/internal/config"
	"github.com/taagomag/m-generator/internal/generator"
)

const (
	defaultPublishAttempts = 3
	defaultPublishBackoff  = 100 * time.Millisecond
	defaultPublishTimeout  = 2 * time.Second
)

type NATSPublisher struct {
	conn     *nats.Conn
	js       jetstream.JetStream
	subject  string
	attempts int
	backoff  time.Duration
	timeout  time.Duration
	logger   *slog.Logger
}

func newPublisher(conn *nats.Conn, js jetstream.JetStream, subject string) *NATSPublisher {
	return &NATSPublisher{
		conn:     conn,
		js:       js,
		subject:  subject,
		attempts: defaultPublishAttempts,
		backoff:  defaultPublishBackoff,
		timeout:  defaultPublishTimeout,
		logger:   slog.Default(),
	}
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
		return newPublisher(conn, js, cfg.SubjectPrefix+".batch"), nil
	}

	stream, err := js.Stream(ctx, cfg.Stream)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("get existing JetStream stream %q: %w", cfg.Stream, err)
	}
	info := stream.CachedInfo()
	for _, subject := range info.Config.Subjects {
		if subject == subjectPattern {
			return newPublisher(conn, js, cfg.SubjectPrefix+".batch"), nil
		}
	}
	conn.Close()
	return nil, fmt.Errorf("JetStream stream %q exists but does not capture subject %q", cfg.Stream, subjectPattern)
}

// Publish sends one batch, retrying failed attempts with exponential backoff.
// All attempts reuse the same message ID so JetStream drops a duplicate if an
// earlier attempt was stored but its acknowledgment was lost.
func (p *NATSPublisher) Publish(ctx context.Context, metrics []generator.Metric) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("encode metrics as JSON: %w", err)
	}
	msgID, err := newMessageID()
	if err != nil {
		return fmt.Errorf("create message ID: %w", err)
	}
	backoff := p.backoff
	for attempt := 1; ; attempt++ {
		err := p.publishOnce(ctx, data, msgID)
		if err == nil {
			if attempt > 1 {
				p.logger.Info("published batch after retry", "subject", p.subject, "attempt", attempt, "msg_id", msgID)
			}
			return nil
		}
		willRetry := attempt < p.attempts && ctx.Err() == nil
		p.logger.Warn("publish attempt failed",
			"subject", p.subject,
			"attempt", attempt,
			"max_attempts", p.attempts,
			"will_retry", willRetry,
			"error", err,
		)
		if !willRetry {
			return fmt.Errorf("publish to NATS subject %q after %d attempt(s): %w", p.subject, attempt, err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("publish to NATS subject %q: %w", p.subject, ctx.Err())
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

func (p *NATSPublisher) publishOnce(ctx context.Context, data []byte, msgID string) error {
	publishCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	_, err := p.js.Publish(publishCtx, p.subject, data, jetstream.WithMsgID(msgID))
	return err
}

func newMessageID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (p *NATSPublisher) Close() {
	p.conn.Close()
}
