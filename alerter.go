package main

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

type ScriptRunner func(scriptPath, message string)

func defaultRunner(scriptPath, message string) {
	cmd := exec.Command("bash", scriptPath, message)
	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Printf("Script %s failed: %v (output: %s)", scriptPath, err, string(output))
	} else {
		log.Printf("Script %s executed successfully", scriptPath)
	}
}

type MatchedLine struct {
	RuleName string
	FilePath string
	Line     string
}

type AlertWindow struct {
	mu          sync.Mutex
	ruleName    string
	count       int
	firstLine   string
	firstFile   string
	windowStart time.Time
	cooldown    time.Duration
	actions     []string
	runner      ScriptRunner
	timer       *time.Timer
	flushChan   chan struct{}
}

type AlertManager struct {
	mu      sync.Mutex
	windows map[string]*AlertWindow
	runner  ScriptRunner
}

func NewAlertManager() *AlertManager {
	return &AlertManager{
		windows: make(map[string]*AlertWindow),
		runner:  defaultRunner, // use real bash by default
	}
}

// SetRunner overrides the script runner for all windows (used in tests).
func (am *AlertManager) SetRunner(r ScriptRunner) {
	am.runner = r
}

func (am *AlertManager) AddMatch(line MatchedLine, cooldown time.Duration, actions []string) {
	am.mu.Lock()
	window, exists := am.windows[line.RuleName]
	if !exists {
		window = &AlertWindow{
			ruleName:    line.RuleName,
			cooldown:    cooldown,
			actions:     actions,
			runner:      am.runner,
			windowStart: time.Now(),
			flushChan:   make(chan struct{}),
		}
		window.timer = time.AfterFunc(cooldown, func() {
			window.flushChan <- struct{}{}
		})
		am.windows[line.RuleName] = window
		go window.runFlusher()
	}
	am.mu.Unlock()

	window.mu.Lock()
	defer window.mu.Unlock()

	window.count++
	if window.count == 1 {
		window.firstLine = line.Line
		window.firstFile = line.FilePath
	}
}

func (aw *AlertWindow) runFlusher() {
	for range aw.flushChan {
		aw.flush()
	}
}

func (aw *AlertWindow) flush() {
	aw.mu.Lock()
	count := aw.count
	firstLine := aw.firstLine
	firstFile := aw.firstFile
	windowStart := aw.windowStart
	aw.mu.Unlock()

	if count == 0 {
		aw.timer.Reset(aw.cooldown)
		return
	}

	minutes := max(int(time.Since(windowStart).Minutes()), 1)

	message := fmt.Sprintf(
		"ALERT: [%s] %s: %d lines in logs for last %d minutes with like %s",
		aw.ruleName, firstFile, count, minutes, firstLine)

	log.Printf("[%s] Flushing alert: %s", aw.ruleName, message)

	for _, script := range aw.actions {
		go aw.executeScript(script, message)
	}

	aw.mu.Lock()
	aw.count = 0
	aw.firstLine = ""
	aw.firstFile = ""
	aw.windowStart = time.Now()
	aw.timer.Reset(aw.cooldown)
	aw.mu.Unlock()
}

func (aw *AlertWindow) executeScript(scriptPath string, message string) {
	aw.runner(scriptPath, message)
}

func (am *AlertManager) FlushAll() {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, window := range am.windows {
		window.flush()
	}
}
