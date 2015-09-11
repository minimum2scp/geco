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
	app.Authors = []cli.Author{cli.Author{Name: "YAMADA Tsuyoshi", Email: "tyamada@minimum2scp.org"}}
	app.Commands = Commands

	app.Run(os.Args)
}
