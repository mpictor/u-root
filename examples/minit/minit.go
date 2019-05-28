// Copyright 2012-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a basic init script.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/u-root/u-root/cmds/elvish/program"
	"github.com/u-root/u-root/pkg/gpt"
	"github.com/u-root/u-root/pkg/kexec"
	"github.com/u-root/u-root/pkg/mount"
	"golang.org/x/sys/unix"
)

const (
	// /dev/nvme0n1: PTUUID="e6bb521c-a495-4d06-ab6a-d94b1c07bdc9" PTTYPE="gpt"
	// /dev/nvme0n1p2: LABEL="nvme" UUID="63047a3d-996f-4be9-84ff-9351b3d4306f" TYPE="ext4"
	//                 PARTLABEL="nvme" PARTUUID="ec02ad2a-caeb-44de-9fc7-1b4b5358faf2"
	PTUUID    = "e6bb521c-a495-4d06-ab6a-d94b1c07bdc9"
	PARTLABEL = "nvme"
	PARTUUID  = "ec02ad2a-caeb-44de-9fc7-1b4b5358faf2"
	newroot   = "/newroot"
)

func main() {
	id := flag.Bool("id", false, "dump config and exit")
	d := flag.Bool("d", false, "quiet")
	flag.Parse()
	if *id {
		fmt.Printf("%s:\nptuuid = %s\nlabel = %s\npartuuid = %s\n", os.Args[0], PTUUID, PARTLABEL, PARTUUID)
		return
	}
	if *d {
		fmt.Printf("quiet mode (ignored)\n")
	}
	devs := getBlockDevs()
	tgt := findPartByUUIDs(devs, PTUUID, PARTUUID)
	log.Printf("device: %s", tgt)
	//mount
	err := os.Mkdir(newroot, 0755)
	if err != nil {
		log.Printf("mkdir err %s", err)
	}
	err = mount.Mount(tgt, newroot, "ext4", "", unix.MS_RDONLY)
	if err != nil {
		log.Printf("mount err %s", err)
	}

	//ask
	//kexec or parse grub2
	umount := func() {
		err = mount.Unmount(newroot, false, true)
		if err != nil {
			log.Printf("umount err %s", err)
			return
		}
	}

	kexecBootSymlink([]finalizer{umount})
	log.Print("something went wrong - see logs above. starting shell...")
	os.Exit(program.Main(os.Args))
}

func getBlockDevs() []string {
	files, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		log.Printf("error %s reading block devs", err)
	}
	var devs []string
	for _, f := range files {
		if (f.Mode() | os.ModeSymlink) == 0 {
			continue
		}
		link, err := os.Readlink(f.Name())
		if err != nil {
			log.Printf("error %s reading block link %s", err, f.Name())
		}
		if strings.Contains(link, "/virtual/") {
			continue
		}
		//should only have real block devices now
		devs = append(devs, f.Name())
	}
	return devs
}

func findPartByUUIDs(devs []string, ptuuid, partuuid string) string {
	for _, dev := range devs {
		blk, err := os.OpenFile("/dev/"+dev, syscall.O_DIRECT, 0400)
		if err != nil {
			log.Printf("error %s opening dev %s", err, dev)
			continue
		}
		defer blk.Close()
		table, err := gpt.New(blk)
		if err != nil {
			log.Printf("error %s opening gpt for %s", err, dev)
			continue
		}
		if table.Primary.DiskGUID.String() != PTUUID {
			continue
		}
		for i, p := range table.Primary.Parts {
			if p.PartGUID.String() != PARTUUID {
				continue
			}
			node := fmt.Sprintf("%sp%d", dev, i+1)
			//how to map this partition to a linux block device?
			siz := p.LastLBA - p.FirstLBA
			bs, err := ioutil.ReadFile("/sys/class/block/" + node + "/size")
			if err != nil {
				log.Printf("error %s reading size of %s", err, node)
			}
			s, err := strconv.ParseUint(string(bs), 10, 64)
			if err != nil {
				log.Printf("error %s reading size of %s", err, node)
			}
			if s != siz {
				log.Printf("sizes do not match for p%d: gpt says %d, /sys says %d", i+1, siz, s)
			}
			//probably not this simple
			return "/dev/" + node
		}
	}
	log.Printf("no device matching pt %s / part %s", ptuuid, partuuid)
	return ""
}

func kexecBootSymlink(finalizers []finalizer) {
	//read symlink /boot/kernel
	kpath, err := fp.EvalSymlinks(newroot + "/boot/kernel")
	if err != nil {
		log.Printf("EvalSymlinks err %s", err)
		return
	}
	// buf, err := ioutil.ReadFile(kpath)
	kfile, err := os.Open(kpath)
	if err != nil {
		log.Printf("loading kernel: %s", err)
		return
	}
	// kexecFile(ioutil.NopCloser(bytes.NewReader(buf)))
	kexecFile(kfile, finalizers)
}

type finalizer func()

func kexecFile(kfile *os.File, finalizers []finalizer) {
	err := kexec.FileLoad(kfile, nil, "")
	kfile.Close()
	if err != nil {
		log.Printf("kexec load err %s", err)
		return
	}
	for _, fi := range finalizers {
		fi()
	}
	log.Printf("reboot to new kernel in 2...")
	time.Sleep(time.Second * 2)
	kexec.Reboot()
}
