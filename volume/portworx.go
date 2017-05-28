package volume

import (
	"fmt"
	"log"
	"os"
	"strings"

	dockerclient "github.com/fsouza/go-dockerclient"

	_ "github.com/libopenstorage/openstorage/api"
	clusterclient "github.com/libopenstorage/openstorage/api/client/cluster"
	"github.com/libopenstorage/openstorage/cluster"
)

var (
	nodes  []string
	docker *dockerclient.Client
)

type driver struct {
	hostConfig     *dockerclient.HostConfig
	clusterManager cluster.Cluster
}

func (d *driver) String() string {
	return "pxd"
}

func (d *driver) Init() error {
	log.Printf("Using the Portworx volume driver.\n")

	n := "127.0.0.1"
	if len(nodes) > 0 {
		n = nodes[0]
	}

	if clnt, err := clusterclient.NewClusterClient("http://"+n+":9001", "v1"); err != nil {
		return err
	} else {
		d.clusterManager = clusterclient.ClusterManager(clnt)
	}

	if cluster, err := d.clusterManager.Enumerate(); err != nil {
		return err
	} else {
		log.Printf("The following Portworx nodes are in the cluster:\n")
		for _, n := range cluster.Nodes {
			log.Printf(
				"\tNode ID: %v\tNode IP: %v\tNode Status: %v\n",
				n.Id,
				n.DataIp,
				n.Status,
			)
		}
	}

	return nil
}

// Portworx runs as a container - so all we need to do is ask docker to
// stop the running portworx container.
func (d *driver) Stop(Ip string) error {
	endpoint := "tcp://" + Ip + ":2375"
	docker, err := dockerclient.NewClient(endpoint)
	if err != nil {
		return err
	} else {
		if err = docker.Ping(); err != nil {
			return err
		}
	}

	// Find and stop the Portworx container
	lo := dockerclient.ListContainersOptions{
		All:  true,
		Size: false,
	}
	if allContainers, err := docker.ListContainers(lo); err != nil {
		return err
	} else {
		for _, c := range allContainers {
			if info, err := docker.InspectContainer(c.ID); err != nil {
				return err
			} else {
				if strings.Contains(info.Config.Image, "portworx/px") {
					if !info.State.Running {
						return fmt.Errorf(
							"Portworx container with ID %v is not running.",
							c.ID,
						)
					}

					d.hostConfig = info.HostConfig
					log.Printf("Stopping Portworx container with ID: %v\n", c.ID)
					if err = docker.StopContainer(c.ID, 0); err != nil {
						return err
					}
					return nil
				}
			}
		}
	}

	return fmt.Errorf("Could not find the Portworx container on %v", Ip)
}

func (d *driver) Start(Ip string) error {
	endpoint := "tcp://" + Ip + ":2375"
	docker, err := dockerclient.NewClient(endpoint)
	if err != nil {
		return err
	} else {
		if err = docker.Ping(); err != nil {
			return err
		}
	}

	// Find and stop the Portworx container
	lo := dockerclient.ListContainersOptions{
		All:  true,
		Size: false,
	}
	if allContainers, err := docker.ListContainers(lo); err != nil {
		return err
	} else {
		for _, c := range allContainers {
			if info, err := docker.InspectContainer(c.ID); err != nil {
				return err
			} else {
				if strings.Contains(info.Config.Image, "portworx/px") {
					if !info.State.Running {
						return fmt.Errorf(
							"Portworx container with ID %v is not stopped.",
							c.ID,
						)
					}

					log.Printf("Starting Portworx container with ID: %v\n", c.ID)
					if err = docker.StartContainer(c.ID, d.hostConfig); err != nil {
						return err
					}
					return nil
				}
			}
		}
	}

	return fmt.Errorf("Could not find the Portworx container on %v", Ip)
}

func init() {
	nodes = strings.Split(os.Getenv("CLUSTER_NODES"), ",")

	register("pxd", &driver{})
}
