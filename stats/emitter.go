package stats

import "io"

// emitterBuffer is how many lines can wait for a slow output before new
// ones are dropped.
const emitterBuffer = 1024

// emitter writes lines to an output in the background, so a slow or stalled
// output never holds up a request: when its buffer is full, it drops lines.
type emitter struct {
	queue chan []byte
}

// newEmitter starts an emitter writing to output, for the life of the
// process.
func newEmitter(output io.Writer) *emitter {
	e := &emitter{queue: make(chan []byte, emitterBuffer)}
	go func() {
		for b := range e.queue {
			_, _ = output.Write(b)
		}
	}()
	return e
}

// emit queues b to be written, or drops it if the queue is full.
func (e *emitter) emit(b []byte) {
	select {
	case e.queue <- b:
	default:
	}
}
