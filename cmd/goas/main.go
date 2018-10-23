package main

import (
	"log"
	"os"

	"github.com/Hao1995/go-swagger"
	"github.com/urfave/cli"
)

var version = "v0.1.2"

var flags = []cli.Flag{
	cli.StringFlag{
		Name:  "output",
		Value: "oas.json",
		Usage: "Output file",
	},
	cli.BoolFlag{
		Name:  "debug",
		Usage: "show debug message",
	},
}

func action(c *cli.Context) error {
	// func action() error {
	g := goas.New()

	if c.GlobalIsSet("debug") {
		g.EnableDebug = true
	}

	// fmt.Println(c.GlobalString("output"))
	return g.CreateOASFile(c.GlobalString("output"))
}

func main() {
	app := cli.NewApp()
	app.Name = "goas"
	app.Usage = ""
	app.UsageText = "goas [options]"
	app.Version = version
	app.Copyright = "(c) 2018 mikun800527@gmail.com"
	app.HideHelp = true
	app.OnUsageError = func(c *cli.Context, err error, isSubcommand bool) error {
		cli.ShowAppHelp(c)
		return nil
	}
	app.Flags = flags
	app.Action = action

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
