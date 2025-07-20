package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/facebookincubator/go-belt/tool/logger"
)

type Subscription[T, E any] struct {
	canceler        *triggerable
	readier         *triggerable
	finished        *triggerable
	eventBus        *EventBus
	topic           T
	eventChan       chan E
	eventChanLocker sync.RWMutex
	pile            chan E
	config
}

func newSubscription[T, E any](
	ctx context.Context,
	bus *EventBus,
	topic T,
	opts ...Option,
) *Subscription[T, E] {
	cfg := Options(opts).Config()
	sub := &Subscription[T, E]{
		canceler:  newTriggerable(ctx),
		readier:   newTriggerable(ctx),
		finished:  newTriggerable(context.Background()),
		eventBus:  bus,
		topic:     topic,
		eventChan: make(chan E, cfg.queueSize),
		config:    cfg,
	}
	switch onOverflow := cfg.onOverflow.(type) {
	case onOverflowPileUpOrClose:
		sub.pile = make(chan E, onOverflow.PileSize)
		go sub.pileHandler(ctx)
	}
	return sub
}

func (sub *Subscription[T, E]) EventChan() chan E {
	return sub.eventChan
}

func (sub *Subscription[T, E]) Finish(ctx context.Context) bool {
	return UnsubscribeWithCustomTopic(ctx, sub.eventBus, sub.topic, sub)
}

type sendEventToSubResult int

const (
	sendEventToSubResultUndefined = sendEventToSubResult(iota)
	sendEventToSubResultSent
	sendEventToSubResultPiled
	sendEventToSubResultDropped
	sendEventToSubResultDroppedUnsubscribe
	sendEventToSubResultUnsubscribe
	sendEventToSubResultDeferred
)

func (sub *Subscription[T, E]) sendEvent(
	ctx context.Context,
	event E,
	deferrable bool,
) (_ret sendEventToSubResult) {
	if isTraceEnabled(ctx) {
		logger.Tracef(ctx, "sendEvent[%T](ctx, event, %t)", event, deferrable)
		defer func() { logger.Tracef(ctx, "/sendEvent[%T](ctx, event, %t): %v", event, deferrable, _ret) }()
	}

	return sub.doSendEvent(ctx, event, deferrable)
}

func (sub *Subscription[T, E]) pileHandler(
	ctx context.Context,
) {
	if isTraceEnabled(ctx) {
		var sample E
		logger.Tracef(ctx, "pileHandler[%T](ctx)", sample)
		defer func() { logger.Tracef(ctx, "/pileHandler[%T](ctx)", sample) }()
	}

	onOverflow := sub.onOverflow.(onOverflowPileUpOrClose)

	var cancelFn context.CancelFunc
	if onOverflow.Timeout > 0 {
		defer func() {
			if cancelFn != nil {
				cancelFn()
			}
		}()
	}
	for {
		var ev E
		select {
		case <-ctx.Done():
			return
		case <-sub.Done():
			return
		case ev = <-sub.pile:
		}
		func() {
			sub.eventChanLocker.RLock()
			defer sub.eventChanLocker.RUnlock()
			eventChan := sub.eventChan
			if eventChan == nil {
				return
			}
			waitCtx := ctx
			if onOverflow.Timeout > 0 {
				if cancelFn != nil {
					cancelFn()
				}
				waitCtx, cancelFn = context.WithTimeout(ctx, onOverflow.Timeout)
			}
			select {
			case <-waitCtx.Done():
				// timed out, closing:
				UnsubscribeWithCustomTopic(ctx, sub.eventBus, sub.topic, sub)
				return
			case <-sub.Done():
				return
			case eventChan <- ev:
			}
		}()
	}
}

func (sub *Subscription[T, E]) doSendEvent(
	ctx context.Context,
	event E,
	deferrable bool,
) (_ret sendEventToSubResult) {
	select {
	case <-sub.Done():
		return sendEventToSubResultUnsubscribe
	default:
	}

	// the locking is to prevent `sub.eventChan` from closing
	var eventChan chan E
	if len(sub.pile) == 0 {
		sub.eventChanLocker.RLock()
		defer sub.eventChanLocker.RUnlock()
		eventChan = sub.eventChan
		if eventChan == nil {
			return sendEventToSubResultDropped
		}
	}
	return handleSubChans(
		ctx,
		eventChan, sub.pile, sub.canceler.Done(),
		event,
		deferrable, sub.onOverflow,
	)
}

