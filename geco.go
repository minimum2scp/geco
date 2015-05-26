package main

import (
	"os"

	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "geco"
	app.Version = Version
	app.Usage = ""
	app.Author = "Shinichirow KAMITO"
	app.Email = "updoor@gmail.com"
	app.Commands = Commands

	app.Run(os.Args)
}
