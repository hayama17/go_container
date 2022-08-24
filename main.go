package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"

	cgroupsv2 "github.com/containerd/cgroups/v2"
	"github.com/vishvananda/netns"
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
		Cloneflags: syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWUTS,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	NsHandle, _ := netns.GetFromPid(cmd.Process.Pid)
	fmt.Println(NsHandle.String())
	if err := cmd.Wait(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}

	return nil
}

func child() error {
	//set veth
	// NsHandle, _ := netns.NewNamed("kohei")
	// netns.Set(NsHandle)
	//set hostname
	log.Println("set hostname")
	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("Setting hostname failed: %w", err)
	}

	log.Println("mount /proc")
	if err := syscall.Mount("proc", "//newroot/proc", "proc", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("Proc mount failed: %w", err)
	}
	//cgroup meory limit
	minMem := int64(1)                 // 1K
	maxMem := int64(500 * 1024 * 1024) //100M
	res := cgroupsv2.Resources{
		Memory: &cgroupsv2.Memory{
			Min: &minMem,
			Max: &maxMem,
		},
	}
	log.Println("create cgroup maneger")
	mgr, err := cgroupsv2.NewManager("/sys/fs/cgroup", "/go-container-cgroupv2", &res)
	if err != nil {
		return fmt.Errorf("creating cgroups v2: %w", err)
	}
	defer mgr.Delete()

	log.Println("register tasks to my-container")
	if err := ioutil.WriteFile("/sys/fs/cgroup/go-container-cgroupv2/cgroup.procs", []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("Cgroups register tasks to my-container namespace failed: %w", err)
	}

	//pivot root
	log.Println("prepare Rootfs")
	if err := syscall.Mount("", "/", "", syscall.MS_SLAVE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("prepare Rootfs: %w", err)
	}

	log.Println("bind mount .//newroot")
	if err := syscall.Mount("/newroot", "/newroot", "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting /newroot: %w", err)
	}

	log.Println("mkdir /newroot/putold")
	if err := os.MkdirAll("/newroot/putold", 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	log.Println("pivot_root")
	if err := syscall.PivotRoot("/newroot", "/newroot/putold"); err != nil {
		return fmt.Errorf("pivot root: %w", err)
	}

	log.Println("cd /")
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("change dir to /: %w", err)
	}

	if err := syscall.Unmount("/putold", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root dir %w", err)
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
