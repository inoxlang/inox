package inoxd

import (
	"os"
	"path/filepath"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	CGROUPV2_PATH = "/sys/fs/cgroup/"
)

func getCgroupMode() (cgroups.CGMode, string) {
	mode := cgroups.Mode()
	modeName := "unavailable"
	switch mode {
	case cgroups.Legacy:
		modeName = "legacy"
	case cgroups.Hybrid:
		modeName = "hybrid"
	case cgroups.Unified:
		modeName = "unified"
	}

	return mode, modeName
}

// createInoxCgroup creates the inox cgroup, it's a subgroup of the unit group (inox.service).
func createInoxCgroup(logger zerolog.Logger) bool {

	pid := os.Getpid()

	unitGroupPath, err := cgroup2.PidGroupPath(pid)
	if err != nil {
		logger.Err(err).Send()
		return false
	}

	fullUnitGroupPath := filepath.Join(CGROUPV2_PATH, unitGroupPath)

	logger.Info().Msg("unit cgroup = " + fullUnitGroupPath)

	//the inox group is a subgroup of the unit group (inox.service).
	inoxGroupPath := filepath.Join(unitGroupPath, "inox")
	fullInoxGroupPath := filepath.Join(fullUnitGroupPath, "inox")

	logger.Info().Msg("mkdir " + inoxGroupPath)

	if err != nil {
		logger.Err(err).Send()
		return false
	}

	//we create the inox subgroup.
	err = os.Mkdir(fullInoxGroupPath, 0o770)
	if err != nil {
		logger.Err(err).Send()
		return false

	}

	inoxController, err := cgroup2.Load(inoxGroupPath)

	if err != nil {
		logger.Err(err).Send()
		return false

	}

	//We move the current process in the inox group.
	//This is done first because the 'no internal processes rule' states that a cgroup can't both
	//(1) have member processes, and (2) distribute resources into child cgroups—that is, have a
	//nonempty cgroup.subtree_control file.

	err = inoxController.AddProc(uint64(pid))
	if err != nil {
		logger.Err(err).Send()
		return false

	}

	//enable all systemd-supported controllers in all subgroups.
	{
		f, err := os.OpenFile(fullUnitGroupPath+"/cgroup.subtree_control", os.O_WRONLY, 0)
		if err != nil {
			logger.Err(err).Send()
			return false
		}

		_, err = f.Write([]byte("+cpuset +cpu +io +memory +pids"))

		if err != nil {
			logger.Err(err).Send()
			f.Close()
			return false

		}
		f.Close()
	}

	err = inoxController.Update(&cgroup2.Resources{
		Memory: &cgroup2.Memory{
			Max: utils.New[int64](1_000_000_000),
		},
	})

	if err != nil {
		logger.Err(err).Send()
		return false

	}

	return true
}
