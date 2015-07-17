package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/lxc/lxd/shared"
)

func removeContainerPath(d *Daemon, name string) error {
	cpath := shared.VarPath("lxc", name)

	backingFs, err := shared.GetFilesystem(cpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		shared.Debugf("Error cleaning up %s: %s\n", cpath, err)
		return err
	}

	vgname, vgnameIsSet, err := getServerConfigValue(d, "core.lvm_vg_name")
	if err != nil {
		return fmt.Errorf("Error checking server config: %v", err)
	}

	if vgnameIsSet {
		err = shared.LVMRemoveLV(vgname, name)
		if err != nil {
			return fmt.Errorf("failed to remove deleted container LV: %v", err)
		}

	} else if backingFs == "btrfs" && btrfsIsSubvolume(cpath) {
		if err := btrfsDeleteSubvol(cpath); err != nil {
			return err
		}
	}

	err = os.RemoveAll(cpath)
	if err != nil {
		shared.Debugf("Error cleaning up %s: %s\n", cpath, err)
		return err
	}

	return nil
}

func removeContainer(d *Daemon, name string) error {
	if err := containerDeleteSnapshots(d, name); err != nil {
		return err
	}

	if err := removeContainerPath(d, name); err != nil {
		return err
	}

	if err := dbRemoveContainer(d, name); err != nil {
		return err
	}

	return nil
}

func containerDelete(d *Daemon, r *http.Request) Response {
	name := mux.Vars(r)["name"]
	_, err := dbGetContainerID(d.db, name)
	if err != nil {
		return SmartError(err)
	}

	rmct := func() error {
		return removeContainer(d, name)
	}

	return AsyncResponse(shared.OperationWrap(rmct), nil)
}
