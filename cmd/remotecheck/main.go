// Command remotecheck sends a task to tarkov.dev Remote Control with the
// Remote ID from MAYAK' settings (the first target for tasks, else the
// main ID), to check the connection.
//
//	go run ./cmd/remotecheck <task-slug>
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/local/mayak/internal/config"
	"github.com/local/mayak/internal/remote"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: remotecheck <task-slug>")
		os.Exit(2)
	}
	settings, err := config.Load()
	remoteID := ""
	for _, target := range settings.RemoteTargets {
		if target.Tasks && target.ID != "" {
			remoteID = target.ID
			break
		}
	}
	if remoteID == "" {
		remoteID = settings.RemoteID
	}
	if err != nil || remoteID == "" {
		fmt.Fprintln(os.Stderr, "MAYAK Remote ID is not configured")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := remote.New(remoteID)
	defer client.Close()
	if err := client.SendTask(ctx, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("task command sent")
}
