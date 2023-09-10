package metricsperf

import (
	"bytes"
	"errors"
	"runtime/pprof"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
)

const (
	MIN_SAVE_PERIOD = 20 * time.Second
)

type PerfDataCollectionConfig struct {
	// this duration is truncated to the second
	ProfileSavePeriod time.Duration

	Bucket s3_ns.OpenBucketWithCredentialsInput
}

// StartPeriodicPerfProfilesCollection starts a goroutine that collect several profiles: CPU, MEM, ..
// Every conf.Period the profiles are saved, stopChan should be written to & stopped by the caller.
func StartPeriodicPerfProfilesCollection(ctx *core.Context, conf PerfDataCollectionConfig) (stopChan chan struct{}, _ error) {

	if conf.ProfileSavePeriod < MIN_SAVE_PERIOD {
		return nil, errors.New("period is missing or is less than " + MIN_SAVE_PERIOD.String())
	}

	period := conf.ProfileSavePeriod.Truncate(time.Second)

	s3Client, err := s3_ns.OpenBucketWithCredentials(ctx, conf.Bucket)

	if err != nil {
		return nil, err
	}

	cpuProfiles := make(chan *bytes.Buffer, 10)
	stopChan = make(chan struct{})
	lastCpuProfileSaveAck := make(chan struct{}, 1)

	//create a goroutine managing the CPU profiles & sending them to cpuProfiles

	go func() {
		buff := bytes.NewBuffer(nil)
		pprof.StartCPUProfile(buff)

		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for range ticker.C {
			pprof.StopCPUProfile()
			cpuProfiles <- buff

			select {
			case <-stopChan:
				close(cpuProfiles)
				<-lastCpuProfileSaveAck
				return
			default:
			}

			buff = bytes.NewBuffer(nil)
			pprof.StartCPUProfile(buff)
		}
	}()

	childCtx := ctx.BoundChild()
	state := ctx.GetClosestState()
	logger := state.Logger

	//create a goroutine saving the CPU profiles to a S3 bucket

	go func() {
		for profile := range cpuProfiles {
			date := time.Now().UTC().Format(time.RFC3339)
			key := "cpu-" + period.String() + "-" + date + ".pprof"

			_, err := s3Client.PutObject(childCtx, key, profile)
			if err != nil {
				logger.Err(err)
				continue
			}
		}
		lastCpuProfileSaveAck <- struct{}{}
	}()

	return stopChan, nil
}
