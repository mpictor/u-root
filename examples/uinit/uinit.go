// Copyright 2012-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a basic init script.
package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/u-root/u-root/pkg/gpt"
	"github.com/u-root/u-root/pkg/kexec"
	"github.com/u-root/u-root/pkg/mount"
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
	devs := getBlockDevs()
	tgt := findPartByUUIDs(devs, PTUUID, PARTUUID)
	log.Print("device: %s", tgt)
	//mount
	err := os.Mkdir(newroot, 0755)
	if err != nil {
		log.Printf("mkdir err %s", err)
	}
	err = mount.Mount(tgt, newroot, "ext4", data, flags)
	if err != nil {
		log.Printf("mount err %s", err)
	}

	//ask
	//kexec or parse grub2
	//kexecBootSymlink()

	log.Print("Uinit Done!")
}

func getBlockDevs() []string {
	files, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		log.Printf("error %s reading block devs", err)
	}
	var devs []string
	for _, f := range files {
		if !f.Mode() | os.ModeSymlink {
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
		table := gpt.New(blk)
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
			s, err := strconv.ParseInt(bs, 10, 64)
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
}

func kexecBootSymlink() {
	//read symlink /boot/kernel
	kpath, err := fp.EvalSymlinks(newroot + "/boot/kernel")
	if err != nil {
		log.Printf("EvalSymlinks err %s", err)
		return
	}
	buf, err := ioutil.ReadFile(kpath)
	if err != nil {
		log.Printf("loading kernel: %s", err)
		return
	}
	err = mount.Unmount(newroot, false, false)
	if err != nil {
		log.Printf("umount err %s", err)
		return
	}
	kexecFile(bytes.NewReader(buf))
}

func kexecFile(kfile io.ReadCloser) {
	defer kfile.Close()
	err = kexec.FileLoad(kfile, nil, "")
	kfile.Close()
	if err != nil {
		log.Printf("kexec load err %s", err)
		return
	}
	log.Printf("reboot to new kernel in 2...")
	time.Sleep(time.Second / 2)
	kexec.Reboot()
}

