package kernel

import (
	"fmt"
	"strings"
	"sync"
)

type ContextManager struct {
	mu      sync.Mutex
	agent   *AgentPCB
	history []string
}

func NewContextManager(agent *AgentPCB) *ContextManager {
	return &ContextManager{
		agent:   agent,
		history: make([]string, 0),
	}
}

func (cm *ContextManager) AppendHistory(msg string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.history = append(cm.history, msg)
}

func (cm *ContextManager) BuildNextPrompt(basePrompt string) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var sb strings.Builder
	
	if basePrompt != "" {
		sb.WriteString(basePrompt)
		sb.WriteString("\n")
	}

	if GlobalMessageBus != nil {
		msgs := GlobalMessageBus.Receive(cm.agent.PID)
		if len(msgs) > 0 {
			sb.WriteString("\n[SYSTEM] You received the following messages from other agents:\n")
			for _, msg := range msgs {
				sb.WriteString(fmt.Sprintf("From PID %d: %s\n", msg.FromPID, msg.Content))
			}
		}
	}

	finalPrompt := sb.String()
	cm.history = append(cm.history, finalPrompt)
	
	// Example hook for memory paging: if history > threshold, page out.
	if len(cm.history) > 1000 {
		// Paging logic would go here
	}

	return finalPrompt
}
