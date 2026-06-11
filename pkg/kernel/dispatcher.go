package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type ExecDriver interface {
	Exec(ctx context.Context, cmd string) (string, string, int, error)
}

type MemoryDriver interface {
	Write(ctx context.Context, key, val string) error
	Read(ctx context.Context, query string) ([]string, error)
}

type FileSystemDriver interface {
	Read(ctx context.Context, path string) (string, error)
	Write(ctx context.Context, path, content string) error
}

type NetworkDriver interface {
	Fetch(ctx context.Context, url string) (string, error)
}

type SyscallDispatcher struct {
	execDriver ExecDriver
	memDriver  MemoryDriver
	fsDriver   FileSystemDriver
	netDriver  NetworkDriver
}

var GlobalDispatcher *SyscallDispatcher

func InitDispatcher(exec ExecDriver, mem MemoryDriver, fs FileSystemDriver, net NetworkDriver) {
	GlobalDispatcher = &SyscallDispatcher{
		execDriver: exec,
		memDriver:  mem,
		fsDriver:   fs,
		netDriver:  net,
	}
}

func (d *SyscallDispatcher) Dispatch(ctx context.Context, agent *AgentPCB, req SyscallRequest) SyscallResponse {
	if !agent.HasPermission(req.Name) {
		return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("permission denied for syscall: %s", req.Name)}
	}

	switch req.Name {
	case "EXEC_CMD":
		if d.execDriver == nil {
			return SyscallResponse{Status: "ERROR", Error: "EXEC_CMD driver not available"}
		}
		var args struct {
			Cmd string `json:"cmd"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		
		stdout, stderr, exitCode, err := d.execDriver.Exec(ctx, args.Cmd)
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS", Result: map[string]interface{}{
			"stdout":   stdout,
			"stderr":   stderr,
			"exitCode": exitCode,
		}}

	case "MEM_WRITE":
		if d.memDriver == nil {
			return SyscallResponse{Status: "ERROR", Error: "MEM_WRITE driver not available"}
		}
		var args struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		
		if err := d.memDriver.Write(ctx, args.Key, args.Value); err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS"}

	case "MEM_READ":
		if d.memDriver == nil {
			return SyscallResponse{Status: "ERROR", Error: "MEM_READ driver not available"}
		}
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		
		results, err := d.memDriver.Read(ctx, args.Query)
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS", Result: results}

	case "SPAWN_AGENT":
		var args struct {
			Model       string   `json:"model"`
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
			PipeToPID   int      `json:"pipe_to"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if args.Model == "" {
			args.Model = agent.Model
		}
		child := GlobalProcessManager.Spawn(args.Model, args.Role, args.Permissions)
		child.ParentPID = agent.PID
		child.PipeToPID = args.PipeToPID
		agent.ChildPIDs = append(agent.ChildPIDs, child.PID)
		
		if GlobalScheduler != nil {
			GlobalScheduler.Schedule(child)
		}
		return SyscallResponse{Status: "SUCCESS", Result: map[string]interface{}{"pid": child.PID}}

	case "SEND_MSG":
		var args struct {
			ToPID   int    `json:"to_pid"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if GlobalMessageBus == nil {
			return SyscallResponse{Status: "ERROR", Error: "IPC MessageBus not initialized"}
		}
		err := GlobalMessageBus.Send(Message{
			FromPID: agent.PID,
			ToPID:   args.ToPID,
			Content: args.Content,
		})
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS"}

	case "SLEEP":
		var args struct {
			Seconds int `json:"seconds"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		
		agent.Status = StatusSleeping
		go func() {
			time.Sleep(time.Duration(args.Seconds) * time.Second)
			if GlobalScheduler != nil {
				GlobalScheduler.Schedule(agent)
			}
		}()
		return SyscallResponse{Status: "SUCCESS", Result: fmt.Sprintf("Slept for %d seconds", args.Seconds)}

	case "SYS_EXIT":
		err := GlobalProcessManager.Kill(agent.PID)
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS", Result: "Agent Terminated"}

	case "RECV_MSG":
		if GlobalMessageBus == nil {
			return SyscallResponse{Status: "ERROR", Error: "IPC MessageBus not initialized"}
		}
		msgs := GlobalMessageBus.Receive(agent.PID)
		return SyscallResponse{Status: "SUCCESS", Result: msgs}

	case "FS_READ":
		if d.fsDriver == nil {
			return SyscallResponse{Status: "ERROR", Error: "FS_READ driver not available"}
		}
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		content, err := d.fsDriver.Read(ctx, args.Path)
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS", Result: content}

	case "FS_WRITE":
		if d.fsDriver == nil {
			return SyscallResponse{Status: "ERROR", Error: "FS_WRITE driver not available"}
		}
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if err := d.fsDriver.Write(ctx, args.Path, args.Content); err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS"}

	case "NET_FETCH":
		if d.netDriver == nil {
			return SyscallResponse{Status: "ERROR", Error: "NET_FETCH driver not available"}
		}
		var args struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		content, err := d.netDriver.Fetch(ctx, args.URL)
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS", Result: content}

	case "SYS_WRITE_STDOUT":
		var args struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if agent.PipeToPID > 0 {
			if target, ok := GlobalProcessManager.Get(agent.PipeToPID); ok {
				select {
				case target.Stdin <- args.Text:
				default:
					return SyscallResponse{Status: "ERROR", Error: "target STDIN buffer full"}
				}
				return SyscallResponse{Status: "SUCCESS", Result: "piped"}
			}
			return SyscallResponse{Status: "ERROR", Error: "target pipe process not found"}
		}
		fmt.Printf("[STDOUT PID %d] %s\n", agent.PID, args.Text)
		return SyscallResponse{Status: "SUCCESS"}

	case "SYS_READ_STDIN":
		select {
		case data := <-agent.Stdin:
			return SyscallResponse{Status: "SUCCESS", Result: data}
		default:
			return SyscallResponse{Status: "SUCCESS", Result: ""}
		}

	case "SYS_SCHEDULE":
		var args struct {
			Seconds int    `json:"seconds"`
			Prompt  string `json:"prompt"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if GlobalTimerManager == nil {
			return SyscallResponse{Status: "ERROR", Error: "TimerManager not initialized"}
		}
		GlobalTimerManager.Schedule(agent.PID, args.Seconds, args.Prompt)
		return SyscallResponse{Status: "SUCCESS"}

	case "SYS_REGISTER_WEBHOOK":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if GlobalWebhookManager == nil {
			return SyscallResponse{Status: "ERROR", Error: "WebhookManager not initialized"}
		}
		GlobalWebhookManager.Register(args.Path, agent.PID)
		return SyscallResponse{Status: "SUCCESS"}

	case "SYS_WEBHOOK_REPLY":
		var args struct {
			RequestID string `json:"request_id"`
			Status    int    `json:"status"`
			Body      string `json:"body"`
		}
		if err := json.Unmarshal(req.Args, &args); err != nil {
			return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("invalid args: %v", err)}
		}
		if GlobalWebhookManager == nil {
			return SyscallResponse{Status: "ERROR", Error: "WebhookManager not initialized"}
		}
		err := GlobalWebhookManager.Reply(args.RequestID, WebhookResponse{Status: args.Status, Body: args.Body})
		if err != nil {
			return SyscallResponse{Status: "ERROR", Error: err.Error()}
		}
		return SyscallResponse{Status: "SUCCESS"}

	default:
		return SyscallResponse{Status: "ERROR", Error: fmt.Sprintf("unsupported syscall: %s", req.Name)}
	}
}
