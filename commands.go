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
	"sort"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/mitchellh/go-homedir"
	"github.com/olekukonko/tablewriter"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1beta1"
	compute "google.golang.org/api/compute/v1"
)

var commands = []cli.Command{
	commandCache,
	commandProject,
	commandSSH,
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
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name: "zsh-widget, z",
		},
	},
}

var commandSSH = cli.Command{
	Name:  "ssh",
	Usage: "",
	Description: `
`,
	Action: doSSH,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name: "zsh-widget, z",
		},
	},
}

var commandCurrentProject = cli.Command{
	Name:  "current",
	Usage: "",
	Description: `
`,
	Action: doCurrentProject,
}

var maxParallelAPICalls = 10

// JSON struct
// config
type configCore struct {
	Project string `json:"project"`
	Account string `json:"account"`
}
type configRoot struct {
	Core configCore `json:"core"`
}

// sort Projects by projectId
type projectsByID []*cloudresourcemanager.Project

func (by projectsByID) Len() int {
	return len(by)
}
func (by projectsByID) Less(i, j int) bool {
	return by[i].ProjectId < by[j].ProjectId
}
func (by projectsByID) Swap(i, j int) {
	by[i], by[j] = by[j], by[i]
}

// sort Instances by selfLink
type instancesBySelfLink []*compute.Instance

func (by instancesBySelfLink) Len() int {
	return len(by)
}
func (by instancesBySelfLink) Less(i, j int) bool {
	return by[i].SelfLink < by[j].SelfLink
}
func (by instancesBySelfLink) Swap(i, j int) {
	by[i], by[j] = by[j], by[i]
}

// cache
type cache struct {
	CacheDir  string
	Instances []*compute.Instance
	Projects  []*cloudresourcemanager.Project
}

func loadCache() (*cache, error) {
	cachedir, err := homedir.Expand("~/.cache/geco/")
	if err != nil {
		return nil, err
	}
	c := cache{CacheDir: cachedir}
	_, err = os.Stat(c.CacheDir)
	if os.IsNotExist(err) {
		os.MkdirAll(c.CacheDir, 0700)
	}
	fileinfos, _ := ioutil.ReadDir(c.CacheDir)
	for _, fileinfo := range fileinfos {
		if fileinfo.IsDir() {
			continue
		}
		if fileinfo.Name() == "projects.json" {
			projectsJSON, err := ioutil.ReadFile(filepath.Join(c.CacheDir, fileinfo.Name()))
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(projectsJSON, &c.Projects)
			if err != nil {
				return nil, err
			}
		}
		if fileinfo.Name() == "instances.json" {
			instancesJSON, err := ioutil.ReadFile(filepath.Join(c.CacheDir, fileinfo.Name()))
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(instancesJSON, &c.Instances)
			if err != nil {
				return nil, err
			}
		}
	}

	return &c, nil
}

