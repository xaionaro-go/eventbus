# `eventbus`

[![PkgGoDev](https://pkg.go.dev/badge/github.com/xaionaro-go/eventbus)](https://pkg.go.dev/github.com/xaionaro-go/eventbus#pkg-index)

`eventbus` is a naive implementation of a type-safe thread-safe optimistically-sending-event-zero-allocation event bus.

Initially I used [github.com/asaskevich/EventBus](https://pkg.go.dev/github.com/asaskevich/EventBus), but it was buggy (deadlocks) and on top of that quite inconvenient. So I just quickly thrown together a bit of code to implement an event bus from scratch. This type with type safety, no deadlock and enough flexibility.

Project priorities:
* Safety
* Flexibility
* User-friendliness (e.g. cancel&cleanup by simply `Done()`-ing the context).

Non-priorities:
* Performance fine-tuning.

## Quick start

Make an event type:
```go
type MyCustomEvent struct {
    // ...fields...
}
```

Init:
```go
bus := eventbus.New()
```

Subscribe:
```go
ctx, cancelFn := context.WithCancel(ctx)
sub := eventbus.Subscribe[MyCustomEvent](
    ctx,
    bus,
    eventbus.OptionQueueSize(10),
)
for ev := range sub.EventChan() {
    // ...do something with `ev`, which is already of type `MyCustomEvent`...
}
```

Send an event:
```go
eventbus.SendEvent(ctx, bus, MyCustomEvent{ /*...field values...*/ })
```
(the `for range` above will receive the event)

To cancel the subscription:
```go
cancelFn()
```
or
```go
sub.Finish(context.Background())
```

If you need a custom topic, instead of using the event type as the topic then:
```go
sub := eventbus.SubscribeWithCustomTopic[MyCustomEvent](
    ctx,
    bus, "my-custom-topic"
    eventbus.OptionQueueSize(10),
)
```
and:
```go
eventbus.SendEventWithCustomTopic(ctx, bus, "my-custom-topic", MyCustomEvent{ /*...field values...*/ })
```

## Logging

For example, if you use `logrus`:
```go
import (
    "github.com/sirupsen/logrus"
    beltlogger "github.com/facebookincubator/go-belt/tool/logger"
    beltlogrus "github.com/facebookincubator/go-belt/tool/logger/implementation/logrus"
)

...

    eventbus.LoggingEnabled = true
    myLogrusLogger.SetLevel(logrus.TraceLevel)
    ctx = beltlogger.CtxWithLogger(ctx, beltlogrus.New(myLogrusLogger))

    ...

    eventbus.SendEventWithCustomTopic(ctx, bus, "my-custom-topic", MyCustomEvent{ /*...field values...*/ })

```

But other loggers are also supported.

## Benchmark
```
goos: linux
goarch: amd64
pkg: github.com/xaionaro-go/eventbus
cpu: AMD Ryzen 9 5900X 12-Core Processor            
BenchmarkSendEvent/subCount0-24         	14530309	        76.87 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount1-24         	 3626359	       318.9 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount2-24         	 2091470	       555.3 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount4-24         	 1459594	       792.6 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount8-24         	  864446	      1529 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount16-24        	  423340	      2759 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount32-24        	  228435	      5341 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount64-24        	  122569	     10100 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount128-24       	   61632	     21412 ns/op	       0 B/op	       0 allocs/op
BenchmarkSendEvent/subCount256-24       	   26605	     40580 ns/op	       1 B/op	       0 allocs/op
BenchmarkSendEvent/subCount512-24       	   14868	     83902 ns/op	       3 B/op	       0 allocs/op
BenchmarkSendEvent/subCount1024-24      	    5662	    178440 ns/op	      20 B/op	       0 allocs/op
```

You can remove logging, replace `chanLocker` with normal `sync.Mutex` and perform other trivial optimizations, and it will be at least 2-3 times faster (e.g. in the case of a single subscriber). But we consciously don't care about that: we care about usability more than about performance.

## Examples of usage:

* [`streamctl`](https://github.com/xaionaro-go/streamctl)
