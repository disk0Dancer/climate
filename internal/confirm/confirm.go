package confirm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Ask prompts the user for a y/N confirmation.
func Ask(in io.Reader, out io.Writer, prompt string) (bool, error) {
	reader := bufio.NewReader(in)

	for {
		if _, err := fmt.Fprintf(out, "%s [y/N]: ", prompt); err != nil {
			return false, err
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}

		answer := strings.ToLower(strings.TrimSpace(line))
		switch answer {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		}

		if _, writeErr := fmt.Fprintln(out, "Please answer yes or no."); writeErr != nil {
			return false, writeErr
		}

		if err == io.EOF {
			return false, nil
		}
	}
}