func saveCache(c *cache) error {
	projectsJSON, _ := json.Marshal(c.Projects)
	err := ioutil.WriteFile(filepath.Join(c.CacheDir, "projects.json"), projectsJSON, 0600)
	if err != nil {
		return err
	}
	instancesJSON, _ := json.Marshal(c.Instances)
	err = ioutil.WriteFile(filepath.Join(c.CacheDir, "instances.json"), instancesJSON, 0600)
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

func doCache(cliCtx *cli.Context) {
	c, err := loadCache()
	if err != nil {
		panic(err)
	}
	c.Projects = []*cloudresourcemanager.Project{}
	c.Instances = []*compute.Instance{}

	ctx := oauth2.NoContext
	scopes := []string{compute.ComputeReadonlyScope}
	client, err := google.DefaultClient(ctx, scopes...)
	if err != nil {
		panic(err)
	}

	// gcloud beta projects list
	log.Println("loading projects...")
	service, err := cloudresourcemanager.New(client)
	if err != nil {
		panic(err)
	}

	projectsListCall := service.Projects.List()
	for {
		res, err := projectsListCall.Do()

		if err != nil {
			panic(err)
		}

		c.Projects = append(c.Projects, res.Projects...)

		if res.NextPageToken != "" {
			log.Printf("loading more projects with nextPageToken ...")
			projectsListCall.PageToken(res.NextPageToken)
		} else {
			break
		}
	}
	log.Printf("loaded projects, %d projects found.\n", len(c.Projects))

	semaphore := make(chan int, maxParallelAPICalls)
	notify := make(chan []*compute.Instance)

	// gcloud compute instances list (in parallel)
	for _, project := range c.Projects {
		go func(project *cloudresourcemanager.Project, notify chan<- []*compute.Instance) {
			semaphore <- 0
			var instances []*compute.Instance

			log.Printf("loading instances in %s (%s)...\n", project.Name, project.ProjectId)
			service, err := compute.New(client)
			if err != nil {
				log.Printf("error on loading instances in %s (%s), ignored: %s\n", project.Name, project.ProjectId, err)
				notify <- nil
				<-semaphore
				return
			}

			aggregatedListCall := service.Instances.AggregatedList(project.ProjectId)
			// aggregated_list_call.MaxResults(10)
			for {
				res, err := aggregatedListCall.Do()

				if err != nil {
					log.Printf("error on loading instances in %s (%s), ignored: %s\n", project.Name, project.ProjectId, err)
					notify <- nil
					<-semaphore
					return
				}

				for _, instancesScopedList := range res.Items {
					instances = append(instances, instancesScopedList.Instances...)
				}

				if res.NextPageToken != "" {
					log.Printf("loading more instances with nextPageToken in %s (%s) ...", project.Name, project.ProjectId)
					aggregatedListCall.PageToken(res.NextPageToken)
				} else {
					break
				}
			}

			<-semaphore
			notify <- instances

			log.Printf("loaded instances in %s (%s), %d instances found.\n", project.Name, project.ProjectId, len(instances))
		}(project, notify)
	}
	for _ = range c.Projects {
		instances, _ := <-notify
		if instances != nil {
			c.Instances = append(c.Instances, instances...)
		}
	}

	// sort projects, instances
	sort.Sort(projectsByID(c.Projects))
	sort.Sort(instancesBySelfLink(c.Instances))

	saveCache(c)
	log.Println("saved cache.")
}

func doProject(cliCtx *cli.Context) {
	cache, err := loadCache()
	if err != nil {
		panic(err)
	}

	buf := renderProjectTable(cache.Projects)
	out := pecoCommand(buf)
	projectLine := strings.Fields(out)
	if len(projectLine) == 0 {
		os.Exit(1)
	}

	projectID := projectLine[1]

	cmd := []string{"gcloud", "config", "set", "project", projectID}

	if cliCtx.Bool("zsh-widget") {
		fmt.Println(strings.Join(cmd, " "))
	} else {
		log.Println(strings.Join(cmd, " "))
		exec.Command(cmd[0], cmd[1:]...).Output()
	}
}

func doSSH(cliCtx *cli.Context) {
	cache, err := loadCache()
	if err != nil {
		panic(err)
	}

	config := loadConfig()
	project := config.Core.Project

	buf := renderInstanceTable(project, cache.Instances)
	out := pecoCommand(buf)
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

	cmd := []string{"gcloud", "compute", "ssh", "--project=" + project, "--zone=" + zone, instance}

	if cliCtx.Bool("zsh-widget") {
		fmt.Println(strings.Join(cmd, " "))
	} else {
		log.Println(strings.Join(cmd, " "))
		sshCom := exec.Command(cmd[0], cmd[1:]...)
		sshCom.Stdout = os.Stdout
		sshCom.Stderr = os.Stderr
		sshCom.Stdin = os.Stdin
		err = sshCom.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func doCurrentProject(cliCtx *cli.Context) {
	config := loadConfig()
	project := config.Core.Project
	fmt.Println("project: " + project)
}

func loadConfig() configRoot {
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

func renderInstanceTable(projectID string, instances []*compute.Instance) []byte {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	if projectID == "" {
		table.SetHeader([]string{"PROJECT", "NAME", "ZONE", "MACHINE_TYPE", "INTERNAL_IP", "EXTERNAL_IP", "STATUS"})
	} else {
		table.SetHeader([]string{"NAME", "ZONE", "MACHINE_TYPE", "INTERNAL_IP", "EXTERNAL_IP", "STATUS"})
	}
	for _, ins := range instances {
		p := (func(selflink string) string {
			return strings.Split(strings.Split(selflink, "https://www.googleapis.com/compute/v1/projects/")[1], "/")[0]
		})(ins.SelfLink)
		zone := (func(a []string) string { return a[len(a)-1] })(strings.Split(ins.Zone, "/"))
		machineType := (func(a []string) string { return a[len(a)-1] })(strings.Split(ins.MachineType, "/"))
		internalIP := ins.NetworkInterfaces[0].NetworkIP
		externalIP := ins.NetworkInterfaces[0].AccessConfigs[0].NatIP
		var row []string
		if projectID == "" {
			row = []string{p, ins.Name, zone, machineType, internalIP, externalIP, ins.Status}
			table.Append(row)
		} else {
			if projectID == p {
				row = []string{ins.Name, zone, machineType, internalIP, externalIP, ins.Status}
				table.Append(row)
			}
		}
	}
	table.Render()

	return buf.Bytes()
}

func pecoCommand(into []byte) string {
	var buff bytes.Buffer
	pecoCom := exec.Command("peco")
	pecoCom.Stdin = bytes.NewReader(into) // , _ = c.StdoutPipe()
	pecoCom.Stdout = &buff

	_ = pecoCom.Start()
	_ = pecoCom.Wait()

	out := buff.String()
	return out
}
