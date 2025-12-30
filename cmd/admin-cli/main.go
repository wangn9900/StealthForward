package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
)

var controllerAddr string

func main() {
	flag.StringVar(&controllerAddr, "api", "http://127.0.0.1:8080", "Controller API address")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	switch command {
	case "add-exit":
		addExit(args[1:])
	case "set-forward":
		setForward(args[1:])
	case "list-entries":
		listEntries()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("StealthForward Admin CLI")
	fmt.Println("Usage:")
	fmt.Println("  admin-cli add-exit <name> <protocol> <config_json>")
	fmt.Println("  admin-cli set-forward <user_email> <entry_id> <exit_id> <uuid>")
	fmt.Println("  admin-cli list-entries")
}

func addExit(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: add-exit <name> <protocol> <config_json>")
		return
	}
	name, proto, config := args[0], args[1], args[2]
	payload := map[string]interface{}{
		"name":     name,
		"protocol": proto,
		"config":   config,
	}
	post("/api/v1/exits", payload)
}

func setForward(args []string) {
	if len(args) < 4 {
		fmt.Println("Usage: set-forward <user_email> <entry_id> <exit_id> <uuid>")
		return
	}
	email, entryID, exitID, uuid := args[0], args[1], args[2], args[3]

	// 这里简单处�?ID 类型转换
	var eid, exid int
	fmt.Sscanf(entryID, "%d", &eid)
	fmt.Sscanf(exitID, "%d", &exid)

	payload := map[string]interface{}{
		"user_email":    email,
		"user_id":       uuid,
		"entry_node_id": eid,
		"exit_node_id":  exid,
	}
	post("/api/v1/rules", payload)
}

func listEntries() {
	resp, err := http.Get(controllerAddr + "/api/v1/entries")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func post(path string, payload interface{}) {
	jsonBody, _ := json.Marshal(payload)
	resp, err := http.Post(controllerAddr+path, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", string(body))
}
