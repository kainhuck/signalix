package main

import (
	"os"

	"github.com/kainhuck/signalix/internal/cli/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
