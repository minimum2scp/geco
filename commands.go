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
	"github.com/olekukonko/tablewriter"
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

// JSON struct
// config
type configCore struct {
	Project string `json:"project"`
	Account string `json:"account"`
}
type configRoot struct {
	Core configCore `json:"core"`
}

// projects
type projectJSON struct {
	ProjectNumber string `json:"projectNumber"`
	ProjectID string `json:"projectId"`
	ProjectName string `json:"name"`
	LifecycleState string `json:"lifecycleState"`
	CreateTime string `json:"createTime"`
}

// instance
type accessConfigsJSON struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	NatIP string `json:"natIP"`
	Type string `json:"type"`
}
type networkInterfacesJSON struct {
	Name string `json:"name"`
	Network string `json:"network"`
	NetworkIP string `json:"networkIP"`
	AccessConfigs []accessConfigsJSON `json:"accessConfigs"`
}
type instanceJSON struct {
	Id string `json:"id"`
	Kind string `json:"kind"`
	MachineType string `json:"machineType"`
	Name string `json:"name"`
	Status string `json:"status"`
	Zone string `json:"zone"`
	NetworkInterfaces []networkInterfacesJSON `json:"networkInterfaces"`
	CreationTimestamp string `json:"creationTimestamp"`
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
	gcloudCom := exec.Command("gcloud", "alpha", "projects", "list", "--format=json")
	gcloudComOut, _ := gcloudCom.Output()
	buf := renderProjectTable(gcloudComOut)
	out := PecoCommand(buf)
	projectLine := strings.Fields(out)
	project := projectLine[1]

	exec.Command("gcloud", "config", "set", "project", project).Output()
}

func doSsh(c *cli.Context) {
	config := LoadConfig()
	project := config.Core.Project

	gcloudCom := exec.Command("gcloud", "compute", "--project=" + project, "instances", "list",
		"--sort-by=name", "--format=json")
	gcloudComOut, _ := gcloudCom.Output()
	buf := renderInstanceTable(gcloudComOut)

	out := PecoCommand(buf)
	instanceLine := strings.Fields(out)
	instance := instanceLine[1]
	zone := instanceLine[3]

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

func LoadConfig() configRoot {
	out, _ := exec.Command("gcloud", "config", "list", "--format=json").Output()
	buf := string(out)
	decoder := json.NewDecoder(strings.NewReader(buf))
	var d configRoot
	decoder.Decode(&d)
	return d
}

func renderProjectTable(b []byte) []byte {
	decoder := json.NewDecoder(bytes.NewReader(b))
	var d []projectJSON
	decoder.Decode(&d)

	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"PROJECT_ID", "NAME", "PROJECT_NUMBER"})
	for _, p := range d {
		row := []string{ p.ProjectID, p.ProjectName, p.ProjectNumber }
		table.Append(row)
	}
	table.Render()

	return buf.Bytes()
}

func renderInstanceTable(b []byte) []byte {
	decoder := json.NewDecoder(bytes.NewReader(b))
	var d []instanceJSON
	decoder.Decode(&d)

	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"NAME", "ZONE", "MACHINE_TYPE", "INTERNAL_IP", "EXTERNAL_IP", "STATUS"})
	for _, ins := range d {
		internal_ip := ins.NetworkInterfaces[0].NetworkIP
		external_ip := ins.NetworkInterfaces[0].AccessConfigs[0].NatIP
		row := []string{ ins.Name, ins.Zone, ins.MachineType, internal_ip, external_ip, ins.Status}
		table.Append(row)
	}
	table.Render()

	return buf.Bytes()
}


func PecoCommand(into []byte) string {
	var buff bytes.Buffer
	pecoCom := exec.Command("peco")
	pecoCom.Stdin  = bytes.NewReader(into) // , _ = c.StdoutPipe()
	pecoCom.Stdout = &buff

	_ = pecoCom.Start()
	_ = pecoCom.Wait()

	out := buff.String()
	return out
}
