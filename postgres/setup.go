package postgres

import "fmt"

const (
	createExchangeTable = `CREATE TABLE IF NOT EXISTS exchange (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		url TEXT NOT NULL);`

	createExchangeTickTable = `CREATE TABLE IF NOT EXISTS exchange_tick (
		id SERIAL PRIMARY KEY,
		exchange_id INT REFERENCES exchange(id) NOT NULL, 
		interval INT NOT NULL,
		high FLOAT NOT NULL,
		low FLOAT NOT NULL,
		open FLOAT NOT NULL,
		close FLOAT NOT NULL,
		volume FLOAT NOT NULL,
		currency_pair TEXT NOT NULL,
		time TIMESTAMPTZ NOT NULL
	);`

	createExchangeTickIndex = `CREATE UNIQUE INDEX IF NOT EXISTS exchange_tick_idx ON exchange_tick (exchange_id, interval, currency_pair, time);`

	lastExchangeTickEntryTime = `SELECT time FROM exchange_tick ORDER BY time DESC LIMIT 1`

	lastExchangeEntryID = `SELECT id FROM exchange ORDER BY id DESC LIMIT 1`

	createVSPInfoTable = `CREATE TABLE IF NOT EXISTS vsp (
		id SERIAL PRIMARY KEY,
		name TEXT,
		api_enabled BOOLEAN,
		api_versions_supported INT8[],
		network TEXT,
		url TEXT,
		launched TIMESTAMPTZ
	);`

	createVSPTickTable = `CREATE TABLE IF NOT EXISTS vsp_tick (
		id SERIAL PRIMARY KEY,
		vsp_id INT REFERENCES vsp(id) NOT NULL,
		immature INT NOT NULL,
		live INT NOT NULL,
		voted INT NOT NULL,
		missed INT NOT NULL,
		pool_fees FLOAT NOT NULL,
		proportion_live FLOAT NOT NULL,
		proportion_missed FLOAT NOT NULL,
		user_count INT NOT NULL,
		users_active INT NOT NULL,
		time TIMESTAMPTZ NOT NULL
	);`

	createVSPTickIndex = `CREATE UNIQUE INDEX IF NOT EXISTS vsp_tick_idx ON vsp_tick (vsp_id,immature,live,voted,missed,pool_fees,proportion_live,proportion_missed,user_count,users_active, time);`

	lastVspTickEntryTime = `SELECT time FROM vsp_tick ORDER BY time DESC LIMIT 1`

	// PoW table
	createPowDataTable = `CREATE TABLE IF NOT EXISTS pow_data (
 		time INT,
		pool_hashrate VARCHAR(25),
		workers INT,
		coin_price VARCHAR(25),
		btc_price VARCHAR(25),
		source VARCHAR(25),
		PRIMARY KEY (time, source)
	);`

	lastPowEntryTimeBySource = `SELECT time FROM pow_data WHERE source=$1 ORDER BY time DESC LIMIT 1`
	lastPowEntryTime         = `SELECT time FROM pow_data ORDER BY time DESC LIMIT 1`

	createMempoolTable = `CREATE TABLE IF NOT EXISTS mempool (
		time timestamp,
		first_seen_time timestamp,
		number_of_transactions INT,
		voters INT,
		tickets INT,
		revocations INT,
		size INT,
		total_fee FLOAT8,
		total FLOAT8,
		PRIMARY KEY (time)
	);`

	createMempoolDayBinTable = `CREATE TABLE IF NOT EXISTS mempool_bin (
		time timestamp,
		bin VARCHAR(25),
		number_of_transactions INT,
		size INT,
		total_fee FLOAT8,
		PRIMARY KEY (time,bin)
	);`

	lastMempoolBlockHeight = `SELECT last_block_height FROM mempool ORDER BY last_block_height DESC LIMIT 1`
	lastMempoolEntryTime   = `SELECT time FROM mempool ORDER BY time DESC LIMIT 1`

	createBlockTable = `CREATE TABLE IF NOT EXISTS block (
		height INT,
		receive_time timestamp,
		internal_timestamp timestamp,
		hash VARCHAR(512),
		PRIMARY KEY (height)
	);`

	createVoteTable = `CREATE TABLE IF NOT EXISTS vote (
		hash VARCHAR(128),
		voting_on INT8,
		block_hash VARCHAR(128),
		receive_time timestamp,
		block_receive_time timestamp,
		targeted_block_time timestamp,
		validator_id INT,
		validity VARCHAR(128),
		PRIMARY KEY (hash)
	);`

	lastCommStatEntryTime = `SELECT date FROM reddit ORDER BY date DESC LIMIT 1`

	createRedditTable = `CREATE TABLE IF NOT EXISTS reddit (
		date timestamp,
		subreddit VARCHAR(256) NOT NULL,
		subscribers INT NOT NULL,
		active_accounts INT NOT NULL,
		PRIMARY KEY (date)
	);`

	createTwitterTable = `CREATE TABLE IF NOT EXISTS twitter (
		date timestamp,
		handle VARCHAR(256) NOT NULL,
		followers INT NOT NULL,
		PRIMARY KEY (date)
	);`

	createGithubTable = `CREATE TABLE IF NOT EXISTS github (
		date timestamp,
		repository VARCHAR(256) NOT NULL,
		stars INT NOT NULL,
		folks INT NOT NULL,
		PRIMARY KEY (date)
	);`

	createYoutubeTable = `CREATE TABLE IF NOT EXISTS youtube (
		date timestamp,
		subscribers INT NOT NULL,
		view_count INT NOT NULL,
		channel VARCHAR(256) NOT NULL,
		PRIMARY KEY (date)
	);`

	createNetworkSnapshotTable = `CREATE TABLE If NOT EXISTS network_snapshot (
		timestamp INT8 NOT NULL,
		height INT8 NOT NULL,
		node_count INT NOT NULL,
		reachable_nodes INT NOT NULL,
		oldest_node VARCHAR(256) NOT NULL DEFAULT '',
		oldest_node_timestamp INT8 NOT NULL DEFAULT 0,
		latency INT NOT NULL DEFAULT 0,
		PRIMARY KEY (timestamp)
	);`

	createNodeTable = `CREATE TABLE If NOT EXISTS node (
		address VARCHAR(256) NOT NULL PRIMARY KEY,
		ip_version INT NOT NULL,
		country VARCHAR(256) NOT NULL,
		region VARCHAR(256) NOT NULL,
		city VARCHAR(256) NOT NULL,
		zip VARCHAR(256) NOT NULL,
		last_attempt INT8 NOT NULL,
		last_seen INT8 NOT NULL,
		last_success INT8 NOT NULL,
		failure_count INT NOT NULL DEFAULT 0,
		is_dead BOOLEAN NOT NULL,
		connection_time INT8 NOT NULL,
		protocol_version INT NOT NULL,
		user_agent VARCHAR(256) NOT NULL,
		services VARCHAR(256) NOT NULL,
		starting_height INT8 NOT NULL,
		current_height INT8 NOT NULL
	);`

	createHeartbeatTable = `CREATE TABLE If NOT EXISTS heartbeat (
		timestamp INT8 NOT NULL,
		node_id VARCHAR(256) NOT NULL REFERENCES node(address),
		last_seen INT8 NOT NULL,
		latency INT NOT NULL,
		current_height INT8 NOT NULL,
		PRIMARY KEY (timestamp, node_id)
	);`
)

