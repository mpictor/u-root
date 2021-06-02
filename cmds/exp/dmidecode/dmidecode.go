// Copyright 2016-2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"strconv"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/u-root/u-root/pkg/smbios"
)

var (
	flagDumpBin    = flag.String("dump-bin", "", `Do not decode the entries, instead dump the DMI data to a file in binary form. The generated file is suitable to pass to --from-dump later.`)
	flagFromDump   = flag.String("from-dump", "", `Read the DMI data from a binary file previously generated using --dump-bin.`)
	flagType       = flag.StringSliceP("type", "t", nil, `Only  display  the  entries of type TYPE. TYPE can be either a DMI type number, or a comma-separated list of type numbers, or a keyword from the following list: bios, system, baseboard, chassis, processor, memory, cache, connector, slot. If this option is used more than once, the set of displayed entries will be the union of all the given types. If TYPE is not provided or not valid, a list of all valid keywords is printed and dmidecode exits with an error.`)
	flagSysConv    = flag.String("sys-fw-conv", "", `Convert files originally captured from /sys/firmware/dmi/tables into dump files. This is the path to the file 'DMI' from that dir. See also: --sys-fw-ep, --sys-fw-out`)
	flagSysConvEp  = flag.String("sys-fw-ep", "", `Used in conjunction with --sys-fw-conv, this is the path to what was 'smbios-entry-point'. If empty, looks for a file with that name in the same dir as --sys-fw-conv.`)
	flagSysConvOut = flag.String("sys-fw-out", "dmi.raw", "Used in conjunction with --sys-fw-conv, this is the output `path`.")
	// NB: When adding flags, update resetFlags in dmidecode_test.
)

var (
	typeGroups = map[string][]uint8{
		"bios":      {0, 13},
		"system":    {1, 12, 15, 23, 32},
		"baseboard": {2, 10, 41},
		"chassis":   {3},
		"processor": {4},
		"memory":    {5, 6, 16, 17},
		"cache":     {7},
		"connector": {8},
		"slot":      {9},
	}
)

type dmiDecodeError struct {
	error
	code int
}

// parseTypeFilter parses the --type argument(s) and returns a set of types taht should be included.
func parseTypeFilter(typeStrings []string) (map[smbios.TableType]bool, error) {
	types := map[smbios.TableType]bool{}
	for _, ts := range typeStrings {
		if tg, ok := typeGroups[strings.ToLower(ts)]; ok {
			for _, t := range tg {
				types[smbios.TableType(t)] = true
			}
		} else {
			u, err := strconv.ParseUint(ts, 0, 8)
			if err != nil {
				return nil, fmt.Errorf("Invalid type: %s", ts)
			}
			types[smbios.TableType(uint8(u))] = true
		}
	}
	return types, nil
}

func dumpBin(textOut io.Writer, entryData, tableData []byte, fileName string) *dmiDecodeError {
	// Need to rewrite address to be compatible with dmidecode(8).
	e32, e64, err := smbios.ParseEntry(entryData)
	if err != nil {
		return &dmiDecodeError{code: 1, error: fmt.Errorf("error parsing entry point structure: %v", err)}
	}
	var edata []byte
	switch {
	case e32 != nil:
		e32.StructTableAddr = 0x20
		edata, _ = e32.MarshalBinary()
	case e64 != nil:
		e64.StructTableAddr = 0x20
		edata, _ = e64.MarshalBinary()
	}
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return &dmiDecodeError{code: 1, error: fmt.Errorf("error opening file for writing: %v", err)}
	}
	defer f.Close()
	fmt.Fprintf(textOut, "# Writing %d bytes to %s.\n", len(edata), fileName)
	if _, err := f.Write(edata); err != nil {
		return &dmiDecodeError{code: 1, error: fmt.Errorf("error writing entry: %v", err)}
	}
	for i := len(edata); i < 0x20; i++ {
		if _, err := f.Write([]byte{0}); err != nil {
			return &dmiDecodeError{code: 1, error: fmt.Errorf("error writing entry: %v", err)}
		}
	}
	fmt.Fprintf(textOut, "# Writing %d bytes to %s.\n", len(tableData), fileName)
	if _, err := f.Write(tableData); err != nil {
		return &dmiDecodeError{code: 1, error: fmt.Errorf("error writing table data: %v", err)}
	}
	return nil
}

