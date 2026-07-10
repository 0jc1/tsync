package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "tsync:", err)
		os.Exit(1)
	}
}

func runCLI(args []string, stdout, stderr io.Writer) error {
	defaultPath, err := defaultConfigPath()
	if err != nil {
		return err
	}

	flags := flag.NewFlagSet("tsync", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", defaultPath, "path to JSON configuration")
	showHelp := flags.Bool("help", false, "show help")
	flags.BoolVar(showHelp, "h", false, "show help")
	showVersion := flags.Bool("version", false, "show version")
	if err := flags.Parse(orderCLIArgs(args)); err != nil {
		return err
	}

	command := ""
	remaining := flags.Args()
	if len(remaining) > 1 {
		return fmt.Errorf("expected at most one command")
	}
	if len(remaining) == 1 {
		command = strings.ToLower(remaining[0])
	}
	if *showHelp {
		command = "help"
	}
	if *showVersion {
		command = "version"
	}

	switch command {
	case "help":
		printHelp(stdout, defaultPath)
		return nil
	case "version":
		fmt.Fprintf(stdout, "tsync %s\n", version)
		return nil
	case "validate":
		cfg, err := loadConfig(*configPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "valid: %d sync set(s)\n", len(cfg.Syncs))
		return nil
	case "status":
		return printStatus(stdout, *configPath)
	case "":
		return runDaemon(*configPath, stdout)
	default:
		return fmt.Errorf("unknown command %q; run 'tsync help'", command)
	}
}

func orderCLIArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	commands := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(arg, "--config="), arg == "--help", arg == "-h", arg == "--version":
			flags = append(flags, arg)
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		default:
			commands = append(commands, arg)
		}
	}
	return append(flags, commands...)
}

func runDaemon(configPath string, output io.Writer) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	existing, err := readRuntimeStatus(configPath)
	if err != nil {
		return err
	}
	if existing.Running && existing.PID != os.Getpid() {
		return fmt.Errorf("already running with PID %d", existing.PID)
	}

	status := newStatusStore(configPath)
	if err := status.start(); err != nil {
		return fmt.Errorf("write runtime status: %w", err)
	}
	defer status.stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := log.New(output, "tsync: ", log.LstdFlags)
	logger.Printf("started with %d sync set(s)", len(cfg.Syncs))
	err = newEngine(cfg, logger, status).Run(ctx)
	if err == nil {
		logger.Print("stopped")
	}
	return err
}

func printStatus(output io.Writer, configPath string) error {
	state, err := readRuntimeStatus(configPath)
	if err != nil {
		return err
	}
	if !state.Running {
		fmt.Fprintln(output, "stopped")
		if state.LastError != "" {
			fmt.Fprintf(output, "last error: %s\n", state.LastError)
		}
		return nil
	}
	fmt.Fprintf(output, "running (PID %d, since %s)\n", state.PID, state.StartedAt.Format(time.RFC3339))
	if !state.LastSyncAt.IsZero() {
		fmt.Fprintf(output, "last sync: %s (%s -> %s)\n",
			state.LastSyncAt.Format(time.RFC3339), state.LastSource, state.LastDestination)
	}
	if state.LastError != "" {
		fmt.Fprintf(output, "last error: %s\n", state.LastError)
	}
	return nil
}

func printHelp(output io.Writer, defaultPath string) {
	fmt.Fprintf(output, `tsync keeps configured files synchronized.

Usage:
  tsync [--config PATH]           Run the sync process
  tsync [--config PATH] status    Show runtime status
  tsync [--config PATH] validate  Validate the configuration
  tsync help                      Show this help
  tsync version                   Show the version

Default config: %s
`, defaultPath)

}
