package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var version = "1.2.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
		os.Exit(0)
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "propose":
		os.Exit(runPropose(os.Args[2:]))
	case "proposal":
		os.Exit(runProposal(os.Args[2:]))
	case "gate":
		os.Exit(runGate(os.Args[2:]))
	case "hook":
		os.Exit(runHook(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: agenterm <command> [args]")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  init      [--push-key KEY] [--relay-url URL]")
	fmt.Fprintln(os.Stderr, "  version   Print version and exit")
	fmt.Fprintln(os.Stderr, "  propose   --title \"...\" --body \"...\" [--wait] [--timeout 60]")
	fmt.Fprintln(os.Stderr, "  proposal  status <proposal_id>")
	fmt.Fprintln(os.Stderr, "  hook      install [claude|gemini]")
	fmt.Fprintln(os.Stderr, "  hook      uninstall [claude|gemini]")
	fmt.Fprintln(os.Stderr, "  gate      (internal) used by hooks, not for direct use")
}

func outputJSON(v interface{}) {
	json.NewEncoder(os.Stdout).Encode(v)
}

// askLocallyInTerminal opens the user's terminal directly (/dev/tty) to prompt for approval
// bypassing stdin/stdout which are used for Hook JSON communication.
func askLocallyInTerminal(cmd string) bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open terminal for local prompt, defaulting to deny: %v\n", err)
		return false
	}
	defer tty.Close()

	fmt.Fprintf(tty, "\n🛡️  [AgenTerm Local Security Approval]\n")
	fmt.Fprintf(tty, "Agent is requesting to execute the following command/tool:\n> %s\n\n", cmd)
	fmt.Fprintf(tty, "Allow execution? [y/N]: ")

	reader := bufio.NewReader(tty)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	return answer == "y" || answer == "yes"
}
