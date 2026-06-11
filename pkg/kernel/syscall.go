package kernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

type SyscallRequest struct {
	Name string
	Args json.RawMessage
}

type SyscallResponse struct {
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

var syscallRegex = regexp.MustCompile(`\[SYS_CALL::([A-Z_]+)\]`)

func ParseSyscalls(text string) ([]SyscallRequest, error) {
	var reqs []SyscallRequest
	matches := syscallRegex.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		name := text[match[2]:match[3]]
		remainder := text[match[1]:]

		// Extract json part. Since there might be markdown or other text,
		// we find the first '{' after the syscall marker.
		braceIdx := bytes.IndexByte([]byte(remainder), '{')
		if braceIdx == -1 {
			return nil, fmt.Errorf("no JSON object found for syscall %s", name)
		}
		jsonStart := remainder[braceIdx:]

		// Use json.Decoder to parse the first valid JSON object
		dec := json.NewDecoder(bytes.NewBufferString(jsonStart))
		var raw json.RawMessage
		err := dec.Decode(&raw)
		if err != nil {
			return nil, fmt.Errorf("Syscall Syntax Error for %s: expected valid JSON but got parsing error: %v. Please correct and retry.", name, err)
		}

		reqs = append(reqs, SyscallRequest{
			Name: name,
			Args: raw,
		})
	}

	return reqs, nil
}
