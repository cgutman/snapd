// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package backend

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ubuntu-core/snappy/logger"
	"github.com/ubuntu-core/snappy/progress"
	"github.com/ubuntu-core/snappy/snap"
	// XXX: eventually not needed
	"github.com/ubuntu-core/snappy/snappy"
	"github.com/ubuntu-core/snappy/wrappers"
)

func updateCurrentSymlinks(info *snap.Info) error {
	mountDir := info.MountDir()

	currentActiveSymlink := filepath.Join(mountDir, "..", "current")
	if err := os.Remove(currentActiveSymlink); err != nil && !os.IsNotExist(err) {
		logger.Noticef("Failed to remove %q: %v", currentActiveSymlink, err)
	}

	dataDir := info.DataDir()
	dbase := filepath.Dir(dataDir)
	currentDataSymlink := filepath.Join(dbase, "current")
	if err := os.Remove(currentDataSymlink); err != nil && !os.IsNotExist(err) {
		logger.Noticef("Failed to remove %q: %v", currentDataSymlink, err)
	}

	// symlink is relative to parent dir
	if err := os.Symlink(filepath.Base(mountDir), currentActiveSymlink); err != nil {
		return err
	}

	if err := os.MkdirAll(info.DataDir(), 0755); err != nil {
		return err
	}

	// XXX: should migrate to a different package/form
	if err := snappy.SetNextBoot(info); err != nil {
		return err
	}

	return os.Symlink(filepath.Base(dataDir), currentDataSymlink)
}

// LinkSnap makes the snap available by generating wrappers and setting the current symlinks
func (b Backend) LinkSnap(info *snap.Info) error {
	if err := generateWrappers(info); err != nil {
		return err
	}

	return updateCurrentSymlinks(info)
}

func generateWrappers(s *snap.Info) error {
	// add the CLI apps from the snap.yaml
	if err := wrappers.AddSnapBinaries(s); err != nil {
		return err
	}
	// add the daemons from the snap.yaml
	if err := wrappers.AddSnapServices(s, &progress.NullProgress{}); err != nil {
		return err
	}
	// add the desktop files
	if err := wrappers.AddSnapDesktopFiles(s); err != nil {
		return err
	}

	return nil
}

func removeGeneratedWrappers(s *snap.Info, meter progress.Meter) error {
	err1 := wrappers.RemoveSnapBinaries(s)
	if err1 != nil {
		logger.Noticef("Failed to remove binaries for %q: %v", s.Name(), err1)
	}

	err2 := wrappers.RemoveSnapServices(s, meter)
	if err2 != nil {
		logger.Noticef("Failed to remove services for %q: %v", s.Name(), err2)
	}

	err3 := wrappers.RemoveSnapDesktopFiles(s)
	if err3 != nil {
		logger.Noticef("Failed to remove desktop files for %q: %v", s.Name(), err3)
	}

	return firstErr(err1, err2, err3)
}

// UnlinkSnap makes the snap unavailable to the system removing wrappers and symlinks.
func (b Backend) UnlinkSnap(info *snap.Info, meter progress.Meter) error {
	// remove generated services, binaries etc
	err1 := removeGeneratedWrappers(info, meter)

	// and finally remove current symlinks
	err2 := removeCurrentSymlinks(info)

	// FIXME: aggregate errors instead
	return firstErr(err1, err2)
}

func removeCurrentSymlinks(info snap.PlaceInfo) error {
	var err1, err2 error

	// the snap "current" symlink
	currentActiveSymlink := filepath.Join(info.MountDir(), "..", "current")
	err1 = os.Remove(currentActiveSymlink)
	if err1 != nil && !os.IsNotExist(err1) {
		logger.Noticef("Failed to remove %q: %v", currentActiveSymlink, err1)
	} else {
		err1 = nil
	}

	// the data "current" symlink
	currentDataSymlink := filepath.Join(filepath.Dir(info.DataDir()), "current")
	err2 = os.Remove(currentDataSymlink)
	if err2 != nil && !os.IsNotExist(err2) {
		logger.Noticef("Failed to remove %q: %v", currentDataSymlink, err2)
	} else {
		err2 = nil
	}

	if err1 != nil && err2 != nil {
		return fmt.Errorf("cannot remove snap current symlink: %v and %v", err1, err2)
	} else if err1 != nil {
		return fmt.Errorf("cannot remove snap current symlink: %v", err1)
	} else if err2 != nil {
		return fmt.Errorf("cannot remove snap current symlink: %v", err2)
	}

	return nil
}
