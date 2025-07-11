package eventbus

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/facebookincubator/go-belt"
	"github.com/facebookincubator/go-belt/tool/logger"
	"github.com/xaionaro-go/xcontext"
)

type EventBus struct {
	chanLocker
	subscriptions map[any]map[any]struct{}
}

func New() *EventBus {
	return &EventBus{
		chanLocker:    make(chanLocker, 1),
		subscriptions: map[any]map[any]struct{}{},
	}
}

type SendEventResult struct {
	SentCountImmediate uint
	SentCountDeferred  uint
	PiledCount         uint
	DropCountImmediate uint
	DropCountDeferred  uint
}

func SendEvent[E any](
	ctx context.Context,
	bus *EventBus,
	event E,
) (result SendEventResult) {
	var zeroValue E
	return SendEventWithCustomTopic(ctx, bus, zeroValue, event)
}

func SendEventWithCustomTopic[T, E any](
	ctx context.Context,
	bus *EventBus,
	topic T,
	event E,
) (result SendEventResult) {
	if isTraceEnabled(ctx) {
		ctx = belt.WithField(ctx, "topic", fmt.Sprintf("%#+v", topic))
		logger.Tracef(ctx, "SendEventWithCustomTopic[%T, %T]", topic, event)
		defer func() {
			logger.Tracef(ctx, "/SendEventWithCustomTopic[%T, %T]: %v", topic, event, result)
		}()
	}
	var deferredSending []*Subscription[E]

	// bus locking zone (here we cannot wait, and should act swiftly)
	if !bus.Lock(ctx) {
		result.DropCountImmediate = math.MaxUint
		return
	}
	func() {
		defer bus.Unlock()
		if bus.subscriptions[topic] == nil {
			if isTraceEnabled(ctx) {
				logger.Tracef(ctx, "no subscriptions")
			}
			return
		}
		select {
		case <-ctx.Done():
			result.DropCountImmediate = uint(len(bus.subscriptions[topic]))
			return
		default:
		}
		for _sub := range bus.subscriptions[topic] {
			sub, ok := _sub.(*Subscription[E])
			if !ok {
				logger.Errorf(ctx, "invalid type %T, expected %T", _sub, (*Subscription[E])(nil))
				continue
			}
			switch r := sub.sendEvent(ctx, event, true); r {
			case sendEventToSubResultSent:
				result.SentCountImmediate++
			case sendEventToSubResultPiled:
				result.PiledCount++
			case sendEventToSubResultDropped:
				result.DropCountImmediate++
			case sendEventToSubResultDroppedUnsubscribe:
				result.DropCountImmediate++
				unsubscribe(xcontext.DetachDone(ctx), bus, sub, false)
			case sendEventToSubResultUnsubscribe:
				unsubscribe(xcontext.DetachDone(ctx), bus, sub, false)
			case sendEventToSubResultDeferred:
				deferredSending = append(deferredSending, sub)
			default:
				panic(fmt.Errorf("unexpected value: %d", r))
			}
		}
	}()

	// bus-lock-free zone (here we can wait)

	if len(deferredSending) > 0 {
		var successCount, dropCount atomic.Uint64
		var wg sync.WaitGroup
		for _, sub := range deferredSending {
			wg.Add(1)
			go func(sub *Subscription[E]) {
				defer wg.Done()
				switch r := sub.sendEvent(ctx, event, false); r {
				case sendEventToSubResultSent:
					successCount.Add(1)
				case sendEventToSubResultDropped:
					dropCount.Add(1)
				case sendEventToSubResultDroppedUnsubscribe:
					dropCount.Add(1)
					unsubscribe(xcontext.DetachDone(ctx), bus, sub, true)
				case sendEventToSubResultUnsubscribe:
					unsubscribe(xcontext.DetachDone(ctx), bus, sub, true)
				default:
					panic(fmt.Errorf("unexpected value: %d", r))
				}
			}(sub)
		}
		wg.Wait()
		result.SentCountDeferred = uint(successCount.Load())
		result.DropCountDeferred = uint(dropCount.Load())
	}

	return
}

