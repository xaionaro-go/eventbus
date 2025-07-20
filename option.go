package eventbus

import (
	"context"
)

type Option interface {
	apply(*config)
}

type SubscriptionCallback[T, E any] func(context.Context, *Subscription[T, E])

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

type OptionBeforeSubscribed[T, E any] SubscriptionCallback[T, E]

func (opt OptionBeforeSubscribed[T, E]) apply(cfg *config) {
	cfg.beforeSubscribed = SubscriptionCallback[T, E](opt)
}

type OptionOnSubscribed[T, E any] SubscriptionCallback[T, E]

func (opt OptionOnSubscribed[T, E]) apply(cfg *config) {
	cfg.onSubscribed = SubscriptionCallback[T, E](opt)
}

type OptionOnUnsubscribe[T, E any] SubscriptionCallback[T, E]

func (opt OptionOnUnsubscribe[T, E]) apply(cfg *config) {
	cfg.onUnsubscribe = SubscriptionCallback[T, E](opt)
}

type OptionQueueSize uint

func (opt OptionQueueSize) apply(cfg *config) {
	cfg.queueSize = uint(opt)
}
