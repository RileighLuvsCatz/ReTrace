package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/RileighLuvsCatz/retrace/internal/daemon"
)

// Execute is the CLI entrypoint. Cobra commands will hang from here in Phase 3.
func Execute() int {
	if len(os.Args) == 1 {
		fmt.Println("ReTrace")
		return 0
	}

	if os.Args[1] != "daemon" {
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		return 1
	}

	return executeDaemon(os.Args[2:])
}

func executeDaemon(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: retrace daemon start|stop|status|run")
		return 1
	}

	config := daemon.DefaultConfig()

	switch args[0] {
	case "start":
		if err := daemon.Start(config); err != nil {
			fmt.Fprintf(os.Stderr, "daemon start failed: %v\n", err)
			return 1
		}

		fmt.Printf("daemon started; pid file: %s\n", config.PIDPath)
		return 0
	case "stop":
		if err := daemon.Stop(config); err != nil {
			fmt.Fprintf(os.Stderr, "daemon stop failed: %v\n", err)
			return 1
		}

		fmt.Println("daemon stopped")
		return 0
	case "status":
		pid, running, err := daemon.Status(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon status failed: %v\n", err)
			return 1
		}
		if running {
			fmt.Printf("daemon running with pid %d\n", pid)
			return 0
		}

		fmt.Println("daemon stopped")
		return 0
	case "run":
		if err := daemon.RunForeground(context.Background(), config); err != nil {
			fmt.Fprintf(os.Stderr, "daemon run failed: %v\n", err)
			return 1
		}

		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon command %q\n", args[0])
		return 1
	}
}
