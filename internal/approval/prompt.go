package approval

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// CLIPrompter спрашивает подтверждение в терминале и считает Enter отказом.
type CLIPrompter struct {
	In  io.Reader
	Out io.Writer
}

// NewCLIPrompter создаёт интерактивный prompter для команды run.
func NewCLIPrompter(in io.Reader, out io.Writer) CLIPrompter {
	return CLIPrompter{In: in, Out: out}
}

// Prompt печатает краткое описание операции и ждёт y/yes.
func (p CLIPrompter) Prompt(request Request) (Decision, error) {
	if p.In == nil {
		return Decision{}, fmt.Errorf("stdin is not available")
	}
	if p.Out != nil {
		fmt.Fprintf(p.Out, "\nApproval required for %s\n", request.Tool)
		fmt.Fprintf(p.Out, "Reason: %s\n", request.Reason)
		if request.Subject != "" {
			fmt.Fprintf(p.Out, "Subject: %s\n", request.Subject)
		}
		fmt.Fprint(p.Out, "Allow once? [y/N]: ")
	}
	line, err := bufio.NewReader(p.In).ReadString('\n')
	if err != nil && len(line) == 0 {
		return Decision{}, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "y" || answer == "yes" {
		return Decision{Approved: true, Source: SourceCLI, Reason: request.Reason, Message: "approved by user"}, nil
	}
	return Decision{Approved: false, Source: SourceCLI, Reason: request.Reason, Message: "denied by user"}, nil
}
