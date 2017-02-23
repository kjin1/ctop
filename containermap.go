package main

import (
	"sort"
	"strings"

	"github.com/bcicen/ctop/config"
	"github.com/bcicen/ctop/metrics"
	"github.com/bcicen/ctop/widgets"
	"github.com/fsouza/go-dockerclient"
)

type ContainerMap struct {
	client     *docker.Client
	containers map[string]*Container
	collectors map[string]metrics.Collector
}

func NewContainerMap() *ContainerMap {
	// init docker client
	client, err := docker.NewClient(config.GetVal("dockerHost"))
	if err != nil {
		panic(err)
	}
	cm := &ContainerMap{
		client:     client,
		containers: make(map[string]*Container),
		collectors: make(map[string]metrics.Collector),
	}
	cm.Refresh()
	return cm
}

func (cm *ContainerMap) Refresh() {
	var id, name string

	opts := docker.ListContainersOptions{All: true}
	containers, err := cm.client.ListContainers(opts)
	if err != nil {
		panic(err)
	}

	// add new containers
	states := make(map[string]string)
	for _, c := range containers {
		id = c.ID[:12]
		states[id] = c.State

		if _, ok := cm.containers[id]; ok == false {
			name = strings.Replace(c.Names[0], "/", "", 1) // use primary container name
			cm.containers[id] = &Container{
				id:      id,
				name:    name,
				widgets: widgets.NewCompact(id, name),
			}
		}

		if _, ok := cm.collectors[id]; ok == false {
			cm.collectors[id] = metrics.NewDocker(cm.client, id)
		}

	}

	var removeIDs []string
	var collectIDs []string
	for id, c := range cm.containers {
		// mark stale internal containers
		if _, ok := states[id]; ok == false {
			removeIDs = append(removeIDs, id)
			continue
		}
		c.SetState(states[id])
		// start collector if needed
		if c.state == "running" {
			collectIDs = append(collectIDs, id)
		}
	}

	for _, id := range collectIDs {
		if !cm.collectors[id].Running() {
			cm.collectors[id].Start()
			stream := cm.collectors[id].Stream()
			cm.containers[id].Read(stream)
		}
	}

	// delete removed containers
	cm.Del(removeIDs...)
}

// Kill a container by ID
func (cm *ContainerMap) Kill(id string, sig docker.Signal) error {
	opts := docker.KillContainerOptions{
		ID:     id,
		Signal: sig,
	}
	return cm.client.KillContainer(opts)
}

// Return number of containers/rows
func (cm *ContainerMap) Len() uint {
	return uint(len(cm.containers))
}

// Get a single container, by ID
func (cm *ContainerMap) Get(id string) *Container {
	return cm.containers[id]
}

// Remove one or more containers
func (cm *ContainerMap) Del(ids ...string) {
	for _, id := range ids {
		delete(cm.containers, id)
		delete(cm.collectors, id)
	}
}

// Return array of all containers, sorted by field
func (cm *ContainerMap) All() []*Container {
	var containers Containers

	for _, c := range cm.containers {
		containers = append(containers, c)
	}

	sort.Sort(containers)
	return containers
}
