// Command fakeext is a test extension: it speaks the line-JSON protocol and
// can answer, fail, hang or crash on demand.
package main

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return
		}
		switch req.Method {
		case "describe":
			_ = out.Encode(map[string]any{
				"id":     req.ID,
				"result": map[string]any{"name": "fake", "version": "1"},
			})
		case "fail":
			_ = out.Encode(map[string]any{"id": req.ID, "error": "no thanks"})
		case "slow":
			time.Sleep(2 * time.Second)
			_ = out.Encode(map[string]any{"id": req.ID, "result": "late"})
		case "crash":
			os.Exit(1)
		default:
			_ = out.Encode(map[string]any{
				"id":     req.ID,
				"result": map[string]any{"echo": req.Method},
			})
		}
	}
}