func Subscribe[E any](
	ctx context.Context,
	bus *EventBus,
	opts ...Option,
) *Subscription[E] {
	var zeroValue E
	return SubscribeWithCustomTopic[E, E](ctx, bus, zeroValue, opts...)
}

func SubscribeWithCustomTopic[T, E any](
	ctx context.Context,
	bus *EventBus,
	topic T,
	opts ...Option,
) (_ret *Subscription[E]) {
	if isTraceEnabled(ctx) {
		var sample E
		ctx = belt.WithField(ctx, "topic", fmt.Sprintf("%#+v", topic))
		logger.Tracef(ctx, "SubscribeWithCustomTopic[%T]", sample)
		defer func() {
			logger.Tracef(ctx, "/SubscribeWithCustomTopic[%T]: %p", sample, _ret)
		}()
	}
	sub := newSubscription[E](ctx, bus, opts...)
	defer sub.readier.Trigger()

	if _beforeSubscribed := sub.beforeSubscribed; _beforeSubscribed != nil {
		beforeSubscribed, ok := _beforeSubscribed.(SubscriptionCallback[E])
		if !ok {
			logger.Errorf(ctx, "invalid type %T, expected %T", _beforeSubscribed, (SubscriptionCallback[E])(nil))
			return nil
		}
		beforeSubscribed(ctx, sub)
		if isTraceEnabled(ctx) {
			logger.Tracef(ctx, "finished beforeSubscribed")
		}
	}
	if _onSubscribed := sub.onSubscribed; _onSubscribed != nil {
		onSubscribed, ok := _onSubscribed.(SubscriptionCallback[E])
		if !ok {
			logger.Errorf(ctx, "invalid type %T, expected %T", _onSubscribed, (SubscriptionCallback[E])(nil))
			return nil
		}
		sub.eventChanLocker.Lock()
		defer func() {
			go func() {
				defer func() {
					if isTraceEnabled(ctx) {
						logger.Tracef(ctx, "finished onSubscribed")
					}
					sub.eventChanLocker.Unlock()
				}()
				onSubscribed(ctx, sub)
			}()
		}()
	}

	if !bus.Lock(ctx) {
		return nil
	}
	defer bus.Unlock()
	if bus.subscriptions[topic] == nil {
		bus.subscriptions[topic] = map[any]struct{}{}
	}
	bus.subscriptions[topic][sub] = struct{}{}
	return sub
}

func Unsubscribe[E any](
	ctx context.Context,
	bus *EventBus,
	sub *Subscription[E],
) bool {
	return unsubscribe(ctx, bus, sub, true)
}

func unsubscribe[E any](
	ctx context.Context,
	bus *EventBus,
	sub *Subscription[E],
	lockBus bool,
) bool {
	sub.Cancel()
	eventChan := func() chan E {
		sub.eventChanLocker.RLock()
		defer sub.eventChanLocker.RUnlock()
		return sub.eventChan
	}()
	if eventChan == nil {
		return false
	}
	go func() {
		sub.eventChanLocker.Lock()
		defer sub.eventChanLocker.Unlock()
		if _onUnsubscribe := sub.onUnsubscribe; _onUnsubscribe != nil {
			onUnsubscribe, ok := _onUnsubscribe.(SubscriptionCallback[E])
			if !ok {
				logger.Errorf(ctx, "invalid type %T, expected %T", _onUnsubscribe, (SubscriptionCallback[E])(nil))
			} else {
				onUnsubscribe(ctx, sub)
			}
		}
		if sub.eventChan == nil {
			return
		}
		close(sub.eventChan)
		sub.eventChan = nil
		sub.finished.Trigger()
	}()

	if lockBus {
		if !bus.Lock(ctx) {
			return false
		}
		defer bus.Unlock()
	}
	var zeroValue E
	if bus.subscriptions[zeroValue] == nil {
		return false
	}
	if _, ok := bus.subscriptions[zeroValue][sub]; !ok {
		return false
	}
	delete(bus.subscriptions[zeroValue], sub)
	return true
}
