package main

import (
	"log"
	"os"
	"os/exec"
	"bytes"
	"fmt"
	"strings"
	"encoding/json"
	"github.com/codegangsta/cli"
)

var Commands = []cli.Command{
	commandProject,
	commandSsh,
	commandCurrentProject,
}

var commandProject = cli.Command{
	Name:  "project",
	Usage: "",
	Description: `
`,
	Action: doProject,
}

var commandSsh = cli.Command{
	Name:  "ssh",
	Usage: "",
	Description: `
`,
	Action: doSsh,
}

var commandCurrentProject = cli.Command{
	Name:  "current",
	Usage: "",
	Description: `
`,
	Action: doCurrentProject,
}

func debug(v ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		log.Println(v...)
	}
}

func assert(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func doProject(c *cli.Context) {
	gcloudCom := exec.Command("gcloud", "alpha", "projects", "list")
	out := PecoCommand(gcloudCom)
	projectLine := strings.Fields(out)
	project := projectLine[0]
	fmt.Println("Select project: " + project)

	exec.Command("gcloud", "config", "set", "project", project).Output()
}

func doSsh(c *cli.Context) {
	config := LoadConfig()
	project := config.Core.Project

	gcloudCom := exec.Command("gcloud", "compute", "--project=" + project, "instances", "list", "--sort-by=name")
	out := PecoCommand(gcloudCom)
	instanceLine := strings.Fields(out)
	instance := instanceLine[0]
	zone := instanceLine[1]
	fmt.Println("    instance: " + instance)
	fmt.Println("        zone: " + zone)

	sshCom := exec.Command("gcloud", "compute", "--project=" + project, "ssh", "--zone=" + zone, instance)
	sshCom.Stdout = os.Stdout
	sshCom.Stderr = os.Stderr
	sshCom.Stdin = os.Stdin
	sshCom.Run()
}

func doCurrentProject(c *cli.Context) {
	config := LoadConfig()
	project := config.Core.Project
	fmt.Println("project: " + project)
}

type configCore struct {
	Project string `json:"project"`
	Account string `json:"account"`
}
type configRoot struct {
	Core configCore `json:"core"`
}

func LoadConfig() configRoot {
	out, _ := exec.Command("gcloud", "config", "list", "--format=json").Output()
	buf := string(out)
	decoder := json.NewDecoder(strings.NewReader(buf))
	var d configRoot
	decoder.Decode(&d)
	return d
}

func PecoCommand(c *exec.Cmd) string {
	var buff bytes.Buffer
	pecoCom := exec.Command("peco")
	pecoCom.Stdin, _ = c.StdoutPipe()
	pecoCom.Stdout = &buff

	_ = pecoCom.Start()
	_ = c.Run()
	_ = pecoCom.Wait()

	out := buff.String()
	return out
}
