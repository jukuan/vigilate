package main

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// capturedCall records a single script invocation
type capturedCall struct {
	scriptPath string
	message    string
}

func TestAlertManager_AccumulateAndFlush(t *testing.T) {
	am := NewAlertManager()

	// Use a mock runner that captures calls
	var mu sync.Mutex
	var calls []capturedCall
	am.SetRunner(func(script, msg string) {
		mu.Lock()
		calls = append(calls, capturedCall{script, msg})
		mu.Unlock()
	})

	ruleName := "test-rule"
	actions := []string{"script1.sh", "script2.sh"}
	cooldown := 100 * time.Millisecond // short for test

	// First match
	am.AddMatch(MatchedLine{RuleName: ruleName, FilePath: "test.log", Line: "FATAL: disk full"}, cooldown, actions)
	// Second match (same window)
	time.Sleep(10 * time.Millisecond)
	am.AddMatch(MatchedLine{RuleName: ruleName, FilePath: "test.log", Line: "ERROR: timeout"}, cooldown, actions)

	// Wait for cooldown to expire + a bit for flush
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(calls) != 2 { // two scripts per flush
		t.Fatalf("expected 2 script calls, got %d", len(calls))
	}

	// Both calls should have the same message
	msg := calls[0].message
	if msg != calls[1].message {
		t.Errorf("expected same message for both scripts, got %q and %q", calls[0].message, calls[1].message)
	}

	expectedMsg := fmt.Sprintf("ALERT: [test-rule] test.log: 2 lines in logs for last 1 minutes with like %s", truncateLogStr("FATAL: disk full", 16))
	if msg != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, msg)
	}
}

func TestAlertManager_CooldownResets(t *testing.T) {
	am := NewAlertManager()
	var mu sync.Mutex
	var calls []capturedCall
	am.SetRunner(func(script, msg string) {
		mu.Lock()
		calls = append(calls, capturedCall{script, msg})
		mu.Unlock()
	})

	ruleName := "test"
	actions := []string{"script.sh"}
	cooldown := 80 * time.Millisecond

	// First window
	am.AddMatch(MatchedLine{RuleName: ruleName, FilePath: "f", Line: "ERROR: A"}, cooldown, actions)
	time.Sleep(100 * time.Millisecond) // let first flush fire

	// Second window (should be new window, count = 1)
	am.AddMatch(MatchedLine{RuleName: ruleName, FilePath: "f", Line: "ERROR: B"}, cooldown, actions)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(calls) != 2 { // one flush of the first, one flush of the second
		t.Fatalf("expected 2 flushes (2 script calls), got %d", len(calls))
	}

	// First flush message
	expected1 := fmt.Sprintf("ALERT: [test] f: 1 lines in logs for last 1 minutes with like %s", truncateLogStr("ERROR: A", 16))
	if calls[0].message != expected1 {
		t.Errorf("first flush: expected %q, got %q", expected1, calls[0].message)
	}
	// Second flush message
	expected2 := fmt.Sprintf("ALERT: [test] f: 1 lines in logs for last 1 minutes with like %s", truncateLogStr("ERROR: B", 16))
	if calls[1].message != expected2 {
		t.Errorf("second flush: expected %q, got %q", expected2, calls[1].message)
	}
}

func TestAlertManager_FlushAll(t *testing.T) {
	am := NewAlertManager()
	var mu sync.Mutex
	var calls []capturedCall
	am.SetRunner(func(script, msg string) {
		mu.Lock()
		calls = append(calls, capturedCall{script, msg})
		mu.Unlock()
	})

	am.AddMatch(MatchedLine{RuleName: "r1", FilePath: "f", Line: "WARN: x"}, 10*time.Second, []string{"a.sh"})
	am.AddMatch(MatchedLine{RuleName: "r2", FilePath: "f", Line: "ERR: y"}, 10*time.Second, []string{"b.sh"})

	// Immediate flush
	am.FlushAll()
	time.Sleep(50 * time.Millisecond) // let goroutines deliver

	mu.Lock()
	defer mu.Unlock()

	if len(calls) != 2 {
		t.Fatalf("expected 2 script calls, got %d", len(calls))
	}
}
