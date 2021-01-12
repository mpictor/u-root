// Copyright 2020 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package bzimage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

/*
values from kernel documentation and libmagic src

off val
510 0xAA55
514 HdrS
526	(4 bytes) != 0x0000
526 (2 bytes, little endian) + 0x200 -> start of null-terminated version string
*/

const kverMax = 1024 //arbitrary

var (
	ErrBootSig = errors.New("missing 0x55AA boot sig")
	ErrBadSig  = errors.New("missing kernel header sig")
	ErrBadOff  = errors.New("null version string offset")
	ErrParse   = errors.New("parse error")
)

//Read kernel version string. See also: (*BZImage)Kver()
func KVer(k io.ReadSeeker) (string, error) {
	var buf = make([]byte, kverMax)
	_, err := k.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}
	_, err = k.Read(buf[:530])
	if err != nil {
		return "", err
	}
	if !bytes.Equal(buf[510:512], []byte{0x55, 0xaa}) {
		return "", ErrBootSig
	}
	if string(buf[514:518]) != "HdrS" {
		return "", ErrBadSig
	}
	if bytes.Equal(buf[526:530], []byte{0, 0, 0, 0}) {
		return "", ErrBadOff
	}
	off := int64(binary.LittleEndian.Uint16(buf[526:528])) + 0x200
	_, err = k.Seek(off, io.SeekStart)
	if err != nil {
		return "", err
	}
	if _, err := k.Read(buf[:]); err != nil {
		return "", err
	}
	return nullterm(buf), nil
}

//Read kernel version string. See also: KVer()
func (bz *BzImage) KVer(imgbytes []byte) string {
	if bz.Header.Kveraddr == 0 {
		return ("(unknown)")
	}
	start := uint64(bz.Header.Kveraddr + 0x200)
	return nullterm(imgbytes[start : start+kverMax])
}

//read c string from buffer
func nullterm(buf []byte) string {
	var i int
	var b byte
	for i, b = range buf {
		if b == 0 {
			break
		}
	}
	return string(buf[:i])
}

type KInfo struct {
	//2.6.24.111 (bluebat@linux-vm-os64.site) #606 Mon Apr 14 00:06:11 CEST 2014
	//4.19.16-norm_boot (user@host) #300 SMP Fri Jan 25 16:32:19 UTC 2019
	//   release               (builder)                              version
	//maj.min.patch-localver                                      #buildnum SMP buildtime
	Release, Version string //uname -r, uname -v respectfully
	Builder          string //user@hostname in parenthesis, shown by `file` but not `uname`

	//the following are extracted from Release and Version

	BuildNum        uint64    //#nnn in Version, 300 in example above
	BuildTime       time.Time //from Version
	Maj, Min, Patch uint64    //from Release
	LocalVer        string    //from Release
}

func (l KInfo) Equal(r KInfo) bool {
	return l.Release == r.Release &&
		l.Builder == r.Builder &&
		l.Version == r.Version &&
		l.BuildNum == r.BuildNum &&
		l.BuildTime.Equal(r.BuildTime) &&
		l.Maj == r.Maj &&
		l.Min == r.Min &&
		l.Patch == r.Patch &&
		l.LocalVer == r.LocalVer
}

const layout = "Mon Jan 2 15:04:05 MST 2006"

//Parse output of GetKDesc
func ParseDesc(desc string) (KInfo, error) {
	var ki KInfo

	//first split at #
	split := strings.Split(desc, "#")
	if len(split) != 2 {
		return KInfo{}, fmt.Errorf("%w: %s: wrong number of '#' chars", ErrParse, desc)
	}
	ki.Version = "#" + split[1]

	//now split first part into release and builder
	elements := strings.SplitN(split[0], " ", 2)
	if len(elements) > 2 {
		return KInfo{}, fmt.Errorf("%w: %s: wrong number of spaces in release/builder", ErrParse, desc)
	}
	ki.Release = elements[0]
	if len(elements) == 2 {
		//not sure if this is _always_ present
		ki.Builder = strings.Trim(elements[1], " ()")
	}
	//split build number off version
	elements = strings.SplitN(split[1], " ", 2)
	if len(elements) != 2 {
		return KInfo{}, fmt.Errorf("%w: %s: wrong number of spaces in build/version", ErrParse, desc)
	}
	i, err := strconv.ParseUint(elements[0], 10, 64)
	if err != nil {
		return KInfo{}, fmt.Errorf("%s: bad uint %s: %w", desc, elements[0], err)
	}
	ki.BuildNum = i
	//remove SMP if present
	t := strings.TrimSpace(strings.TrimPrefix(elements[1], "SMP"))
	//parse remainder as time, using reference time
	ki.BuildTime, err = time.Parse(layout, t)
	if err != nil {
		return KInfo{}, fmt.Errorf("%s: bad time %s: %w", desc, t, err)
	}
	elements = strings.Split(ki.Release, ".")
	if len(elements) < 3 {
		return KInfo{}, fmt.Errorf("%w: %s: wrong number of dots in release %s", ErrParse, desc, ki.Release)
	}
	ki.Maj, err = strconv.ParseUint(elements[0], 10, 64)
	if err != nil {
		return KInfo{}, fmt.Errorf("%s: bad uint %s: %w", desc, elements[0], err)
	}
	ki.Min, err = strconv.ParseUint(elements[1], 10, 64)
	if err != nil {
		return KInfo{}, fmt.Errorf("%s: bad uint %s: %w", desc, elements[1], err)
	}
	elem := strings.SplitN(elements[2], "-", 2)
	ki.Patch, err = strconv.ParseUint(elem[0], 10, 64)
	if err != nil {
		return KInfo{}, fmt.Errorf("%s: bad uint %s: %w", desc, elem[0], err)
	}

	elements = strings.SplitN(elements[len(elements)-1], "-", 2)
	if len(elements) > 1 {
		ki.LocalVer = elements[1]
	}
	return ki, nil
}
