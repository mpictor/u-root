// Copyright 2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/u-root/u-root/pkg/testutil"
	"testing"
)

func TestCksum(t *testing.T) {
	var testMatrix = []struct {
		data  []byte
		cksum uint64
	}{
		{[]byte("abcdef\n"), 3512391007},
		{[]byte("pqra\n"), 1063566492},
	}

	for _, testData := range testMatrix {
		if testData.cksum != calculateCksum(testData.data) {
			t.Errorf("Cksum verification failed. (Expected: %lu, Received: %lu)", testData.cksum, calculateCksum(testData.data))
		}
	}
}

func TestMain(m *testing.M) {
	testutil.Run(m, main)
}
