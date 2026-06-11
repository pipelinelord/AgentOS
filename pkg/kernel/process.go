package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ProcessStatus string

const (
	StatusRunning    ProcessStatus = "RUNNING"
	StatusBlocked    ProcessStatus = "BLOCKED"
	StatusSleeping   ProcessStatus = "SLEEPING"
	StatusTerminated ProcessStatus = "TERMINATED"
)

type AgentPCB struct {
	PID          int
	ParentPID    int
	ChildPIDs    []int
	UUID         string
	Status       ProcessStatus
	Model        string
	Role         string
	Permissions  []string
	Stdin        chan string
	Stdout       chan string
	PipeToPID    int
	TokenCount   int64
	ContextFile  string
	KillChan     chan bool
	LogChan      chan string
	ctx          context.Context
	cancel       context.CancelFunc
	SyscallCount int
	StartTime    time.Time
	contextMgr   *ContextManager
}

type ProcessManager struct {
	mu      sync.RWMutex
	agents  map[int]*AgentPCB
	nextPID int
}

var GlobalProcessManager *ProcessManager

func InitProcessManager() {
	GlobalProcessManager = &ProcessManager{
		agents:  make(map[int]*AgentPCB),
		nextPID: 1,
	}
}

func (pm *ProcessManager) Spawn(model string, role string, permissions []string) *AgentPCB {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pid := pm.nextPID
	pm.nextPID++

	ctx, cancel := context.WithCancel(context.Background())

	pcb := &AgentPCB{
		PID:          pid,
		ParentPID:    0,
		ChildPIDs:    []int{},
		UUID:         uuid.New().String(),
		Status:       StatusRunning,
		Model:        model,
		Role:         role,
		Permissions:  permissions,
		Stdin:        make(chan string, 100),
		Stdout:       make(chan string, 100),
		PipeToPID:    0,
		TokenCount:   0,
		ContextFile:  fmt.Sprintf("/tmp/agentos_%d.ctx", pid), // Placeholder
		KillChan:     make(chan bool, 1),
		LogChan:      make(chan string, 100),
		ctx:          ctx,
		cancel:       cancel,
		SyscallCount: 0,
		StartTime:    time.Now(),
	}
	pcb.contextMgr = NewContextManager(pcb)

	pm.agents[pid] = pcb

	return pcb
}

func (pm *ProcessManager) Get(pid int) (*AgentPCB, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	agent, ok := pm.agents[pid]
	return agent, ok
}

func (pm *ProcessManager) Kill(pid int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	agent, ok := pm.agents[pid]
	if !ok {
		return fmt.Errorf("process %d not found", pid)
	}

	if agent.Status == StatusTerminated {
		return nil
	}

	agent.Status = StatusTerminated
	agent.cancel()
	agent.KillChan <- true
	close(agent.LogChan)

	return nil
}

func (pm *ProcessManager) List() []*AgentPCB {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	list := make([]*AgentPCB, 0, len(pm.agents))
	for _, pcb := range pm.agents {
		list = append(list, pcb)
	}
	return list
}

func (pcb *AgentPCB) Log(msg string) {
	select {
	case pcb.LogChan <- msg:
	default:
		// Channel is full, drop or handle
	}
}

func (pcb *AgentPCB) Context() *ContextManager {
	return pcb.contextMgr
}

func (pcb *AgentPCB) Ctx() context.Context {
	return pcb.ctx
}

func (pcb *AgentPCB) HasPermission(syscall string) bool {
	if len(pcb.Permissions) == 0 {
		return true // Root level by default
	}
	for _, p := range pcb.Permissions {
		if p == syscall {
			return true
		}
	}
	return false
}
