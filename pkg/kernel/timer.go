package kernel

import (
	"fmt"
	"sync"
	"time"
)

type ScheduledTask struct {
	ExecuteAt time.Time
	Prompt    string
	AgentPID  int
}

type TimerManager struct {
	mu    sync.Mutex
	tasks []ScheduledTask
}

var GlobalTimerManager *TimerManager

func InitTimerManager() {
	GlobalTimerManager = &TimerManager{
		tasks: make([]ScheduledTask, 0),
	}
	go GlobalTimerManager.Run()
}

func (tm *TimerManager) Schedule(pid int, seconds int, prompt string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tasks = append(tm.tasks, ScheduledTask{
		ExecuteAt: time.Now().Add(time.Duration(seconds) * time.Second),
		Prompt:    prompt,
		AgentPID:  pid,
	})
}

func (tm *TimerManager) Run() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		tm.mu.Lock()
		now := time.Now()
		var remaining []ScheduledTask
		for _, task := range tm.tasks {
			if now.After(task.ExecuteAt) || now.Equal(task.ExecuteAt) {
				if agent, ok := GlobalProcessManager.Get(task.AgentPID); ok {
					agent.Context().AppendHistory(fmt.Sprintf("TIMER FIRED: %s", task.Prompt))
					if agent.Status == StatusSleeping || agent.Status == StatusBlocked {
						agent.Status = StatusRunning
					}
					if GlobalScheduler != nil {
						GlobalScheduler.Schedule(agent)
					}
				}
			} else {
				remaining = append(remaining, task)
			}
		}
		tm.tasks = remaining
		tm.mu.Unlock()
	}
}
