package bfd

import (
	"sync"
	"testing"
	"time"

	cmn "github.com/loxilb-io/loxilb/common"
)

// recNotifier records deliveries. It takes the session mutex on every
// delivery, so if the notifier ever called it with that mutex held the
// test would self-deadlock and time out.
type recNotifier struct {
	sess   *bfdSession
	mu     sync.Mutex
	states []string
}

func (n *recNotifier) BFDSessionNotify(_ string, _ string, state string) {
	if n.sess != nil {
		n.sess.Mutex.Lock()
		n.sess.Mutex.Unlock()
	}
	n.mu.Lock()
	n.states = append(n.states, state)
	n.mu.Unlock()
}

func (n *recNotifier) snapshot() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.states...)
}

func newTestSession(t *testing.T, myDisc uint32) (*bfdSession, *recNotifier) {
	t.Helper()
	b := &bfdSession{
		Instance:   cmn.CIDefault,
		RemoteName: "127.0.0.1:3784",
		State:      BFDDown,
		MyDisc:     myDisc,
		MyMulti:    1,
		Fin:        make(chan bool),
		ntfSig:     make(chan struct{}, 1),
		ntfFin:     make(chan struct{}),
	}
	n := &recNotifier{sess: b}
	b.Notify = n
	go b.bfdSessionNotifier()
	t.Cleanup(func() { close(b.ntfFin) })
	return b, n
}

func waitDrained(t *testing.T, b *bfdSession, n *recNotifier) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.Mutex.RLock()
		pending := b.ntfPending
		b.Mutex.RUnlock()
		if pending == "" && len(b.ntfSig) == 0 {
			// give the notifier a moment to finish an in-flight delivery
			before := len(n.snapshot())
			time.Sleep(20 * time.Millisecond)
			if len(n.snapshot()) == before {
				return n.snapshot()
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("notifier did not drain")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestElectionNotifyOrder hammers the listener path (RunSessionSM), the
// timeout path (checkSessTimeout) and the transmit path (encodeCtrlPacket)
// concurrently. Under -race this catches unlocked access; the assertion
// checks that the last delivered role is the role the session believes it
// has, which is the invariant that keeps force-reelection working.
func TestElectionNotifyOrder(t *testing.T) {
	const myDisc = 1000
	b, n := newTestSession(t, myDisc)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// listener: peer flips between a lower and a higher discriminator
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			disc := uint32(myDisc - 1)
			if i%2 == 1 {
				disc = myDisc + 1
			}
			// DesMinTxInt=1 with Multi=1 makes the timeout path fire on
			// almost every tick without sleeping in the test.
			b.RunSessionSM(&WireRaw{State: BFDUp, Multi: 1, Disc: disc, DesMinTxInt: 1})
			i++
		}
	}()

	// timeout path
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b.checkSessTimeout()
		}
	}()

	// transmit path
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			b.encodeCtrlPacket()
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()

	states := waitDrained(t, b, n)
	if len(states) == 0 {
		t.Fatalf("no notifications delivered")
	}
	b.Mutex.RLock()
	ciState := b.CiState
	b.Mutex.RUnlock()
	last := states[len(states)-1]
	if last != ciState {
		t.Fatalf("last delivered %q but session CiState %q (%d deliveries)", last, ciState, len(states))
	}
	for _, st := range states {
		switch st {
		case cmn.CIMasterStateString, cmn.CIBackupStateString, cmn.CIUnDefStateString:
		default:
			t.Fatalf("unexpected state %q", st)
		}
	}
}

// TestElectionLatestWins queues several decisions while the notifier is
// blocked and checks that the final role is delivered and the queueing
// path never blocks the caller.
func TestElectionLatestWins(t *testing.T) {
	b, n := newTestSession(t, 1000)

	// Block the notifier by holding the mutex it needs to read the mailbox.
	b.Mutex.Lock()
	b.RemDisc = 2000
	b.electLocked(BFDUp, BFDDown) // BACKUP
	b.electLocked(BFDDown, BFDUp) // MASTER
	b.electLocked(BFDUp, BFDDown) // BACKUP (latest)
	b.Mutex.Unlock()

	states := waitDrained(t, b, n)
	if len(states) != 1 || states[0] != cmn.CIBackupStateString {
		t.Fatalf("expected single coalesced BACKUP delivery, got %v", states)
	}
	if b.CiState != cmn.CIBackupStateString {
		t.Fatalf("CiState %q", b.CiState)
	}
}

// TestElectionSkipsWhenRemoteUnknown checks the RemDisc==0 guard.
func TestElectionSkipsWhenRemoteUnknown(t *testing.T) {
	b, n := newTestSession(t, 1000)
	b.Mutex.Lock()
	b.electLocked(BFDUp, BFDDown)
	b.Mutex.Unlock()
	if states := waitDrained(t, b, n); len(states) != 0 {
		t.Fatalf("expected no delivery, got %v", states)
	}
}