func (sub *Subscription[T, E]) Done() <-chan struct{} {
	return sub.canceler.Done()
}

func (sub *Subscription[T, E]) Cancel() {
	sub.canceler.Trigger()
}

func (sub *Subscription[T, E]) Ready() <-chan struct{} {
	return sub.readier.Done()
}

func (sub *Subscription[T, E]) SetReady() {
	sub.readier.Trigger()
}

func handleSubChans[E any](
	ctx context.Context,
	eventChan chan<- E,
	pile chan<- E,
	subDone <-chan struct{},
	event E,
	deferrable bool,
	onOverflow OnOverflow,
) (_ret sendEventToSubResult) {
	if isTraceEnabled(ctx) {
		logger.Tracef(ctx, "handleSubChans")
		defer func() { logger.Tracef(ctx, "/handleSubChans: %v", _ret) }()
	}
	if !deferrable {
		return handleSubChansSync(ctx, eventChan, subDone, event, onOverflow)
	}
	return handleSubChansDeferrable(ctx, eventChan, pile, subDone, event, onOverflow)
}

func handleSubChansDeferrable[E any](
	ctx context.Context,
	eventChan chan<- E,
	pile chan<- E,
	subDone <-chan struct{},
	event E,
	onOverflow OnOverflow,
) sendEventToSubResult {
	select {
	case <-ctx.Done():
		return sendEventToSubResultDropped
	case <-subDone:
		return sendEventToSubResultUnsubscribe
	case eventChan <- event:
		return sendEventToSubResultSent
	default:
		switch onOverflow.(type) {
		case OnOverflowWait, OnOverflowWaitOrClose:
			return sendEventToSubResultDeferred
		case OnOverflowDrop:
			return sendEventToSubResultDropped
		case OnOverflowClose:
			return sendEventToSubResultDroppedUnsubscribe
		case onOverflowPileUpOrClose:
			select {
			case <-ctx.Done():
				return sendEventToSubResultDropped
			case <-subDone:
				return sendEventToSubResultUnsubscribe
			case eventChan <- event:
				return sendEventToSubResultSent
			case pile <- event:
				return sendEventToSubResultPiled
			default:
				return sendEventToSubResultDroppedUnsubscribe
			}
		default:
			panic(fmt.Errorf("unexpected value: %T:%#+v", onOverflow, onOverflow))
		}
	}
}

func handleSubChansSync[E any](
	ctx context.Context,
	eventChan chan<- E,
	subDone <-chan struct{},
	event E,
	onOverflow OnOverflow,
) sendEventToSubResult {
	var waitDuration time.Duration
	switch onOverflow := onOverflow.(type) {
	case OnOverflowWait:
		waitDuration = time.Duration(onOverflow)
	case OnOverflowWaitOrClose:
		waitDuration = time.Duration(onOverflow)
	case OnOverflowDrop:
		panic("internal error: this was supposed to be processed in handleSubChansDeferrable")
	case OnOverflowClose:
		panic("internal error: this was supposed to be processed in handleSubChansDeferrable")
	case onOverflowPileUpOrClose:
		panic("internal error: this was supposed to be processed in handleSubChansDeferrable")
	}
	waitCtx := ctx
	if waitDuration > 0 {
		var cancelFn context.CancelFunc
		waitCtx, cancelFn = context.WithTimeout(ctx, waitDuration)
		defer cancelFn()
	}

	select {
	case <-waitCtx.Done():
		if _, ok := onOverflow.(OnOverflowWaitOrClose); ok && !isCtxDone(ctx) {
			return sendEventToSubResultDroppedUnsubscribe
		}
		return sendEventToSubResultDropped
	case <-subDone:
		return sendEventToSubResultUnsubscribe
	case eventChan <- event:
		return sendEventToSubResultSent
	}
}

func isCtxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
