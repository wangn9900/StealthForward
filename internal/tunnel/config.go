package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Task struct {
	ID         uint   `json:"id"`
	Mode       string `json:"mode"` // transit or exit
	ListenAddr string `json:"listen_addr"`
	TargetAddr string `json:"target_addr"` // transit: "exit_ip:port", exit: "127.0.0.1:port"
	Key        string `json:"key"`
}

type TunnelConfig struct {
	Tasks []Task `json:"tasks"`
}

func RunStandalone(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var cfg TunnelConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	ctx := context.Background()
	errChan := make(chan error, len(cfg.Tasks))

	for _, task := range cfg.Tasks {
		task := task // capture range variable
		if task.Mode == "transit" {
			server := &TransitServer{
				RuleID:     task.ID,
				ListenAddr: task.ListenAddr,
				TargetAddr: task.TargetAddr,
				Key:        task.Key,
				Counter:    &TrafficCounter{},
			}
			log.Printf("[Agent] Starting Transit: %s -> %s", task.ListenAddr, task.TargetAddr)
			go func() {
				errChan <- server.Start(ctx)
			}()
		} else if task.Mode == "exit" {
			server := &ExitServer{
				ListenAddr: task.ListenAddr,
				LocalAddr:  task.TargetAddr,
				Key:        task.Key,
			}
			log.Printf("[Agent] Starting Exit listening on %s", task.ListenAddr)
			go func() {
				errChan <- server.Start(ctx)
			}()
		}
	}

	if len(cfg.Tasks) == 0 {
		return fmt.Errorf("no tasks found in config")
	}

	// Wait for the first error or keep running
	return <-errChan
}
