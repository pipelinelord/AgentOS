package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Scheduler struct {
	mu           sync.Mutex
	readyQueue   chan *AgentPCB
	workerCount  int
	tickCallback func(ctx context.Context, agent *AgentPCB)
}

var GlobalScheduler *Scheduler

func InitScheduler(workers int, tick func(ctx context.Context, agent *AgentPCB)) {
	GlobalScheduler = &Scheduler{
		readyQueue:   make(chan *AgentPCB, 1000), // Buffer for ready queue
		workerCount:  workers,
		tickCallback: tick,
	}
}

func (s *Scheduler) Start() {
	for i := 0; i < s.workerCount; i++ {
		go s.worker(i)
	}
}

func (s *Scheduler) worker(id int) {
	for agent := range s.readyQueue {
		// If agent was killed, ignore it
		if agent.Status == StatusTerminated {
			continue
		}

		// Agent is now running on a processor
		agent.Status = StatusRunning

		// Provide a short timeout for the tick, enforcing time slicing (e.g., 30 seconds max per tick)
		tickCtx, cancel := context.WithTimeout(agent.ctx, 30*time.Second)
		s.tickCallback(tickCtx, agent)
		cancel()

		// If still running/blocked, re-schedule. If sleeping, the sleep syscall handles re-scheduling.
		if agent.Status == StatusRunning || agent.Status == StatusBlocked {
			// Back to ready
			s.Schedule(agent)
		}
	}
}

func (s *Scheduler) Schedule(agent *AgentPCB) {
	if agent.Status == StatusTerminated {
		return
	}
	
	// Prevent blocking the caller if the queue is full, though with 1000 buffer it's unlikely
	select {
	case s.readyQueue <- agent:
		// scheduled successfully
	default:
		fmt.Printf("WARNING: Scheduler queue full, dropping schedule for PID %d\n", agent.PID)
	}
}