func (pg *PgDb) CreateExchangeTable() error {
	log.Trace("Creating exchange tick table")
	_, err := pg.db.Exec(createExchangeTable)
	return err
}

func (pg *PgDb) ExchangeTableExits() bool {
	exists, _ := pg.tableExists("exchange")
	return exists
}

func (pg *PgDb) CreateExchangeTickTable() error {
	log.Trace("Creating exchange tick table")
	_, err := pg.db.Exec(createExchangeTickTable)
	return err
}

func (pg *PgDb) CreateExchangeTickIndex() error {
	_, err := pg.db.Exec(createExchangeTickIndex)
	return err
}

func (pg *PgDb) ExchangeTickTableExits() bool {
	exists, _ := pg.tableExists("exchange_tick")
	return exists
}

func (pg *PgDb) CreateVSPInfoTables() error {
	_, err := pg.db.Exec(createVSPInfoTable)
	return err
}

func (pg *PgDb) VSPInfoTableExits() bool {
	exists, _ := pg.tableExists("vsp")
	return exists
}

func (pg *PgDb) CreateVSPTickTables() error {
	_, err := pg.db.Exec(createVSPTickTable)
	return err
}

func (pg *PgDb) CreateVSPTickIndex() error {
	_, err := pg.db.Exec(createVSPTickIndex)

	return err
}

func (pg *PgDb) VSPTickTableExits() bool {
	exists, _ := pg.tableExists("vsp_tick")
	return exists
}

func (pg *PgDb) CreatePowDataTable() error {
	_, err := pg.db.Exec(createPowDataTable)
	return err
}

func (pg *PgDb) PowDataTableExits() bool {
	exists, _ := pg.tableExists("pow_data")
	return exists
}

func (pg *PgDb) CreateMempoolDataTable() error {
	_, err := pg.db.Exec(createMempoolTable)
	return err
}

func (pg *PgDb) CreateMempoolDayBinTable() error {
	_, err := pg.db.Exec(createMempoolDayBinTable)
	return err
}

func (pg *PgDb) MempoolDataTableExits() bool {
	exists, _ := pg.tableExists("mempool")
	return exists
}

