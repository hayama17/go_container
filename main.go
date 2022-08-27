package main

import (
	"flag"
	"fmt"
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

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	netns := fs.String("n", "go_container", "netns flag")
	fs.String("i", "10.0.0.2/24", "container ip address flag")
	fs.String("c", "go-container-cgroupv2", "cgroup name flag")
	fs.Parse(os.Args[2:])
	netns_path := "/var/run/netns/" + *netns

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

	//make netns
	log.Println(netns)
	proc_path := "/proc/" + fmt.Sprint(cmd.Process.Pid) + "/ns/net"

	fd, err := Make_netns(netns_path, proc_path)
	if err != nil {
		return fmt.Errorf("make_netns")
	}

	//set veth
	if err := Make_attach_veth(*netns, fd, "br0"); err != nil {
		return fmt.Errorf("make_attach_veth")
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("ERROR", err)
		log.Println("delete netns")
		if err := syscall.Unmount(netns_path, syscall.MNT_DETACH); err != nil {
			return fmt.Errorf("unmount old root dir %w", err)
		}
		if err := os.Remove(netns_path); err != nil {
			return fmt.Errorf("rm %s: %w", netns_path, err)
		}
		os.Exit(1)
	}

	//delete netns
	log.Println("delete netns")
	if err := syscall.Unmount(netns_path, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root dir %w", err)
	}
	if err := os.Remove(netns_path); err != nil {
		return fmt.Errorf("rm %s: %w", netns_path, err)
	}

	return nil
}

func child() error {

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	eth := fs.String("n", "go_container", "string flag")
	ip_address := fs.String("i", "10.0.0.2/24", "string flag")
	cg := fs.String("c", "go-container-cgroupv2", "cgroup name flag")
	fs.Parse(os.Args[2:])

	//set hostname
	log.Println("set hostname")
	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("Setting hostname failed: %w", err)
	}

	//mount /proc
	log.Println("mount /proc")
	if err := syscall.Mount("proc", "/newroot/proc", "proc", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("Proc mount failed: %w", err)
	}

	if err := syscall.Mount("/dev", "/newroot/dev", "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting /dev: %w", err)
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

	if err := Make_register_cgroup(*cg, res); err != nil {
		return err
	}
	if err := syscall.Mount("sysfs", "/newroot/sys", "sysfs", syscall.MS_NOEXEC|syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("bind mounting /sys: %w", err)
	}

	if err := syscall.Mount("/sys/fs/cgroup/"+*cg, "/newroot/sys/fs/cgroup", "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mounting"+*cg+": %w", err)
	}

	//pivot root
	log.Println("prepare Rootfs")
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("prepare Rootfs: %w", err)
	}

	log.Println("bind mount /newroot")
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

	//prepare network from args
	log.Println("setup network")

	// setup network
	if err := Network_setup(*eth, *ip_address); err != nil {
		return err
	}
	Args := fs.Args()
	cmd := exec.Command(Args[0], Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
	return nil
}
