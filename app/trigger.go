package app

import "sync"

// trigger runs a function in the background on request, coalescing
// requests: while a run is in progress, any number of further requests cause
// exactly one more run after it. That later run starts after every request
// it stands for, so it sees whatever they announced.
type trigger struct {
	run func()

	mu      sync.Mutex
	running bool
	pending bool
	wg      sync.WaitGroup
}

// fire requests a run without waiting for it.
func (t *trigger) fire() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		t.pending = true
		return
	}
	t.running = true
	t.wg.Go(t.loop)
}

// loop runs until no request is pending.
func (t *trigger) loop() {
	for {
		t.run()
		t.mu.Lock()
		if !t.pending {
			t.running = false
			t.mu.Unlock()
			return
		}
		t.pending = false
		t.mu.Unlock()
	}
}

// wait waits until no run is in progress or pending.
func (t *trigger) wait() {
	t.wg.Wait()
}
