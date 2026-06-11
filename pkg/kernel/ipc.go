package kernel

import (
	"fmt"
	"sync"
)

type Message struct {
	FromPID int
	ToPID   int
	Content string
}

type MessageBus struct {
	mu       sync.RWMutex
	channels map[int]chan Message
}

var GlobalMessageBus *MessageBus

func InitMessageBus() {
	GlobalMessageBus = &MessageBus{
		channels: make(map[int]chan Message),
	}
}

func (mb *MessageBus) Register(pid int) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.channels[pid] = make(chan Message, 100)
}

func (mb *MessageBus) Unregister(pid int) {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	if ch, ok := mb.channels[pid]; ok {
		close(ch)
		delete(mb.channels, pid)
	}
}

func (mb *MessageBus) Send(msg Message) error {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	ch, ok := mb.channels[msg.ToPID]
	if !ok {
		return fmt.Errorf("process %d not found or not registered for IPC", msg.ToPID)
	}

	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("message queue full for process %d", msg.ToPID)
	}
}

func (mb *MessageBus) Receive(pid int) []Message {
	mb.mu.RLock()
	ch, ok := mb.channels[pid]
	mb.mu.RUnlock()

	if !ok {
		return nil
	}

	var msgs []Message
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}
