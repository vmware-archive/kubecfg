package cmd

import (
	"testing"
)

func TestGuessShell(t *testing.T) {
	t.Parallel()

	for _, test := range [][]string{
		{"/bin/bash", "bash"},
		{"/usr/bin/zsh", "zsh"},
		{"/usr/bin/zsh5", "zsh"},
	} {
		if result := guessShell(test[0]); result != test[1] {
			t.Errorf("Guessed %q instead of %q from %q", result, test[1], test[0])
		}
	}
}
