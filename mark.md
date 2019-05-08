
# compile
u-root -build=bb

# what to modify (and how)

- locate kernel on root volume and kexec it
  - locate root by uuid
  - how to choose kernel? symlink?
  - args? or rely on those embedded?

# final

- embed in kernel with minimal options (for fast boot)
  - needed:
    - efi
    - nvme
    - usb
    - usb keyboard
    - no sound
    - no network (??)
    - no video
      - except efifb?
    - udev support


# plan
- modify examples/uinit
- search for uuid and/or disk label
- use pkg/diskboot to parse grub file
- kexec

- additional build options? single binary? don't minimize - may need rescue shell at some point
