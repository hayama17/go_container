package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"

	cgroupsv2 "github.com/containerd/cgroups/v2"
)

func main() {
	switch os.Args[1] {
	case "run":
		parent()
	case "child":
		if err := child(); err != nil {
			panic(err)
		}
	default:
		panic("wat should I do")
	}
}

func parent() error {

	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	return nil
}

func child() error {
	minMem := int64(1)                 // 1K
	maxMem := int64(500 * 1024 * 1024) //100M
	res := cgroupsv2.Resources{
		Memory: &cgroupsv2.Memory{
			// values are in bytes: https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#memory-interface-files
			Min: &minMem,
			Max: &maxMem,
		},
	}
	mgr, err := cgroupsv2.NewManager("/sys/fs/cgroup", "/go-container-cgroupv2", &res)

	if err != nil {
		return fmt.Errorf("creating cgroups v2: %w", err)
	}
	defer mgr.Delete()

	if err := ioutil.WriteFile("/sys/fs/cgroup/go-container-cgroupv2/cgroup.procs", []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("Cgroups register tasks to my-container namespace failed: %w", err)
	}
	log.Println("prepareRoot")
	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("prepare RootFS: %w", err)
	}

	log.Println("mkdir newroot/putold")
	if err := os.MkdirAll("newroot/putold", 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	log.Println("bind mount to ./newroot")
	if err := syscall.Mount("newroot", "newroot", "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting newroot: %w", err)
	}

	log.Println("run pivot_root")
	if err := syscall.PivotRoot("newroot", "newroot/putold"); err != nil {
		return fmt.Errorf("pivot root: %w", err)
	}

	log.Println("===========================================")
	log.Println("chdir / and go inside pivot root jail. Now we are in created container!")
	log.Println("===========================================")
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change dir to /: %w", err)
	}

	// TIPS: by unmounting, parent resource will be hidden from child process
	if err := syscall.Unmount("/putold", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %w", err)
	}

	// mouting /proc inside container
	log.Println("mount /proc")
	// NOTE: somehow mount /proc fails in lima & nerdctl with EPERM
	if err := syscall.Mount("proc", "proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mounting new /proc in container: %w", err)
	}

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	return nil
}
