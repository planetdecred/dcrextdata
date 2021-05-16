package postgres

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	migrate "github.com/rubenv/sql-migrate"
)

const (
	migrateUp   = "up"
	migrateDown = "down"
	migrateNew  = "new"

	migrateDir = "postgres/migrations"

	lastExchangeTickEntryTime = `SELECT time FROM exchange_tick ORDER BY time DESC LIMIT 1`
	lastExchangeEntryID       = `SELECT id FROM exchange ORDER BY id DESC LIMIT 1`
	lastVspTickEntryTime      = `SELECT time FROM vsp_tick ORDER BY time DESC LIMIT 1`
	lastPowEntryTimeBySource  = `SELECT time FROM pow_data WHERE source=$1 ORDER BY time DESC LIMIT 1`
	lastPowEntryTime          = `SELECT time FROM pow_data ORDER BY time DESC LIMIT 1`
	lastMempoolBlockHeight    = `SELECT last_block_height FROM mempool ORDER BY last_block_height DESC LIMIT 1`
	lastMempoolEntryTime      = `SELECT time FROM mempool ORDER BY time DESC LIMIT 1`
	lastCommStatEntryTime     = `SELECT date FROM reddit ORDER BY date DESC LIMIT 1`
)

var (
	ErrInvalidMigrateConvention = fmt.Errorf("Invalid migrate convention")
)

func (pg *PgDb) DropCacheTables() error {
	// vsp_tick
	if err := pg.dropTable("vsp_tick_bin"); err != nil {
		return err
	}

	// pow_bin
	if err := pg.dropTable("pow_bin"); err != nil {
		return err
	}

	// mempool_bin
	if err := pg.dropTable("mempool_bin"); err != nil {
		return err
	}

	// propagation
	if err := pg.dropTable("propagation"); err != nil {
		return err
	}

	// vote_receive_time_deviation
	if err := pg.dropTable("vote_receive_time_deviation"); err != nil {
		return err
	}

	//network_snapshot_bin
	if err := pg.dropTable("network_snapshot_bin"); err != nil {
		return err
	}

	return nil
}

func (pg *PgDb) dropTable(name string) error {
	log.Tracef("Dropping table %s", name)
	_, err := pg.db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s;`, name))
	return err
}

func (pg *PgDb) dropIndex(name string) error {
	log.Tracef("Dropping table %s", name)
	_, err := pg.db.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS %s;`, name))
	return err
}

func (pg *PgDb) newMigrateFile(message string) error {
	reg, err := regexp.Compile("[^a-zA-Z0-9 ]+")
	if err != nil {
		return err
	}
	message = reg.ReplaceAllString(message, "")
	message = strings.Trim(message, " ")
	message = strings.ReplaceAll(message, " ", "_")
	nowUnix := time.Now().Unix()
	fileName := fmt.Sprintf("%d-%s.sql", nowUnix, message)
	filePath := filepath.Join(migrateDir, fileName)
	content := []byte("\n-- +migrate Up\n\n-- +migrate Down\n\n")
	err = ioutil.WriteFile(filePath, content, 0700)
	if err == nil {
		log.Infof("Created sql migration file: %s", fileName)
	}
	return err
}

func (pg *PgDb) MigrateDatabase(migrateCode string) error {
	migrateInfos := strings.Split(strings.ToLower(migrateCode), ":")
	var migrateAction migrate.MigrationDirection
	switch migrateInfos[0] {
	case migrateUp:
		migrateAction = migrate.Up
	case migrateDown:
		migrateAction = migrate.Down
	case migrateNew:
		if len(migrateInfos) != 2 {
			return fmt.Errorf("Please enter the migration message. ")
		}
		return pg.newMigrateFile(migrateInfos[1])
	default:
		return ErrInvalidMigrateConvention
	}
	log.Infof("Start do migrating action: %s", migrateInfos[0])
	migrations := &migrate.FileMigrationSource{
		Dir: migrateDir,
	}
	var n int
	var err error
	if len(migrateInfos) == 1 {
		n, err = migrate.Exec(pg.db, "postgres", migrations, migrateAction)
	} else if len(migrateInfos) == 2 {
		var limit int
		limit, err = strconv.Atoi(migrateInfos[1])
		if err != nil {
			return ErrInvalidMigrateConvention
		}
		n, err = migrate.ExecMax(pg.db, "postgres", migrations, migrateAction, limit)
	}
	if err != nil {
		return err
	}
	log.Infof("Applied %d migrations!", n)
	return nil
}
