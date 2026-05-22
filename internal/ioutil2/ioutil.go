// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Vendored from github.com/vuvuzela/internal/ioutil2 (upstream is an
// internal/ package and therefore unimportable from this module).

// Package ioutil2 provides extra functionality along similar lines to io/ioutil.
package ioutil2

import (
	"io/ioutil"
	"os"
	"path"
)

// WriteFileAtomic writes a file to temp and atomically renames it when
// everything else succeeds.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir, name := path.Split(filename)
	f, err := ioutil.TempFile(dir, name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if permErr := os.Chmod(f.Name(), perm); err == nil {
		err = permErr
	}
	if err == nil {
		err = os.Rename(f.Name(), filename)
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}
