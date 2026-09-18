// simkit is an opt-in Linux sidecar toolkit. It has no ROS or Gazebo dependency.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const toolkitVersion = "0.5.1"

func main() {
	runtime.GOMAXPROCS(1)
	if err := cli(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Fprintln(os.Stderr, "simkit:", err)
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() > 0 {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
}
func cli(args []string) error {
	if len(args) > 0 && args[0] == "prepare-robot" {
		return prepareRobot(args[1:])
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Println("78QuadLink: simkit prepare-robot | version | doctor | run | inspect | record | verify | analyze\nUse simkit COMMAND -h for options. Run requires -- COMMAND [ARGS...].")
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: 78QuadLink: simkit prepare-robot | version | doctor | run | inspect | record | verify | analyze (use COMMAND -h)")
	}
	if args[0] == "version" {
		fmt.Println(toolkitVersion)
		return nil
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	if args[0] == "analyze" {
		input := fs.String("input", "", "G1REC1 .gz file; matching .json manifest required")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected positional arguments")
		}
		report, err := analyzeArchive(*input)
		if err != nil {
			return err
		}
		return printJSON(report)
	}
	switch args[0] {
	case "doctor", "run", "inspect", "record", "verify":
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	manifest := fs.String("adapter", "", "adapter JSON (required)")
	backend := fs.String("backend", "go1relay", "installed adapter backend executable")
	profile := fs.String("profile", "motor-imu", "declared adapter profile")
	board := fs.String("board", "", "existing board path for inspect/record")
	output := fs.String("output", "", "new archive for record, existing archive for verify")
	duration := fs.Duration("duration", 30*time.Second, "record duration")
	period := fs.Duration("period", 2*time.Millisecond, "record sampling interval")
	root := fs.String("shm-root", "/dev/shm", "board directory parent for run")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	a, err := loadAdapter(*manifest)
	if err != nil {
		return err
	}
	bin, err := exec.LookPath(*backend)
	if err != nil {
		return err
	}
	bin, err = filepath.Abs(bin)
	if err != nil {
		return err
	}
	if _, err = a.environment(nil, *profile, "<allocated-on-run>"); err != nil {
		return err
	}
	if args[0] != "run" && fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	switch args[0] {
	case "doctor":
		return printJSON(map[string]interface{}{"toolkit_version": toolkitVersion, "adapter": a, "backend": bin, "selected_profile": *profile, "runtime_compatibility": "not probed; install matching source/consumer adapters separately"})
	case "run":
		if fs.NArg() == 0 {
			return fmt.Errorf("run requires -- COMMAND [ARGS...]")
		}
		return runWithBoard(a, *profile, bin, *root, fs.Args())
	case "inspect":
		if *board == "" {
			return fmt.Errorf("inspect requires -board")
		}
		return invoke(bin, "-inspect-board", *board)
	case "record":
		if *board == "" || *output == "" || *duration <= 0 || *period <= 0 {
			return fmt.Errorf("record requires board, output and positive durations")
		}
		return invoke(bin, "-record-board", *board, "-record-output", *output, "-record-duration", duration.String(), "-record-period", period.String(), "-procs", "1")
	case "verify":
		if *output == "" {
			return fmt.Errorf("verify requires -output")
		}
		return invoke(bin, "-verify-record", *output)
	}
	return nil
}
func printJSON(v interface{}) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func invoke(bin string, args ...string) error { return supervise(exec.Command(bin, args...)) }
func runWithBoard(a adapter, profile, backend, root string, args []string) error {
	dir, err := os.MkdirTemp(root, "simkit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	board := filepath.Join(dir, "state")
	init := exec.Command(backend, "-init-board", board)
	// Backend diagnostics belong on stderr; stdout is reserved for the workload.
	init.Stdout = os.Stderr
	init.Stderr = os.Stderr
	if err = init.Run(); err != nil {
		return fmt.Errorf("initialize board: %w", err)
	}
	env, err := a.environment(os.Environ(), profile, board)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "simkit adapter=%s profile=%s board=%s\n", a.ID, profile, board)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	return supervise(cmd)
}

// Own only a new process group. Stop descendants before unlinking their board.
// Workloads must remain in this group (do not daemonize via setsid).
func supervise(cmd *exec.Cmd) error {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// An interactive child group must own the terminal, otherwise tcsetattr/read
	// stops the whole simulation with SIGTTOU/SIGTTIN. Non-TTY jobs stay unchanged.
	var foreground int32
	_, _, ttyErr := syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), syscall.TIOCGPGRP, uintptr(unsafe.Pointer(&foreground)))
	if ttyErr == 0 {
		signal.Ignore(syscall.SIGTTOU)
		defer signal.Reset(syscall.SIGTTOU)
		cmd.SysProcAttr.Foreground = true
		cmd.SysProcAttr.Ctty = int(os.Stdin.Fd())
		defer func() {
			syscall.Syscall(syscall.SYS_IOCTL, os.Stdin.Fd(), syscall.TIOCSPGRP, uintptr(unsafe.Pointer(&foreground)))
		}()
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	defer syscall.Kill(-pid, syscall.SIGKILL)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case sig := <-signals:
		_ = syscall.Kill(-pid, sig.(syscall.Signal))
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case err := <-done:
			return err
		case <-timer.C:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			return <-done
		case <-signals:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			return <-done
		}
	}
}