func (pg *PgDb) MempoolBinDataTableExits() bool {
	exists, _ := pg.tableExists("mempool_bin")
	return exists
}

// block table
func (pg *PgDb) CreateBlockTable() error {
	_, err := pg.db.Exec(createBlockTable)
	return err
}

func (pg *PgDb) BlockTableExits() bool {
	exists, _ := pg.tableExists("block")
	return exists
}

// vote table
func (pg *PgDb) CreateVoteTable() error {
	_, err := pg.db.Exec(createVoteTable)
	return err
}

func (pg *PgDb) VoteTableExits() bool {
	exists, _ := pg.tableExists("vote")
	return exists
}

// reddit table
func (pg *PgDb) CreateRedditTable() error {
	_, err := pg.db.Exec(createRedditTable)
	return err
}

func (pg *PgDb) RedditTableExits() bool {
	exists, _ := pg.tableExists("reddit")
	return exists
}

// twitter table
func (pg *PgDb) CreateTwitterTable() error {
	_, err := pg.db.Exec(createTwitterTable)
	return err
}

func (pg *PgDb) TwitterTableExits() bool {
	exists, _ := pg.tableExists("twitter")
	return exists
}

// youtube table
func (pg *PgDb) CreateGithubTable() error {
	_, err := pg.db.Exec(createGithubTable)
	return err
}

func (pg *PgDb) GithubTableExits() bool {
	exists, _ := pg.tableExists("github")
	return exists
}

// youtube table
func (pg *PgDb) CreateYoutubeTable() error {
	_, err := pg.db.Exec(createYoutubeTable)
	return err
}

func (pg *PgDb) YoutubeTableExits() bool {
	exists, _ := pg.tableExists("youtube")
	return exists
}

// network snapshot
func (pg *PgDb) CreateNetworkSnapshotTable() error {
	_, err := pg.db.Exec(createNetworkSnapshotTable)
	return err
}

func (pg *PgDb) NetworkSnapshotTableExists() bool {
	exists, _ := pg.tableExists("network_snapshot")
	return exists
}

// network node
func (pg *PgDb) CreateNetworkNodeTable() error {
	_, err := pg.db.Exec(createNodeTable)
	return err
}

func (pg *PgDb) NetworkNodeTableExists() bool {
	exists, _ := pg.tableExists("node")
	return exists
}

// network peer
func (pg *PgDb) CreateHeartbeatTable() error {
	_, err := pg.db.Exec(createHeartbeatTable)
	return err
}

func (pg *PgDb) HeartbeatTableExists() bool {
	exists, _ := pg.tableExists("heartbeat")
	return exists
}

func (pg *PgDb) tableExists(name string) (bool, error) {
	rows, err := pg.db.Query(`SELECT relname FROM pg_class WHERE relname = $1`, name)
	if err == nil {
		defer func() {
			if e := rows.Close(); e != nil {
				log.Error("Close of Query failed: ", e)
			}
		}()
		return rows.Next(), nil
	}
	return false, err
}

func (pg *PgDb) DropAllTables() error {
	// vsp_tick
	if err := pg.dropIndex("vsp_tick_idx"); err != nil {
		return err
	}

	if err := pg.dropTable("vsp_tick"); err != nil {
		return err
	}

	// vsp
	if err := pg.dropTable("vsp"); err != nil {
		return err
	}

	// exchange_tick
	if err := pg.dropIndex("exchange_tick_idx"); err != nil {
		return err
	}

	if err := pg.dropTable("exchange_tick"); err != nil {
		return err
	}

	// exchange
	if err := pg.dropTable("exchange"); err != nil {
		return err
	}

	// pow_data
	if err := pg.dropTable("pow_data"); err != nil {
		return err
	}

	// mempool
	if err := pg.dropTable("mempool"); err != nil {
		return err
	}

	// mempool_bin
	if err := pg.dropTable("mempool_bin"); err != nil {
		return err
	}

	// block
	if err := pg.dropTable("block"); err != nil {
		return err
	}

	// vote
	if err := pg.dropTable("vote"); err != nil {
		return err
	}

	// reddit
	if err := pg.dropTable("reddit"); err != nil {
		return err
	}

	// reddit
	if err := pg.dropTable("github"); err != nil {
		return err
	}

	// reddit
	if err := pg.dropTable("twitter"); err != nil {
		return err
	}

	// reddit
	if err := pg.dropTable("youtube"); err != nil {
		return err
	}

	// comm_stat
	if err := pg.dropTable("comm_stat"); err != nil {
		return err
	}

	// network_snapshot
	if err := pg.dropTable("network_snapshot"); err != nil {
		return err
	}

	// heartbeat
	if err := pg.dropTable("heartbeat"); err != nil {
		return err
	}

	// node
	if err := pg.dropTable("node"); err != nil {
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
