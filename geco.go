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
	app.Authors = []cli.Author{
		cli.Author{Name: "YAMADA Tsuyoshi", Email: "tyamada@minimum2scp.org"},
		cli.Author{Name: "Shinichirow KAMITO", Email: "updoor@gmail.com"},
	}
	app.Commands = Commands

	app.Run(os.Args)
}
