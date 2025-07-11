package eventbus

import (
	"context"
)

type Option interface {
	apply(*config)
}

type SubscriptionCallback[E any] func(context.Context, *Subscription[E])

type abstractSubscriptionCallback any

type config struct {
	onOverflow       OnOverflow
	beforeSubscribed abstractSubscriptionCallback
	onSubscribed     abstractSubscriptionCallback
	onUnsubscribe    abstractSubscriptionCallback
	queueSize        uint
}

type Options []Option

func (s Options) Config() config {
	cfg := config{
		onOverflow: OnOverflowWait(0),
		queueSize:  1,
	}
	for _, opt := range s {
		opt.apply(&cfg)
	}
	return cfg
}

type optionOnOverflowT struct {
	OnOverflow
}

func OptionOnOverflow(v OnOverflow) optionOnOverflowT {
	return optionOnOverflowT{
		OnOverflow: v,
	}
}

func (opt optionOnOverflowT) apply(cfg *config) {
	cfg.onOverflow = opt.OnOverflow
}

type OptionBeforeSubscribed[E any] SubscriptionCallback[E]

func (opt OptionBeforeSubscribed[E]) apply(cfg *config) {
	cfg.beforeSubscribed = SubscriptionCallback[E](opt)
}

type OptionOnSubscribed[E any] SubscriptionCallback[E]

func (opt OptionOnSubscribed[E]) apply(cfg *config) {
	cfg.onSubscribed = SubscriptionCallback[E](opt)
}

type OptionOnUnsubscribe[E any] SubscriptionCallback[E]

func (opt OptionOnUnsubscribe[E]) apply(cfg *config) {
	cfg.onUnsubscribe = SubscriptionCallback[E](opt)
}

type OptionQueueSize uint

func (opt OptionQueueSize) apply(cfg *config) {
	cfg.queueSize = uint(opt)
}
