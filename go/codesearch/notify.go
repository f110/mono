package codesearch

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"go.f110.dev/xerrors"

	"go.f110.dev/mono/go/logger/slogger"
)

type Notify struct {
	subject string

	js nats.JetStreamContext
}

func NewNotify(u, streamName, subject string) (*Notify, error) {
	nc, err := nats.Connect(u, nats.RetryOnFailedConnect(true), nats.PingInterval(30*time.Second))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	js, err := nc.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	n := &Notify{subject: fmt.Sprintf("%s.%s", streamName, subject), js: js}

	si, err := js.StreamInfo(streamName)
	if errors.Is(err, nats.ErrStreamNotFound) {
		if err = n.setupStream(streamName); err != nil {
			return nil, xerrors.WithStack(err)
		}
	} else {
		slogger.Log.Debug("Exist stream", slog.Any("stream_info", si))
	}
	if err != nil {
		return nil, xerrors.WithStack(err)
	}

	return n, nil
}

func (n *Notify) Notify(ctx context.Context, manifest *Manifest) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, manifest.ExecutionKey)
	pubAck, err := n.js.PublishAsync(n.subject, buf)
	if err != nil {
		return xerrors.WithStack(err)
	}
	select {
	case <-pubAck.Ok():
	case err := <-pubAck.Err():
		return xerrors.WithStack(err)
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case <-n.js.PublishAsyncComplete():
		slogger.Log.Debug("Notify", slog.String("subject", n.subject))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *Notify) Subscribe(manifestManager *ManifestManager) (*Subscription, error) {
	ch := make(chan Manifest)
	sub, err := n.js.Subscribe(n.subject, func(msg *nats.Msg) {
		executionKey := binary.LittleEndian.Uint64(msg.Data)
		manifest, err := manifestManager.Get(context.TODO(), executionKey)
		if err != nil {
			slogger.Log.Info("Failed get manifest", slogger.E(err), slog.Uint64("key", executionKey))
			return
		}
		ch <- manifest
		if err := msg.Ack(); err != nil {
			slogger.Log.Warn("Something occurred when acknowledge", slogger.E(err), slog.Uint64("key", executionKey))
		}
	})
	if err != nil {
		return nil, xerrors.WithStack(err)
	}
	subscription := &Subscription{ch: ch, done: make(chan struct{})}
	go func() {
		select {
		case <-subscription.done:
			if err := sub.Unsubscribe(); err != nil {
				slogger.Log.Info("Failed unsubscribe", slogger.E(err))
			}
		}
	}()

	return subscription, nil
}

func (n *Notify) setupStream(streamName string) error {
	slogger.Log.Info("Add stream", slog.String("subject", streamName+".*"))
	_, err := n.js.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{streamName + ".*"},
		Retention: nats.InterestPolicy,
	})
	if err != nil {
		return xerrors.WithStack(err)
	}

	return nil
}

type Subscription struct {
	ch   chan Manifest
	done chan struct{}
}

func (s *Subscription) Close() {
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
}
