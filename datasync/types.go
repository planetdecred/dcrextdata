package datasync

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSyncDisabled = errors.New("data sharing is disabled on this instance")
)

type SyncCoordinator struct {
	period      int
	syncers     map[string]Syncer
	syncersKeys map[int]string
	instances   []instance
	isEnabled   bool
}

type instance struct {
	database string
	store    Store
	url      string
}

type Syncer struct {
	LastEntry func(ctx context.Context, db Store) (string, error)
	Collect   func(ctx context.Context, url string) (*Result, error)
	Retrieve  func(ctx context.Context, last string, skip, take int) (*Result, error)
	Append    func(ctx context.Context, db Store, data interface{})
}

type Store interface {
	TableNames() []string
	LastEntry(ctx context.Context, tableName string, receiver interface{}) error

	LastExchangeEntryID() (id int64)
	SaveExchangeFromSync(ctx context.Context, exchange interface{}) error
	LastExchangeTickEntryTime() (time time.Time)
	SaveExchangeTickFromSync(ctx context.Context, tick interface{}) error

	StoreMempoolFromSync(ctx context.Context, mempoolDto interface{}) error
	SaveBlockFromSync(ctx context.Context, block interface{}) error
	SaveVoteFromSync(ctx context.Context, vote interface{}) error
	UpdatePropagationData(ctx context.Context) error

	AddPowDataFromSync(ctx context.Context, data interface{}) error

	AddVspSourceFromSync(ctx context.Context, vspDto interface{}) error
	AddVspTicksFromSync(ctx context.Context, tick VSPTickSyncDto) error
}

type VSPTickSyncDto struct {
	ID               int       `json:"id"`
	VSPID            int       `json:"vspid"`
	VSP              string    `json:"vsp"`
	Immature         int       `json:"immature"`
	Live             int       `json:"live"`
	Voted            int       `json:"voted"`
	Missed           int       `json:"missed"`
	PoolFees         float64   `json:"pool_fees"`
	ProportionLive   float64   `json:"proportion_live"`
	ProportionMissed float64   `json:"proportion_missed"`
	UserCount        int       `json:"user_count"`
	UsersActive      int       `json:"users_active"`
	Time             time.Time `json:"time"`
}

type Request struct {
	Table        string
	Date         time.Time
	MaxSkipCount int
	MaxTakeCount int
}

type Result struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message,omitempty"`
	Records    interface{} `json:"records,omitempty"`
	TotalCount int64       `json:"total_count,omitempty"`
}
