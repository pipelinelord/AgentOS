package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"agentos/pkg/drivers/io"
	"agentos/pkg/drivers/memory"
	"agentos/pkg/kernel"
	"agentos/pkg/llm"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const sysInstruction = `You are an AI agent running in AgentOS.
Available syscalls:
[SYS_CALL::EXEC_CMD] {"cmd": "<command>"}
[SYS_CALL::MEM_WRITE] {"key": "<label>", "value": "<text>"}
[SYS_CALL::MEM_READ] {"query": "<search query>"}
[SYS_CALL::SPAWN_AGENT] {"model": "gemini-1.5-flash", "role": "prompt", "permissions": ["SYS_WRITE_STDOUT"], "pipe_to": 3}
[SYS_CALL::SEND_MSG] {"to_pid": 2, "content": "hello"}
[SYS_CALL::RECV_MSG] {}
[SYS_CALL::SYS_WRITE_STDOUT] {"text": "hello"}
[SYS_CALL::SYS_READ_STDIN] {}
[SYS_CALL::SYS_SCHEDULE] {"seconds": 10, "prompt": "Wake up"}
[SYS_CALL::SYS_REGISTER_WEBHOOK] {"path": "/api/chat"}
[SYS_CALL::SYS_WEBHOOK_REPLY] {"request_id": "123", "status": 200, "body": "OK"}
[SYS_CALL::SLEEP] {"seconds": 5}
[SYS_CALL::FS_READ] {"path": "/test.txt"}
[SYS_CALL::FS_WRITE] {"path": "/test.txt", "content": "hello"}
[SYS_CALL::NET_FETCH] {"url": "https://example.com"}
[SYS_CALL::SYS_EXIT] {}

Respond with one syscall at a time to explore the system. Always output the JSON immediately following the syscall tag.`

var (
	clientMap      = make(map[int]*llm.Client)
	clientMapMutex sync.Mutex
)

