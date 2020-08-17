# sshdBg


Modified version of the uinit example. Gets an address via dhcp, sets time via ntpdate, then runs sshd on port 22 in the background.

## Nonstandard beaglebone black
Tested on a found antminer beaglebone black, which differs from the standard bbone black. I know the antminer dtb is required; it's possible that other changes (such as to uEnv.txt) may also be needed for standard bbone black's.

## Build
To build for beaglebone-black:

```shell
#!/bin/sh -e

#create an ssh key for the unit
if [[ ! -f id_rsa_bbb.pub ]]; then
  ssh-keygen -f id_rsa_bbb
fi

#note that your id_rsa.pub is copied
GOARCH=arm GOARM=7 u-root \
    -o uroot.cpio \
    -files id_rsa_bbb:/root/.ssh/id_rsa \
    -files id_rsa_bbb.pub:/root/.ssh/id_rsa.pub \
    -files $HOME/.ssh/id_rsa.pub:/root/.ssh/authorized_keys \
    -uinitcmd sshdBg \
    github.com/u-root/u-root/cmds/core/* \
    github.com/u-root/u-root/cmds/exp/* \
    github.com/u-root/u-root/examples/sshdBg

gzip <uroot.cpio >uroot.gz

#mkimage is part of u-boot-tools
mkimage -n uroot -A arm -O linux -T ramdisk -C gzip -d uroot.gz uroot.ub

echo "now copy uroot.ub onto sd card or into flash"
```

## Boot from SD/mmc card
uEnv.txt:
```
rdaddr=0x81000000
optargs=fixrtc
mmcargs=setenv bootargs console=${console} ${optargs}
loadfdt=ext4load mmc ${mmcdev}:2 ${fdtaddr} /boot/am/dtb
loadramdisk=ext4load mmc ${mmcdev}:2 ${rdaddr} /boot/am/uroot.ub
loaduimage=mw.l 4804c134 fe1fffff; if ext4load mmc 0:2 ${loadaddr} /boot/am/kern.uimg; then mw.l 4804c194 01200000; echo Booting from external microSD...; setenv mmcdev 0; else setenv mmcdev 1; if test $mmc0 = 1; then setenv mmcroot /dev/mmcblk1p2 rw; fi; ext4load mmc 1:2 ${loadaddr} /boot/am/kern.uimg && mw.l 4804c194 00c00000; echo Booting from internal eMMC...; fi
mmcboot=run mmcargs; bootm ${loadaddr} ${rdaddr} ${fdtaddr}
uenvcmd=i2c mw 0x24 1 0x3e; run findfdt; if test $board_name = A335BNLT; then setenv mmcdev 1; mmc dev ${mmcdev}; if mmc rescan; then setenv mmc1 1; else setenv mmc1 0; fi; fi; setenv mmcdev 0; mmc dev ${mmcdev}; if mmc rescan; then setenv mmc0 1; else setenv mmc0 0; fi; run loaduimage && run loadramdisk && run loadfdt && run mmcboot
```

You _must_ extract the dtb from nand (`mtdblock6`), as the antminer version of the bbone black differs from the standard version. For u-root, I also extracted the kernel (`mtdblock7`), but the ubuntu kernel also seemed to work as long as the nand dtb was used.

As `uEnv.txt` is set up, it goes on first partition, with other files on 2nd. The other files are all under `/boot/am/*`. Note that if the kernel is missing or not a uImage, u-boot will act as if the sd card isn't inserted and will boot from nand - so double-check.

## NAND

The following is currently untested. JTAG is likely necessary to recover from any mistakes.

### Manual

Write uroot.eb to mtd 8, using the utilities in the antminer fw:

```shell
flash_eraseall /dev/mtd8 >/dev/null 2>&1
nandwrite -p /dev/mtd8 uroot.ub >/dev/null 2>&1
```

### Auto update

The antminer firmware has a mechanism to update using a downloaded file.

#### TODO
Figure out how to create one.

Also figure out how to add an update mechanism to the u-root image so it can update itself.
