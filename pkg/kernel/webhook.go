package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type WebhookRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type WebhookResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

type WebhookManager struct {
	mu         sync.RWMutex
	routes     map[string]int // path -> PID
	pendingReq map[string]chan WebhookResponse
}

var GlobalWebhookManager *WebhookManager

func InitWebhookManager(port int) {
	GlobalWebhookManager = &WebhookManager{
		routes:     make(map[string]int),
		pendingReq: make(map[string]chan WebhookResponse),
	}
	
	http.HandleFunc("/", GlobalWebhookManager.handleRequest)
	go func() {
		fmt.Printf("Webhook server listening on :%d\n", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			fmt.Printf("Webhook server error: %v\n", err)
		}
	}()
}

func (wm *WebhookManager) Register(path string, pid int) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.routes[path] = pid
}

func (wm *WebhookManager) Reply(reqID string, resp WebhookResponse) error {
	wm.mu.RLock()
	ch, ok := wm.pendingReq[reqID]
	wm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("request id not found or already replied")
	}
	ch <- resp
	
	wm.mu.Lock()
	delete(wm.pendingReq, reqID)
	wm.mu.Unlock()
	
	return nil
}

func (wm *WebhookManager) handleRequest(w http.ResponseWriter, r *http.Request) {
	wm.mu.RLock()
	pid, ok := wm.routes[r.URL.Path]
	wm.mu.RUnlock()

	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	bodyBytes, _ := io.ReadAll(r.Body)
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	
	webhookReq := WebhookRequest{
		ID:      reqID,
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: headers,
		Body:    string(bodyBytes),
	}

	reqJSON, _ := json.Marshal(webhookReq)
	
	if agent, ok := GlobalProcessManager.Get(pid); ok {
		agent.Context().AppendHistory(fmt.Sprintf("WEBHOOK HIT:\n%s", string(reqJSON)))
		if agent.Status == StatusSleeping || agent.Status == StatusBlocked {
			agent.Status = StatusRunning
		}
		if GlobalScheduler != nil {
			GlobalScheduler.Schedule(agent)
		}
	} else {
		http.Error(w, "Agent terminated", http.StatusServiceUnavailable)
		return
	}

	ch := make(chan WebhookResponse, 1)
	wm.mu.Lock()
	wm.pendingReq[reqID] = ch
	wm.mu.Unlock()

	select {
	case resp := <-ch:
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	case <-time.After(30 * time.Second):
		wm.mu.Lock()
		delete(wm.pendingReq, reqID)
		wm.mu.Unlock()
		http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
	}
}