func main() {
	_ = godotenv.Load() // Load .env if it exists

	kernel.InitProcessManager()
	kernel.InitMessageBus()
	kernel.InitTimerManager()
	kernel.InitWebhookManager(8088)

	// Initialize drivers with fallbacks
	var execDriver kernel.ExecDriver
	var memDriver kernel.MemoryDriver

	docker, err := io.NewDockerDriver()
	if err != nil {
		fmt.Printf("Docker driver init failed (%v), falling back to LocalExecDriver\n", err)
		execDriver = io.NewLocalExecDriver()
	} else {
		execDriver = docker
	}

	chroma, err := memory.NewChromaDriver(context.Background(), "agentos_memory")
	if err != nil {
		fmt.Printf("ChromaDB driver init failed (%v), falling back to LocalMemoryDriver\n", err)
		memDriver = memory.NewLocalMemoryDriver()
	} else {
		memDriver = chroma
	}

	fsDriver, err := io.NewLocalFSDriver("d:/AgentOS/workspace")
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize FSDriver: %v", err))
	}
	netDriver := io.NewLocalNetDriver()

	kernel.InitDispatcher(execDriver, memDriver, fsDriver, netDriver)

	kernel.InitScheduler(10, agentTick)
	kernel.GlobalScheduler.Start()

	var rootCmd = &cobra.Command{
		Use:   "aos",
		Short: "AgentOS - Proof of Concept",
	}

	var model, role string

	var spawnCmd = &cobra.Command{
		Use:   "spawn",
		Short: "Spawns an agent instance",
		Run: func(cmd *cobra.Command, args []string) {
			agent := kernel.GlobalProcessManager.Spawn(model, role, nil)
			fmt.Printf("Spawned Agent. PID: %d, UUID: %s\n", agent.PID, agent.UUID)

			ctx := context.Background()
			llmClient, err := llm.NewClient(ctx, agent.Model, agent.Role+"\n\n"+sysInstruction)
			if err != nil {
				agent.Log(fmt.Sprintf("Failed to initialize LLM: %v", err))
				_ = kernel.GlobalProcessManager.Kill(agent.PID)
				return
			}
			
			clientMapMutex.Lock()
			clientMap[agent.PID] = llmClient
			clientMapMutex.Unlock()

			agent.Log("Agent spawned.")
			
			// We inject the first prompt into context history
			agent.Context().AppendHistory("Start your execution. You are in a sandboxed environment.")
			
			kernel.GlobalScheduler.Schedule(agent)

			// Stream logs to console
			go func() {
				for msg := range agent.LogChan {
					fmt.Println(msg)
				}
			}()

			// Block until interrupt
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
			fmt.Println("AgentOS running. Press Ctrl+C to exit.")
			<-sigChan
			fmt.Println("\nShutting down AgentOS...")
		},
	}
	spawnCmd.Flags().StringVar(&model, "model", "gemini-3.5-flash", "LLM Engine model ID")
	spawnCmd.Flags().StringVar(&role, "role", "You are an AI developer in AgentOS.", "Primary operational profile prompt")

	var psCmd = &cobra.Command{
		Use:   "ps",
		Short: "Iterates over the running thread collection",
		Run: func(cmd *cobra.Command, args []string) {
			agents := kernel.GlobalProcessManager.List()
			fmt.Printf("%-5s %-36s %-10s %-20s %-10s\n", "PID", "UUID", "STATUS", "MODEL", "TOKENS")
			for _, a := range agents {
				fmt.Printf("%-5d %-36s %-10s %-20s %-10d\n", a.PID, a.UUID, a.Status, a.Model, a.TokenCount)
			}
		},
	}

	var topCmd = &cobra.Command{
		Use:   "top",
		Short: "Renders real-time telemetry",
		Run: func(cmd *cobra.Command, args []string) {
			agents := kernel.GlobalProcessManager.List()
			fmt.Println("--- AgentOS TOP ---")
			for _, a := range agents {
				fmt.Printf("PID: %d | Status: %s | Syscalls: %d | Tokens: %d\n", a.PID, a.Status, a.SyscallCount, a.TokenCount)
			}
		},
	}

	var killCmd = &cobra.Command{
		Use:   "kill [pid]",
		Short: "Sends a structural abort sequence",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pid := 0
			fmt.Sscanf(args[0], "%d", &pid)
			err := kernel.GlobalProcessManager.Kill(pid)
			if err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Printf("Killed agent %d\n", pid)
			}
		},
	}

	var logsCmd = &cobra.Command{
		Use:   "logs [pid]",
		Short: "Streams logs for an agent",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pid := 0
			fmt.Sscanf(args[0], "%d", &pid)
			agent, ok := kernel.GlobalProcessManager.Get(pid)
			if !ok {
				fmt.Printf("Agent %d not found\n", pid)
				return
			}
			fmt.Printf("--- Streaming logs for Agent %d ---\n", pid)
			for msg := range agent.LogChan {
				fmt.Println(msg)
			}
		},
	}

	rootCmd.AddCommand(spawnCmd, psCmd, topCmd, killCmd, logsCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func agentTick(ctx context.Context, agent *kernel.AgentPCB) {
	clientMapMutex.Lock()
	llmClient, ok := clientMap[agent.PID]
	clientMapMutex.Unlock()

	if !ok {
		// This handles the SPAWN_AGENT case where the scheduler picks up the child
		// before we created its llm client. So we lazily initialize it if missing.
		var err error
		llmClient, err = llm.NewClient(ctx, agent.Model, agent.Role+"\n\n"+sysInstruction)
		if err != nil {
			agent.Log(fmt.Sprintf("Failed to init LLM for PID %d: %v", agent.PID, err))
			_ = kernel.GlobalProcessManager.Kill(agent.PID)
			return
		}
		clientMapMutex.Lock()
		clientMap[agent.PID] = llmClient
		clientMapMutex.Unlock()
		agent.Context().AppendHistory("You were spawned by another agent. Start your execution.")
	}

	// We pop the first history item to send as prompt, or empty string if none.
	// ContextManager actually keeps full history, but we don't want to re-send everything
	// since llm.Client maintains a ChatSession.
	// So we need to pull just the new messages (IPC, Syscall results).
	// We modify ContextManager's BuildNextPrompt to accept a base.
	prompt := agent.Context().BuildNextPrompt("Continue.")

	agent.Log(fmt.Sprintf("\n>>> PROMPT:\n%s", prompt))

	resp, err := llmClient.SendMessage(ctx, prompt)
	if err != nil {
		agent.Log(fmt.Sprintf("LLM Error: %v", err))
		agent.Context().AppendHistory(fmt.Sprintf("Error interacting with LLM: %v", err))
		
		// Prevent instant retry spam loop if API returns a fast error (like 404)
		agent.Status = kernel.StatusSleeping
		go func() {
			time.Sleep(5 * time.Second)
			if kernel.GlobalScheduler != nil {
				kernel.GlobalScheduler.Schedule(agent)
			}
		}()
		return
	}

	agent.TokenCount += int64(len(resp) / 4)
	agent.Log(fmt.Sprintf("\n<<< LLM RESPONSE:\n%s", resp))

	syscalls, err := kernel.ParseSyscalls(resp)
	if err != nil || len(syscalls) == 0 {
		agent.Context().AppendHistory("No valid syscall detected. Please output JSON after the syscall tag.")
		return
	}

	agent.SyscallCount++
	sc := syscalls[0]

	agent.Log(fmt.Sprintf("Executing %s", sc.Name))
	sysResp := kernel.GlobalDispatcher.Dispatch(ctx, agent, sc)

	sysRespJSON, _ := json.Marshal(sysResp)
	agent.Context().AppendHistory(fmt.Sprintf("Syscall %s returned:\n%s", sc.Name, string(sysRespJSON)))
}
