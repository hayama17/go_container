package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	cgroupsv2 "github.com/containerd/cgroups/v2"
)

func Make_register_cgroup(Manager_name string, res cgroupsv2.Resources) (*cgroupsv2.Manager, error) {
	log.Println("create cgroup maneger")
	mgr, err := cgroupsv2.NewManager("/sys/fs/cgroup", "/"+Manager_name, &res)
	if err != nil {
		return nil, err
	}

	log.Println("register tasks to my-container")
	if err := ioutil.WriteFile("/sys/fs/cgroup/"+Manager_name+"/cgroup.procs", []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		log.Println("err")

		return nil, err
	}
	log.Println("ok")

	return mgr, nil
}
