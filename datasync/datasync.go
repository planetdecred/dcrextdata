package datasync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/planetdecred/dcrextdata/app/helpers"
)

var coordinator *SyncCoordinator

func NewCoordinator(isEnabled bool, period int) *SyncCoordinator {
	coordinator = &SyncCoordinator{
		period:    period,
		instances: []instance{}, syncers: map[string]Syncer{}, isEnabled: isEnabled, syncersKeys: map[int]string{},
	}
	return coordinator
}

func (s *SyncCoordinator) AddSyncer(tableName string, syncer Syncer) {
	s.syncers[tableName] = syncer
	s.syncersKeys[len(s.syncersKeys)] = tableName
}

func (s *SyncCoordinator) AddSource(url string, store Store, database string) {
	s.instances = append(s.instances, instance{
		store:    store,
		url:      url,
		database: database,
	})
}

func (s *SyncCoordinator) Syncer(tableName string) (Syncer, bool) {
	syncer, found := s.syncers[tableName]
	return syncer, found
}

func (s *SyncCoordinator) StartSyncing(ctx context.Context) {
	runSyncers := func() {
		for _, source := range s.instances {
			for i := 0; i <= len(s.syncersKeys); i++ {
				tableName := s.syncersKeys[i]
				syncer, found := s.syncers[tableName]
				if !found {
					return
				}

				format := "Syncing external %s for %s on %s"
				url := fmt.Sprintf("%s/api/sync/%s", source.url, tableName)
				log.Infof(format, source.database, tableName, url)

				err := s.sync(ctx, source, tableName, syncer)
				if err != nil {
					log.Error(err)
				}

				if err != nil {
					log.Error(err)
				}
			}
		}
	}

	runSyncers()

	ticker := time.NewTicker(time.Duration(s.period) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infof("Stopping sync coordinators")
			return
		case <-ticker.C:
			runSyncers()
		}
	}
}

func (s *SyncCoordinator) sync(ctx context.Context, source instance, tableName string, syncer Syncer) error {
	startTime := helpers.NowUTC()
	skip := 0
	take := 1000
	lastEntry, err := syncer.LastEntry(ctx, source.store)
	if err != nil {
		return fmt.Errorf("error in fetching sync history, %s", err.Error())
	}

	for {
		retries := 0
		url := fmt.Sprintf("%s/api/sync/%s?last=%s&skip=%d&take=%d", strings.TrimSuffix(source.url, "/"),
			tableName, lastEntry, skip, take)
		log.Infof("Syncing %s data from %s", tableName, url)
		var result *Result
		for {
			result, err = syncer.Collect(ctx, url)
			retries++
			if err == nil || retries >= 3 {
				break
			}
			log.Errorf("Sync %s data from %s failed, %s. Retrying...", tableName, url, err.Error())
		}
		if err != nil {
			return fmt.Errorf("error in fetching data for %s, %s", url, err.Error())
		}

		if !result.Success {
			return fmt.Errorf("sync error, %s", result.Message)
		}

		if result.Records == nil {
			if result.TotalCount == 0 {
				return nil
			}
			duration := helpers.NowUTC().Sub(startTime).Seconds()
			log.Infof("Synced %d %s records from %s in %.3f seconds", result.TotalCount, tableName,
				source.url, math.Abs(duration))
			return nil
		}

		syncer.Append(ctx, source.store, result.Records)
		skip += take
	}
}

func RegisteredSources() ([]string, error) {
	if coordinator == nil {
		return nil, errors.New("syncer not initialized")
	}

	var sources = make([]string, len(coordinator.instances))
	for i, s := range coordinator.instances {
		sources[i] = s.database
	}

	return sources, nil
}

func Retrieve(ctx context.Context, tableName string, last string, skip, take int) (*Result, error) {
	log.Infof("Sync request received for %s, last: %d, start: %s", tableName, skip, take)
	if coordinator == nil {
		return nil, errors.New("syncer not initialized")
	}

	if !coordinator.isEnabled {
		return nil, ErrSyncDisabled
	}

	syncer, found := coordinator.syncers[tableName]
	if !found {
		log.Infof("Invalid data type in sync request, %s", tableName)
		return nil, errors.New("syncer not found for " + tableName)
	}

	return syncer.Retrieve(ctx, last, skip, take)
}

func DecodeSyncObj(obj interface{}, receiver interface{}) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, receiver)
	return err
}
