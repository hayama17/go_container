package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	cgroupsv2 "github.com/containerd/cgroups/v2"
)

func Make_register_cgroup(Manager_name string, all_res cgroupsv2.Resources, domain0_res cgroupsv2.Resources) error {
	log.Println("create cgroup maneger")
	mgr, err := cgroupsv2.NewManager("/sys/fs/cgroup", "/"+Manager_name, &all_res)
	if err != nil {
		return err
	}
	defer mgr.Delete()

	pmgr, err := cgroupsv2.NewManager("/sys/fs/cgroup/"+Manager_name, "/domain0", &domain0_res)
	if err != nil {
		return err
	}
	defer pmgr.Delete()

	log.Println("register tasks to my-container")
	if err := ioutil.WriteFile("/sys/fs/cgroup/"+Manager_name+"/domain0/cgroup.procs", []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		log.Println("err")

		return err
	}
	log.Println("ok")

	return nil
}
