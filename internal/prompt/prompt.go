package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AskTerminal opens /dev/tty directly to prompt the user for approval,
// bypassing stdin/stdout which are used for hook JSON communication.
func AskTerminal(description string) bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open terminal for local prompt, defaulting to deny: %v\n", err)
		return false
	}
	defer tty.Close()

	fmt.Fprintf(tty, "\n🛡️  [AgenTerm Local Security Approval]\n")
	fmt.Fprintf(tty, "Agent is requesting to execute the following command/tool:\n> %s\n\n", description)
	fmt.Fprintf(tty, "Allow execution? [y/N]: ")

	reader := bufio.NewReader(tty)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	return answer == "y" || answer == "yes"
}
