package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Confirm writes a question and reads one line of input. It returns true only
// for "y" or "yes", ignoring case and surrounding whitespace.
func Confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	if _, err := fmt.Fprint(out, question); err != nil {
		return false, err
	}

	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
