package repoindexer

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"golang.org/x/xerrors"

	"go.f110.dev/mono/go/pkg/logger"
)

const (
	StreamName    = "repoindexer"
	NotifySubject = StreamName + "." + "notify"
)

type Notify struct {
	js nats.JetStreamContext
}

func NewNotify(u string) (*Notify, error) {
	nc, err := nats.Connect(u)
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	js, err := nc.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	n := &Notify{js: js}

	_, err = js.StreamInfo(StreamName)
	if errors.Is(err, nats.ErrStreamNotFound) {
		if err := n.setupStream(); err != nil {
			return nil, xerrors.Errorf(": %w", err)
		}
	}
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}

	return n, nil
}

func (n *Notify) Notify(ctx context.Context, urls []string) error {
	for _, v := range urls {
		pubAck, err := n.js.PublishAsync(NotifySubject, []byte(v))
		if err != nil {
			return xerrors.Errorf(": %w", err)
		}
		select {
		case <-pubAck.Ok():
		case err := <-pubAck.Err():
			return xerrors.Errorf(": %w", err)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	select {
	case <-n.js.PublishAsyncComplete():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *Notify) Subscribe() (*Subscription, error) {
	ch := make(chan string)
	sub, err := n.js.Subscribe(NotifySubject, func(msg *nats.Msg) {
		ch <- string(msg.Data)
	})
	if err != nil {
		return nil, xerrors.Errorf(": %w", err)
	}
	subscription := &Subscription{ch: ch, done: make(chan struct{})}
	go func() {
		select {
		case <-subscription.done:
			if err := sub.Unsubscribe(); err != nil {
				logger.Log.Info("Failed unsubscribe", zap.Error(err))
			}
		}
	}()

	return subscription, nil
}

func (n *Notify) setupStream() error {
	_, err := n.js.AddStream(&nats.StreamConfig{
		Name:      StreamName,
		Subjects:  []string{StreamName + ".*"},
		Retention: nats.InterestPolicy,
	})
	if err != nil {
		return xerrors.Errorf(": %w", err)
	}

	return nil
}

type Subscription struct {
	ch   chan string
	done chan struct{}
}

func (s *Subscription) Close() {
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
}
