package stats

import (
	"io"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// emitterBuffer is how many lines can wait for a slow output before new
// ones are dropped.
const emitterBuffer = 1024

// emitter writes lines to an output in the background, so a slow or stalled
// output never holds up recording: when its buffer is full, it drops lines,
// and counts them.
type emitter struct {
	queue   chan []byte
	dropped prometheus.Counter
	// stop is closed to stop writing, once what's queued is written.
	stop chan struct{}
	// stopped is closed once the emitter has stopped.
	stopped chan struct{}
	// deadline is how long it may take to write what's queued once stop is
	// closed. It's set before stop is closed.
	deadline time.Time
}

// newEmitter starts an emitter writing to output, counting the lines it
// drops in dropped, until close.
func newEmitter(output io.Writer, dropped prometheus.Counter) *emitter {
	e := &emitter{
		queue:   make(chan []byte, emitterBuffer),
		dropped: dropped,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go e.run(output)
	return e
}

// run writes the queued lines to output until stop is closed, then what's
// still queued, until the queue is empty or the deadline has passed.
func (e *emitter) run(output io.Writer) {
	defer close(e.stopped)
	for {
		select {
		case b := <-e.queue:
			_, _ = output.Write(b)
		case <-e.stop:
			drain(e.queue, e.deadline, func(b []byte) { _, _ = output.Write(b) })
			return
		}
	}
}

// emit queues b to be written, or drops it if the queue is full.
func (e *emitter) emit(b []byte) {
	select {
	case e.queue <- b:
	default:
		e.dropped.Inc()
	}
}

// close writes what's queued and stops, waiting until deadline at most. A
// write that's stalled past it is left behind. Lines emitted after close
// are never written.
func (e *emitter) close(deadline time.Time) {
	e.deadline = deadline
	close(e.stop)
	waitUntil(e.stopped, deadline)
}

// drain passes what's in queue to handle until it's empty or deadline has
// passed. It never waits for more.
func drain[T any](queue <-chan T, deadline time.Time, handle func(T)) {
	for time.Now().Before(deadline) {
		select {
		case v := <-queue:
			handle(v)
		default:
			return
		}
	}
}

// waitUntil waits until done is closed or deadline has passed.
func waitUntil(done <-chan struct{}, deadline time.Time) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}
