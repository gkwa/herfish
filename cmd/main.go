package main

import (
	"os"

	"github.com/taylormonacelli/herfish"
)

func main() {
	code := herfish.Execute()
	os.Exit(code)
}
