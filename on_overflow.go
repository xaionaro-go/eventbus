package eventbus

import (
	"time"
)

// OnOverflow defines a behavior if unable to immediately send an event to an subscriber
// due to the queue being already full.
type OnOverflow interface {
	isOverflow()
}

// OnOverflowWait is an OnOverflow that waits for the given duration
// (which is infinite if value is <= 0), and drops the event if
// ultimately was unable to send it.
type OnOverflowWait time.Duration

var _ OnOverflow = OnOverflowWait(0)

func (OnOverflowWait) isOverflow() {}

// OnOverflowDrop is an OnOverflow that drops the event.
type OnOverflowDrop struct{}

var _ OnOverflow = OnOverflowDrop{}

func (OnOverflowDrop) isOverflow() {}

// OnOverflowClose is an OnOverflow that closes the subscription.
type OnOverflowClose struct{}

var _ OnOverflow = OnOverflowClose{}

func (OnOverflowClose) isOverflow() {}

// OnOverflowWaitOrClose is an OnOverflow that waits for the given duration
// (which is infinite if value is <= 0), and closes the subscription if
// ultimately was unable to send the event.
type OnOverflowWaitOrClose time.Duration

var _ OnOverflow = OnOverflowWaitOrClose(0)

func (OnOverflowWaitOrClose) isOverflow() {}

type onOverflowPileUpOrClose struct {
	PileSize uint
	Timeout  time.Duration
}

var _ OnOverflow = onOverflowPileUpOrClose{}

func OnOverflowPileUpOrClose(
	pileSize uint,
	timeout time.Duration,
) onOverflowPileUpOrClose {
	return onOverflowPileUpOrClose{
		PileSize: pileSize,
		Timeout:  timeout,
	}
}

func (onOverflowPileUpOrClose) isOverflow() {}
