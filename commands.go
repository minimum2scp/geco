package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/codegangsta/cli"
	cloudresourcemanager "github.com/google/google-api-go-client/cloudresourcemanager/v1beta1"
	compute "github.com/google/google-api-go-client/compute/v1"
	"github.com/mitchellh/go-homedir"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var Commands = []cli.Command{
	commandCache,
	commandProject,
	commandSsh,
	commandCurrentProject,
}

var commandCache = cli.Command{
	Name:  "cache",
	Usage: "",
	Description: `
`,
	Action: doCache,
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

// cache
type t_cache struct {
	CacheDir  string
	Instances []*compute.Instance
	Projects  []*cloudresourcemanager.Project
}

func LoadCache() (*t_cache, error) {
	cachedir, err := homedir.Expand("~/.cache/geco/")
	if err != nil {
		return nil, err
	}
	cache := t_cache{CacheDir: cachedir}
	_, err = os.Stat(cache.CacheDir)
	if os.IsNotExist(err) {
		os.MkdirAll(cache.CacheDir, 0700)
	}
	fileinfos, _ := ioutil.ReadDir(cache.CacheDir)
	for _, fileinfo := range fileinfos {
		if fileinfo.IsDir() {
			continue
		}
		if fileinfo.Name() == "projects.json" {
			projects_json, err := ioutil.ReadFile(filepath.Join(cache.CacheDir, fileinfo.Name()))
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(projects_json, &cache.Projects)
			if err != nil {
				return nil, err
			}
		}
		if fileinfo.Name() == "instances.json" {
			instances_json, err := ioutil.ReadFile(filepath.Join(cache.CacheDir, fileinfo.Name()))
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(instances_json, &cache.Instances)
			if err != nil {
				return nil, err
			}
		}
	}

	return &cache, nil
}

func SaveCache(cache *t_cache) error {
	projects_json, _ := json.Marshal(cache.Projects)
	err := ioutil.WriteFile(filepath.Join(cache.CacheDir, "projects.json"), projects_json, 0600)
	if err != nil {
		return err
	}
	instances_json, _ := json.Marshal(cache.Instances)
	err = ioutil.WriteFile(filepath.Join(cache.CacheDir, "instances.json"), instances_json, 0600)
	if err != nil {
		return err
	}
	return nil
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

func doCache(c *cli.Context) {
	cache, err := LoadCache()
	if err != nil {
		panic(err)
	}

	ctx := oauth2.NoContext
	scopes := []string{compute.ComputeReadonlyScope}
	client, err := google.DefaultClient(ctx, scopes...)
	if err != nil {
		panic(err)
	}

	// gcloud projects list
	service, err := cloudresourcemanager.New(client)
	if err != nil {
		panic(err)
	}

	res, err := service.Projects.List().Do()
	if err != nil {
		panic(err)
	}
	cache.Projects = res.Projects

	// gcloud instances list
	for _, project := range res.Projects {
		service, err := compute.New(client)
		if err != nil {
			log.Print(err)
			continue
		}

		res, err := service.Instances.AggregatedList(project.ProjectId).Do()
		if err != nil {
			log.Print(err)
			continue
		}

		for _, instances_scoped_list := range res.Items {
			cache.Instances = append(cache.Instances, instances_scoped_list.Instances...)
		}
	}

	SaveCache(cache)
}

func doProject(c *cli.Context) {
	cache, err := LoadCache()
	if err != nil {
		panic(err)
	}

	buf := renderProjectTable(cache.Projects)
	out := PecoCommand(buf)
	projectLine := strings.Fields(out)

	project := (func(projectName string) *cloudresourcemanager.Project {
		for _, p := range cache.Projects {
			if projectName == p.ProjectId {
				return p
			}
		}
		return nil
	})(projectLine[1])

	exec.Command("gcloud", "config", "set", "project", project.ProjectId).Output()
}

func doSsh(c *cli.Context) {
	cache, err := LoadCache()
	if err != nil {
		panic(err)
	}

	config := LoadConfig()
	project := config.Core.Project

	buf := renderInstanceTable(project, cache.Instances)
	out := PecoCommand(buf)
	instanceLine := strings.Fields(out)

	if len(instanceLine) == 0 {
		os.Exit(1)
	}

	var instance, zone string
	if project == "" {
		project = instanceLine[1]
		instance = instanceLine[3]
		zone = instanceLine[5]
	} else {
		instance = instanceLine[1]
		zone = instanceLine[3]
	}

	sshCom := exec.Command("gcloud", "compute", "--project="+project, "ssh", "--zone="+zone, instance)
	sshCom.Stdout = os.Stdout
	sshCom.Stderr = os.Stderr
	sshCom.Stdin = os.Stdin
	sshCom.Run()
}

func doCurrentProject(c *cli.Context) {
	config := LoadConfig()
	project := config.Core.Project
	fmt.Println("project: " + project)
	fmt.Printf("%#v", project)
}

func LoadConfig() configRoot {
	out, _ := exec.Command("gcloud", "config", "list", "--format=json").Output()
	buf := string(out)
	decoder := json.NewDecoder(strings.NewReader(buf))
	var d configRoot
	decoder.Decode(&d)
	return d
}

func renderProjectTable(projects []*cloudresourcemanager.Project) []byte {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"PROJECT_ID", "NAME", "PROJECT_NUMBER"})
	for _, p := range projects {
		row := []string{p.ProjectId, p.Name, fmt.Sprintf("%v", p.ProjectNumber)}
		table.Append(row)
	}
	table.Render()

	return buf.Bytes()
}

func renderInstanceTable(project_id string, instances []*compute.Instance) []byte {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	if project_id == "" {
		table.SetHeader([]string{"PROJECT", "NAME", "ZONE", "MACHINE_TYPE", "INTERNAL_IP", "EXTERNAL_IP", "STATUS"})
	} else {
		table.SetHeader([]string{"NAME", "ZONE", "MACHINE_TYPE", "INTERNAL_IP", "EXTERNAL_IP", "STATUS"})
	}
	for _, ins := range instances {
		p := (func(selflink string) string {
			return strings.Split(strings.Split(selflink, "https://www.googleapis.com/compute/v1/projects/")[1], "/")[0]
		})(ins.SelfLink)
		zone := (func(a []string) string { return a[len(a)-1] })(strings.Split(ins.Zone, "/"))
		machine_type := (func(a []string) string { return a[len(a)-1] })(strings.Split(ins.MachineType, "/"))
		internal_ip := ins.NetworkInterfaces[0].NetworkIP
		external_ip := ins.NetworkInterfaces[0].AccessConfigs[0].NatIP
		var row []string
		if project_id == "" {
			row = []string{p, ins.Name, zone, machine_type, internal_ip, external_ip, ins.Status}
			table.Append(row)
		} else {
			if project_id == p {
				row = []string{ins.Name, zone, machine_type, internal_ip, external_ip, ins.Status}
				table.Append(row)
			}
		}
	}
	table.Render()

	return buf.Bytes()
}

func PecoCommand(into []byte) string {
	var buff bytes.Buffer
	pecoCom := exec.Command("peco")
	pecoCom.Stdin = bytes.NewReader(into) // , _ = c.StdoutPipe()
	pecoCom.Stdout = &buff

	_ = pecoCom.Start()
	_ = pecoCom.Wait()

	out := buff.String()
	return out
}
