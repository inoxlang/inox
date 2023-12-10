package metricsperf

import (
	"bytes"
	"errors"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MIN_SAVE_PERIOD = 20 * time.Second
)

type PerfDataCollectionConfig struct {
	// this duration is truncated to the second
	ProfileSavePeriod time.Duration

	Bucket s3_ns.OpenBucketWithCredentialsInput
}

// StartPeriodicPerfProfilesCollection starts a goroutine that collect several profiles: CPU, MEM, Mutex, goroutine stack trace.
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
	stopMemProfilingChan := make(chan struct{})
	stopMutexProfilingChan := make(chan struct{})
	stopGoutineStackTraceProfilingChan := make(chan struct{})
	lastCpuProfileSaveAck := make(chan struct{}, 1)
	lastMemProfileSaveAck := make(chan struct{}, 1)
	lastMutexProfileSaveAck := make(chan struct{}, 1)
	lastGoroutineStackProfileSaveAck := make(chan struct{}, 1)

	childCtx := ctx.BoundChild()
	state := ctx.GetClosestState()
	logger := state.Logger

	//create the main profiling goroutine, it manages the CPU profiles and:
	// - the memory profiling goroutine
	// - the mutex profiling goroutine

	go func() {
		defer utils.Recover()
		defer childCtx.CancelGracefully()

		buff := bytes.NewBuffer(nil)
		pprof.StartCPUProfile(buff)

		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for range ticker.C {
			pprof.StopCPUProfile()
			cpuProfiles <- buff

			select {
			case <-stopChan:
				stopMemProfilingChan <- struct{}{}
				stopMutexProfilingChan <- struct{}{}
				stopGoutineStackTraceProfilingChan <- struct{}{}
				close(cpuProfiles)
				<-lastCpuProfileSaveAck
				<-lastMemProfileSaveAck
				<-lastMutexProfileSaveAck
				<-lastGoroutineStackProfileSaveAck
				return
			default:
			}

			buff = bytes.NewBuffer(nil)
			pprof.StartCPUProfile(buff)
		}
	}()

	//create a goroutine recording a memory profile every conf.Period & saving it to a S3 bucket

	go func() {
		defer utils.Recover()
		defer close(lastMemProfileSaveAck)

		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for t := range ticker.C {
			buff := bytes.NewBuffer(nil)
			err := pprof.WriteHeapProfile(buff)
			if err != nil {
				logger.Err(err)
			}

			date := t.UTC().Format(time.RFC3339)
			key := "mem-" + date + ".pprof"

			_, err = s3Client.PutObject(childCtx, key, buff)
			if err != nil {
				logger.Err(err)
				continue
			}

			select {
			case <-stopMemProfilingChan:
				<-lastMemProfileSaveAck
				return
			default:
			}

		}
	}()

	//create a goroutine recording a mutex profile every conf.Period & saving it to a S3 bucket

	go func() {
		defer utils.Recover()
		defer close(lastMutexProfileSaveAck)

		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for t := range ticker.C {
			buff := bytes.NewBuffer(nil)
			date := t.UTC().Format(time.RFC3339)
			profile := pprof.Lookup("mutex")
			profile.WriteTo(buff, 0)

			runtime.SetMutexProfileFraction(1)
			key := "mutex-" + date + ".pprof"

			_, err := s3Client.PutObject(childCtx, key, buff)
			if err != nil {
				logger.Err(err)
				continue
			}

			select {
			case <-stopMutexProfilingChan:
				<-lastMutexProfileSaveAck
				return
			default:
			}
		}
	}()

	//create a goroutine recording a goroutine stack trace profile every conf.Period & saving it to a S3 bucket

	go func() {
		defer utils.Recover()
		defer close(lastGoroutineStackProfileSaveAck)

		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for t := range ticker.C {
			buff := bytes.NewBuffer(nil)
			date := t.UTC().Format(time.RFC3339)
			profile := pprof.Lookup("goroutine")
			profile.WriteTo(buff, 2)

			key := "goroutine-trace-" + date + ".pprof"

			_, err := s3Client.PutObject(childCtx, key, buff)
			if err != nil {
				logger.Err(err)
				continue
			}

			select {
			case <-stopGoutineStackTraceProfilingChan:
				<-lastGoroutineStackProfileSaveAck
				return
			default:
			}
		}
	}()

	//create a goroutine saving the CPU profiles to a S3 bucket

	go func() {
		defer utils.Recover()
		defer close(lastCpuProfileSaveAck)

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
