package eventbus

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventBus(t *testing.T) {

	bus := New()

	for iteration := range 100 {
		t.Run(fmt.Sprintf("iteration_%d", iteration), func(t *testing.T) {
			t.Run("OnOverflowDrop", func(t *testing.T) {
				ctx, cancelFn := context.WithCancel(context.Background())
				sub := Subscribe[uint64](ctx, bus, OptionOnOverflow(OnOverflowDrop{}))
				if iteration%2 == 0 {
					defer sub.Finish(ctx)
				}

				r := SendEvent[uint64](ctx, bus, 1)
				require.Equal(t, SendEventResult{
					SentCountImmediate: 1,
				}, r)
				r = SendEvent[uint64](ctx, bus, 1)
				require.Equal(t, SendEventResult{
					DropCountImmediate: 1,
				}, r)
				<-sub.eventChan
				cancelFn()
				r = SendEvent[uint64](ctx, bus, 1)
			})

			t.Run("OnOverflowWait", func(t *testing.T) {
				ctx := context.Background()
				sub := Subscribe[uint](ctx, bus, OptionOnOverflow(OnOverflowWait(0)))
				if iteration%2 == 0 {
					defer sub.Finish(ctx)
				}

				r := SendEvent[uint](ctx, bus, 1)
				require.Equal(t, SendEventResult{
					SentCountImmediate: 1,
				}, r)
				go func() {
					for range 100 {
						runtime.Gosched()
					}
					<-sub.eventChan
				}()
				r = SendEvent[uint](ctx, bus, 1)
				require.Equal(t, SendEventResult{
					SentCountDeferred: 1,
				}, r)
				go func() {
					for range 100 {
						runtime.Gosched()
					}
					sub.Finish(ctx)
				}()
				r = SendEvent[uint](ctx, bus, 1)
				require.Equal(t, SendEventResult{}, r)
			})

			t.Run("OnOverflowClose", func(t *testing.T) {
				ctx := context.Background()
				sub := Subscribe[byte](ctx, bus, OptionOnOverflow(OnOverflowClose{}))
				if iteration%2 == 0 {
					defer sub.Finish(ctx)
				}

				r := SendEvent[byte](ctx, bus, 1)
				require.Equal(t, SendEventResult{
					SentCountImmediate: 1,
				}, r)
				select {
				case <-sub.Done():
					t.Fatalf("the subscription is not supposed to be closed")
				default:
				}

				r = SendEvent[byte](ctx, bus, 1)
				require.Equal(t, SendEventResult{
					DropCountImmediate: 1,
				}, r)
				<-sub.finished.Done()
				select {
				case <-sub.Done():
				default:
					t.Fatalf("the subscription is supposed to be closed")
				}
			})

			t.Run("onOverflowPileUpOrClose", func(t *testing.T) {
				ctx := context.Background()
				sub := Subscribe[string](ctx, bus, OptionOnOverflow(OnOverflowPileUpOrClose(1, time.Nanosecond)))
				if iteration%2 == 0 {
					defer sub.Finish(ctx)
				}

				r := SendEvent(ctx, bus, "1")
				require.Equal(t, SendEventResult{
					SentCountImmediate: 1,
				}, r)
				select {
				case <-sub.Done():
					t.Fatalf("the subscription is not supposed to be closed")
				default:
				}

				r = SendEvent(ctx, bus, "1")
				require.Equal(t, SendEventResult{
					PiledCount: 1,
				}, r)
				<-sub.finished.Done() // because we have a nanosecond timeout of the pile
				select {
				case <-sub.Done():
				default:
					t.Fatalf("the subscription is supposed to be closed")
				}
			})
		})
	}
}

func BenchmarkSendEvent(b *testing.B) {
	ctx := context.Background()
	for subCount := 0; subCount <= 1024; {
		b.Run(fmt.Sprintf("subCount%d", subCount), func(b *testing.B) {
			bus := New()
			for range subCount {
				Subscribe[int](ctx, bus, OptionOnOverflow(OnOverflowDrop{}))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				SendEvent(ctx, bus, 0)
			}
		})
		if subCount == 0 {
			subCount++
		} else {
			subCount *= 2
		}
	}
}
