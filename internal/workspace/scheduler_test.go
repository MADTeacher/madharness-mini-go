package workspace

import (
	"testing"
	"time"
)

func TestSchedulerAllowsDifferentWritePaths(t *testing.T) {
	s := NewScheduler()
	firstRelease := make(chan struct{})
	secondRelease := make(chan struct{})
	started := make(chan string, 2)
	done := make(chan string, 2)

	go lockedTask(s, "first", LockWrite, []string{"/repo/a.txt"}, started, firstRelease, done)
	waitLockStarted(t, started, "first")
	go lockedTask(s, "second", LockWrite, []string{"/repo/b.txt"}, started, secondRelease, done)
	waitLockStarted(t, started, "second")

	close(firstRelease)
	close(secondRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func TestSchedulerSerializesSameAndNestedPaths(t *testing.T) {
	for _, tc := range []struct {
		name       string
		firstPath  string
		secondPath string
	}{
		{name: "same", firstPath: "/repo/a.txt", secondPath: "/repo/a.txt"},
		{name: "nested", firstPath: "/repo/dir", secondPath: "/repo/dir/a.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewScheduler()
			firstRelease := make(chan struct{})
			secondRelease := make(chan struct{})
			started := make(chan string, 2)
			done := make(chan string, 2)

			go lockedTask(s, "first", LockWrite, []string{tc.firstPath}, started, firstRelease, done)
			waitLockStarted(t, started, "first")
			go lockedTask(s, "second", LockRead, []string{tc.secondPath}, started, secondRelease, done)
			assertNoLockStart(t, started)

			close(firstRelease)
			waitLockStarted(t, started, "second")
			close(secondRelease)
			waitLockDone(t, done)
			waitLockDone(t, done)
		})
	}
}

func TestSchedulerExclusiveLockBlocksPathLocks(t *testing.T) {
	s := NewScheduler()
	exclusiveRelease := make(chan struct{})
	readRelease := make(chan struct{})
	started := make(chan string, 2)
	done := make(chan string, 2)

	go lockedTask(s, "exclusive", LockExclusive, nil, started, exclusiveRelease, done)
	waitLockStarted(t, started, "exclusive")
	go lockedTask(s, "read", LockRead, []string{"/repo/a.txt"}, started, readRelease, done)
	assertNoLockStart(t, started)

	close(exclusiveRelease)
	waitLockStarted(t, started, "read")
	close(readRelease)
	waitLockDone(t, done)
	waitLockDone(t, done)
}

func lockedTask(
	s *Scheduler,
	name string,
	mode LockMode,
	paths []string,
	started chan<- string,
	release <-chan struct{},
	done chan<- string,
) {
	unlock := s.Acquire(mode, paths)
	started <- name
	<-release
	unlock()
	done <- name
}

func waitLockStarted(t *testing.T, started <-chan string, want string) {
	t.Helper()
	select {
	case name := <-started:
		if name != want {
			t.Fatalf("started %s, want %s", name, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s did not start", want)
	}
}

func assertNoLockStart(t *testing.T, started <-chan string) {
	t.Helper()
	select {
	case name := <-started:
		t.Fatalf("unexpected lock start: %s", name)
	case <-time.After(100 * time.Millisecond):
	}
}

func waitLockDone(t *testing.T, done <-chan string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lock holder did not finish")
	}
}
