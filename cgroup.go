package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	cgroupsv2 "github.com/containerd/cgroups/v2"
)

func Make_register_cgroup(Manager_name string, res cgroupsv2.Resources) error {
	log.Println("create cgroup maneger")
	mgr, err := cgroupsv2.NewManager("/sys/fs/cgroup", "/"+Manager_name, &res)
	if err != nil {
		return err
	}
	defer mgr.Delete()

	log.Println("register tasks to my-container")
	if err := ioutil.WriteFile("/sys/fs/cgroup/go-container-cgroupv2/cgroup.procs", []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return err
	}
	return nil
}
