package groot

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type Cleaner struct {
	storeMeasurer    StoreMeasurer
	garbageCollector GarbageCollector
	locksmith        Locksmith
	metricsEmitter   MetricsEmitter
}

func IamCleaner(locksmith Locksmith, sm StoreMeasurer,
	gc GarbageCollector, metricsEmitter MetricsEmitter,
) *Cleaner {
	return &Cleaner{
		locksmith:        locksmith,
		storeMeasurer:    sm,
		garbageCollector: gc,
		metricsEmitter:   metricsEmitter,
	}
}

func (c *Cleaner) Clean(logger lager.Logger, threshold uint64, keepImages []string) (bool, error) {
	startTime := time.Now()
	defer func() {
		c.metricsEmitter.TryEmitDuration(logger, MetricImageCleanTime, time.Since(startTime))
	}()

	logger = logger.Session("groot-cleaning")
	logger.Info("start")
	defer logger.Info("end")

	if threshold > 0 {
		storeSize, err := c.storeMeasurer.MeasureStore(logger)
		if err != nil {
			return true, err
		}

		if threshold >= storeSize {
			return true, nil
		}
	}

	lockFile, err := c.locksmith.Lock(GlobalLockKey)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := c.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	return false, c.garbageCollector.Collect(logger, keepImages)
}