func dmiDecode(textOut io.Writer) *dmiDecodeError {
	typeFilter, err := parseTypeFilter(*flagType)
	if err != nil {
		return &dmiDecodeError{code: 2, error: fmt.Errorf("invalid --type: %v", err)}
	}
	fmt.Fprintf(textOut, "# dmidecode-go\n") // TODO: version.
	entryData, tableData, err := getData(textOut, *flagFromDump, "/sys/firmware/dmi/tables")
	if err != nil {
		return &dmiDecodeError{code: 1, error: fmt.Errorf("error parsing loading data: %v", err)}
	}
	if *flagDumpBin != "" {
		return dumpBin(textOut, entryData, tableData, *flagDumpBin)
	}
	si, err := smbios.ParseInfo(entryData, tableData)
	if err != nil {
		return &dmiDecodeError{code: 1, error: fmt.Errorf("error parsing data: %v", err)}
	}
	if si.Entry64 != nil {
		fmt.Fprintf(textOut, "SMBIOS %d.%d.%d present.\n", si.MajorVersion(), si.MinorVersion(), si.DocRev())
	} else {
		fmt.Fprintf(textOut, "SMBIOS %d.%d present.\n", si.MajorVersion(), si.MinorVersion())
	}
	if si.Entry32 != nil {
		fmt.Fprintf(textOut, "%d structures occupying %d bytes.\n", si.Entry32.NumberOfStructs, si.Entry32.StructTableLength)
	}
	fmt.Fprintf(textOut, "\n")
	for _, t := range si.Tables {
		if len(typeFilter) != 0 && !typeFilter[t.Type] {
			continue
		}
		pt, err := smbios.ParseTypedTable(t)
		if err != nil {
			if err != smbios.ErrUnsupportedTableType {
				fmt.Fprintf(os.Stderr, "%s\n", err)
			}
			// Print as raw table
			pt = t
		}
		fmt.Fprintf(textOut, "%s\n\n", pt)
	}
	return nil
}

//Convert files from the format found in /sys/firmware/dmi/tables into the dmidecode --dump-bin format.
func convertSysFw() error {
	if len(*flagSysConvOut) == 0 {
		return fmt.Errorf("-sys-fw-out cannot be empty")
	}
	tableData, err := ioutil.ReadFile(*flagSysConv)
	if err != nil {
		return err
	}
	epf := *flagSysConvEp
	if len(epf) == 0 {
		epf = fp.Dir(*flagSysConv) + "/smbios_entry_point"
	}
	entryData, err := ioutil.ReadFile(epf)
	if err != nil {
		return err
	}
	e32, e64, err := smbios.ParseEntry(entryData)
	if err != nil {
		return fmt.Errorf("error parsing entry point structure: %v", err)
	}

	out, err := os.Create(*flagSysConvOut)
	if err != nil {
		return err
	}
	defer out.Close()
	var hdr []byte
	if e64 != nil {
		e64.StructTableAddr = 0x20
		hdr, err = e64.MarshalBinary()
		if err != nil {
			return err
		}
	} else {
		e32.StructTableAddr = 0x20
		hdr, err = e32.MarshalBinary()
		if err != nil {
			return err
		}
	}
	_, err = out.Write(hdr)
	if err != nil {
		return err
	}
	if e64 != nil {
		_, err = out.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})
		if err != nil {
			return err
		}
	} else {
		_, err = out.Write([]byte{0})
		if err != nil {
			return err
		}
	}
	_, err = out.Write(tableData)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()
	if len(*flagSysConv) > 0 {
		err := convertSysFw()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
		return
	}
	err := dmiDecode(os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(err.code)
	}
}
