package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"

	"github.com/dgraph-io/badger"
	"github.com/planetdecred/dcrextdata/cache"
	"github.com/planetdecred/dcrextdata/netsnapshot"
	"github.com/planetdecred/dcrextdata/postgres/models"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
)

func (pg PgDb) SaveSnapshot(ctx context.Context, snapshot netsnapshot.SnapShot) error {

	goodNode, err := models.Heartbeats(models.HeartbeatWhere.Timestamp.EQ(snapshot.Timestamp)).Count(ctx, pg.db)
	if err != nil {
		return err
	}
	snapshot.ReachableNodeCount = int(goodNode)
	if snapshot.OldestNodeTimestamp == 0 {
		address, oldestTimestamp, err := pg.getOldestNodeTimestamp(ctx, snapshot.Timestamp)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}
		}
		snapshot.OldestNodeTimestamp = oldestTimestamp
		snapshot.OldestNode = address
	}

	avgLatency, err := pg.averageLatencyByTimestamp(ctx, snapshot.Timestamp)
	if err != nil {
		return err
	}
	snapshot.Latency = avgLatency

	existingSnapshot, err := models.FindNetworkSnapshot(ctx, pg.db, snapshot.Timestamp)
	if err == nil {
		existingSnapshot.Height = snapshot.Height
		existingSnapshot.NodeCount = snapshot.NodeCount
		existingSnapshot.ReachableNodes = snapshot.ReachableNodeCount
		existingSnapshot.OldestNode = snapshot.OldestNode
		existingSnapshot.OldestNodeTimestamp = snapshot.OldestNodeTimestamp
		existingSnapshot.Latency = snapshot.Latency
		_, err = existingSnapshot.Update(ctx, pg.db, boil.Infer())
		return err
	}

	snapshotModel := modelFromSnapshot(snapshot)

	if err := snapshotModel.Insert(ctx, pg.db, boil.Infer()); err != nil {
		if !strings.Contains(err.Error(), "unique constraint") { // Ignore duplicate entries
			return err
		}
	}

	return nil
}

func (pg PgDb) FindNetworkSnapshot(ctx context.Context, timestamp int64) (*netsnapshot.SnapShot, error) {
	snapshotModel, err := models.FindNetworkSnapshot(ctx, pg.db, timestamp)
	if err != nil {
		return nil, err
	}
	return modelToSnapshot(snapshotModel), nil
}

func (pg PgDb) PreviousSnapshot(ctx context.Context, timestamp int64) (*netsnapshot.SnapShot, error) {
	snapshotModel, err := models.NetworkSnapshots(
		models.NetworkSnapshotWhere.Timestamp.LT(timestamp),
		qm.OrderBy(fmt.Sprintf("%s DESC", models.NetworkSnapshotColumns.Timestamp)),
		qm.Limit(1),
	).One(ctx, pg.db)

	if err != nil {
		return nil, err
	}

	return modelToSnapshot(snapshotModel), err
}

func (pg PgDb) SnapshotCount(ctx context.Context) (int64, error) {
	return models.NetworkSnapshots().Count(ctx, pg.db)
}

func (pg PgDb) Snapshots(ctx context.Context, offset, limit int, forChart bool) ([]netsnapshot.SnapShot, int64, error) {
	var queries = []qm.QueryMod{
		models.NetworkSnapshotWhere.Height.GT(0),
		qm.Offset(offset),
	}
	if !forChart {
		queries = append(queries, qm.Limit(limit))
		queries = append(queries, qm.OrderBy("timestamp desc"))
	} else {
		queries = append(queries, qm.OrderBy("timestamp"))
	}

	snapshotSlice, err := models.NetworkSnapshots(queries...).All(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	snapshots := make([]netsnapshot.SnapShot, len(snapshotSlice))
	for i, m := range snapshotSlice {
		snapshot := modelToSnapshot(m)
		snapshots[i] = *snapshot
	}

	total, err := models.NetworkSnapshots(models.NetworkSnapshotWhere.Height.GT(0)).Count(ctx, pg.db)
	if err != nil {
		return nil, 0, err
	}

	return snapshots, total, nil
}

func (pg PgDb) SnapshotsByTime(ctx context.Context, startDate int64, pageSize int) ([]netsnapshot.SnapShot, error) {
	var queries = []qm.QueryMod{
		qm.Select(
			models.NetworkSnapshotColumns.Timestamp,
			models.NetworkSnapshotColumns.NodeCount,
			models.NetworkSnapshotColumns.ReachableNodes,
		),
		models.NetworkSnapshotWhere.Height.GT(0),
		models.NetworkSnapshotWhere.Timestamp.GT(startDate),
		qm.OrderBy("timestamp"),
	}

	if pageSize > 0 {
		queries = append(queries, qm.Limit(pageSize))
	}

	snapshotSlice, err := models.NetworkSnapshots(queries...).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	snapshots := make([]netsnapshot.SnapShot, len(snapshotSlice))
	for i, m := range snapshotSlice {
		snapshot := modelToSnapshot(m)
		snapshots[i] = *snapshot
	}

	return snapshots, nil
}

func (pg PgDb) NextSnapshot(ctx context.Context, timestamp int64) (*netsnapshot.SnapShot, error) {
	snapshotModel, err := models.NetworkSnapshots(
		models.NetworkSnapshotWhere.Timestamp.GT(timestamp),
		qm.OrderBy(models.NetworkSnapshotColumns.Timestamp),
		qm.Limit(1),
	).One(ctx, pg.db)

	if err != nil {
		return nil, err
	}

	return modelToSnapshot(snapshotModel), err
}

func modelToSnapshot(snapshotModel *models.NetworkSnapshot) *netsnapshot.SnapShot {
	return &netsnapshot.SnapShot{
		Timestamp:           snapshotModel.Timestamp,
		Height:              snapshotModel.Height,
		NodeCount:           snapshotModel.NodeCount,
		ReachableNodeCount:  snapshotModel.ReachableNodes,
		OldestNode:          snapshotModel.OldestNode,
		OldestNodeTimestamp: snapshotModel.OldestNodeTimestamp,
		Latency:             snapshotModel.Latency,
	}
}

func modelFromSnapshot(snapshot netsnapshot.SnapShot) models.NetworkSnapshot {
	return models.NetworkSnapshot{
		Timestamp:           snapshot.Timestamp,
		Height:              snapshot.Height,
		NodeCount:           snapshot.NodeCount,
		ReachableNodes:      snapshot.ReachableNodeCount,
		OldestNode:          snapshot.OldestNode,
		OldestNodeTimestamp: snapshot.OldestNodeTimestamp,
		Latency:             snapshot.Latency,
	}
}

func (pg PgDb) DeleteSnapshot(ctx context.Context, timestamp int64) {
	snapshot, err := models.FindNetworkSnapshot(ctx, pg.db, timestamp)
	if err == nil {
		_, _ = models.Heartbeats(models.HeartbeatWhere.Timestamp.EQ(timestamp)).DeleteAll(ctx, pg.db)
		_, _ = snapshot.Delete(ctx, pg.db)
	}
}

func (pg PgDb) getOldestNodeTimestamp(ctx context.Context, timestamp int64) (string, int64, error) {
	sql := fmt.Sprintf(`SELECT node.connection_time, node.address from node 
			INNER JOIN heartbeat ON node.address = heartbeat.node_id
		WHERE heartbeat.timestamp = %d ORDER BY node.connection_time DESC LIMIT 1`, timestamp)

	var result struct {
		ConnectionTime null.Int64  `json:"connection_time"`
		Address        null.String `json:"address"`
	}

	err := models.Nodes(qm.SQL(sql)).Bind(ctx, pg.db, &result)
	if err != nil {
		return "", 0, err
	}

	if result.ConnectionTime.Valid {
		return result.Address.String, result.ConnectionTime.Int64, nil
	}

	return "", 0, nil
}

func (pg PgDb) SaveHeartbeat(ctx context.Context, heartbeat netsnapshot.Heartbeat) error {

	heartbeatModel, err := models.Heartbeats(
		models.HeartbeatWhere.NodeID.EQ(heartbeat.Address),
		models.HeartbeatWhere.Timestamp.EQ(heartbeat.Timestamp)).One(ctx, pg.db)

	if err == nil {
		if heartbeat.CurrentHeight > 0 {
			heartbeatModel.CurrentHeight = heartbeat.CurrentHeight
		}

		if heartbeat.Latency > 0 {
			heartbeatModel.Latency = heartbeat.Latency
		}

		if heartbeat.LastSeen > 0 {
			heartbeatModel.LastSeen = heartbeat.LastSeen
		}

		if _, err = heartbeatModel.Update(ctx, pg.db, boil.Infer()); err != nil {
			return fmt.Errorf("error in saving heartbeatModel, %s", err.Error())
		}
		return nil
	}

	newHeartbeat := models.Heartbeat{
		Timestamp:     heartbeat.Timestamp,
		NodeID:        heartbeat.Address,
		LastSeen:      heartbeat.LastSeen,
		Latency:       heartbeat.Latency,
		CurrentHeight: heartbeat.CurrentHeight,
	}

	if err = newHeartbeat.Insert(ctx, pg.db, boil.Infer()); err != nil {
		return fmt.Errorf("error in saving hearbeat, %s", err.Error())
	}
	return nil
}

func (pg PgDb) AttemptPeer(ctx context.Context, address string, now int64) error {
	var cols = models.M{
		models.NodeColumns.LastAttempt: now,
	}
	_, err := models.Nodes(models.NodeWhere.Address.EQ(address)).UpdateAll(ctx, pg.db, cols)
	return err
}

// RecordNodeConnectionFailure increase the number of failare for the specified node
//
// The node will be marked as dead if the maxAllowedFailure is reached
func (pg PgDb) RecordNodeConnectionFailure(ctx context.Context, address string, maxAllowedFailure int) error {
	node, err := models.Nodes(models.NodeWhere.Address.EQ(address)).One(ctx, pg.db)
	if err != nil {
		if err.Error() == sql.ErrNoRows.Error() {
			return nil
		}
		return err
	}

	node.FailureCount++
	var cols = models.M{
		models.NodeColumns.FailureCount: node.FailureCount,
	}

	if node.FailureCount >= maxAllowedFailure {
		cols[models.NodeColumns.IsDead] = true
	}
	_, err = models.Nodes(models.NodeWhere.Address.EQ(address)).UpdateAll(ctx, pg.db, cols)
	return err
}

func (pg PgDb) NodeExists(ctx context.Context, address string) (bool, error) {
	return models.NodeExists(ctx, pg.db, address)
}

// SaveNode inserts the new node information. The node is marked as alive by default
func (pg PgDb) SaveNode(ctx context.Context, peer netsnapshot.NetworkPeer) error {
	newNode := models.Node{
		Address:         peer.Address,
		IPVersion:       peer.IPVersion,
		Country:         peer.CountryName,
		Region:          peer.RegionName,
		City:            peer.City,
		Zip:             peer.Zip,
		LastAttempt:     peer.LastSeen,
		LastSeen:        peer.LastSeen,
		LastSuccess:     peer.LastSuccess,
		ConnectionTime:  peer.ConnectionTime,
		ProtocolVersion: int(peer.ProtocolVersion),
		UserAgent:       peer.UserAgent,
		Services:        peer.Services,
		StartingHeight:  peer.StartingHeight,
		CurrentHeight:   peer.CurrentHeight,
		IsDead:          false,
	}
	err := newNode.Insert(ctx, pg.db, boil.Infer())
	return err
}

// UpdateNode updates the node information in the database
//
// It reset the node's failure count and marks it as alive
func (pg PgDb) UpdateNode(ctx context.Context, peer netsnapshot.NetworkPeer) error {
	existingNode, err := models.Nodes(models.NodeWhere.Address.EQ(peer.Address)).One(ctx, pg.db)
	if err != nil {
		return fmt.Errorf("update failed: %s", err.Error())
	}

	var cols = models.M{
		models.NodeColumns.LastAttempt:    peer.LastAttempt,
		models.NodeColumns.LastSeen:       peer.LastSeen,
		models.NodeColumns.LastSuccess:    peer.LastSuccess,
		models.NodeColumns.Services:       peer.Services,
		models.NodeColumns.StartingHeight: peer.StartingHeight,
		models.NodeColumns.UserAgent:      peer.UserAgent,
		models.NodeColumns.CurrentHeight:  peer.CurrentHeight,
		models.NodeColumns.IsDead:         false,
		models.NodeColumns.FailureCount:   0,
	}
	if existingNode.ConnectionTime == 0 {
		cols[models.NodeColumns.ConnectionTime] = peer.ConnectionTime
	}
	_, err = models.Nodes(models.NodeWhere.Address.EQ(peer.Address)).UpdateAll(ctx, pg.db, cols)
	return err
}

func (pg PgDb) NetworkPeers(ctx context.Context, timestamp int64, q string, offset int, limit int) ([]netsnapshot.NetworkPeer, int64, error) {
	where := fmt.Sprintf("heartbeat.timestamp = %d", timestamp)
	if q != "" {
		where += fmt.Sprintf(" AND (node.address = '%s' OR node.user_agent = '%s' OR node.country = '%s')", q, q, q)
	}

	sql := `SELECT node.address, node.country, node.last_seen, node.connection_time, node.protocol_version,
			node.user_agent, node.starting_height, node.current_height, node.services FROM heartbeat 
			INNER JOIN node on node.address = heartbeat.node_id WHERE ` + where +
		fmt.Sprintf(" ORDER BY node.last_seen DESC LIMIT %d OFFSET %d", limit, offset)

	var peerSlice models.NodeSlice
	err := models.NewQuery(qm.SQL(sql)).Bind(ctx, pg.db, &peerSlice)
	if err != nil {
		return nil, 0, fmt.Errorf("error %s, on query %s", err.Error(), sql)
	}

	var peers []netsnapshot.NetworkPeer
	for _, node := range peerSlice {
		peer := netsnapshot.NetworkPeer{
			Address:         node.Address,
			LastSeen:        node.LastSeen,
			ConnectionTime:  node.ConnectionTime,
			ProtocolVersion: uint32(node.ProtocolVersion),
			UserAgent:       node.UserAgent,
			StartingHeight:  node.StartingHeight,
			CurrentHeight:   node.CurrentHeight,
			Services:        node.Services,
			IsDead:          node.IsDead,
		}

		peer.IPInfo = netsnapshot.IPInfo{
			CountryName: node.Country,
			RegionName:  node.Region,
			City:        node.City,
			Zip:         node.Zip,
		}
		peers = append(peers, peer)
	}

	sql = "SELECT COUNT(heartbeat.node_id) as total FROM heartbeat INNER JOIN node on node.address = heartbeat.node_id WHERE " + where
	var countResult struct{ Total int64 }
	err = models.NewQuery(qm.SQL(sql)).Bind(ctx, pg.db, &countResult)
	if err != nil {
		return nil, 0, err
	}

	return peers, countResult.Total, nil
}

func (pg PgDb) GetAvailableNodes(ctx context.Context) ([]net.IP, error) {
	peerSlice, err := models.Nodes(models.NodeWhere.IsDead.EQ(false), qm.Select(models.NodeColumns.Address)).All(ctx, pg.db)
	if err != nil {
		return nil, err
	}

	var peers = make([]net.IP, 0, len(peerSlice))
	for _, node := range peerSlice {
		peer := net.ParseIP(node.Address)
		peers = append(peers, peer)
	}

	return peers, nil
}

func (pg PgDb) NetworkPeer(ctx context.Context, address string) (*netsnapshot.NetworkPeer, error) {
	node, err := models.FindNode(ctx, pg.db, address)
	if err != nil {
		return nil, err
	}

	return networkPeerFromModel(node), nil
}

func networkPeerFromModel(nodeModel *models.Node) *netsnapshot.NetworkPeer {
	peer := &netsnapshot.NetworkPeer{
		Address:         nodeModel.Address,
		LastSeen:        nodeModel.LastSeen,
		ConnectionTime:  nodeModel.ConnectionTime,
		ProtocolVersion: uint32(nodeModel.ProtocolVersion),
		UserAgent:       nodeModel.UserAgent,
		StartingHeight:  nodeModel.StartingHeight,
		CurrentHeight:   nodeModel.CurrentHeight,
		Services:        nodeModel.Services,
		IsDead:          nodeModel.IsDead,
	}

	peer.IPInfo = netsnapshot.IPInfo{
		CountryName: nodeModel.Country,
		RegionName:  nodeModel.Region,
		City:        nodeModel.City,
		Zip:         nodeModel.Zip,
	}

	return peer
}

func (pg PgDb) AverageLatency(ctx context.Context, address string) (int, error) {
	heartbeats, err := models.Heartbeats(models.HeartbeatWhere.NodeID.EQ(address),
		models.HeartbeatWhere.Latency.GT(0),
		qm.Select(models.HeartbeatColumns.Latency)).All(ctx, pg.db)
	if err != nil {
		if err.Error() == sql.ErrNoRows.Error() {
			return 0, nil
		}
		return 0, err
	}

	if len(heartbeats) == 0 {
		return 0, nil
	}

	var total int
	for _, h := range heartbeats {
		total += h.Latency
	}

	return total / len(heartbeats), nil
}

func (pg PgDb) averageLatencyByTimestamp(ctx context.Context, timestamp int64) (int, error) {
	heartbeats, err := models.Heartbeats(models.HeartbeatWhere.Timestamp.EQ(timestamp),
		models.HeartbeatWhere.Latency.GT(0),
		qm.Select(models.HeartbeatColumns.Latency)).All(ctx, pg.db)
	if err != nil {
		if err.Error() == sql.ErrNoRows.Error() {
			return 0, nil
		}
		return 0, err
	}

	if len(heartbeats) == 0 {
		return 0, nil
	}

	var total int
	for _, h := range heartbeats {
		total += h.Latency
	}

	return total / len(heartbeats), nil
}

func (pg PgDb) GetIPLocation(ctx context.Context, ip string) (string, int, error) {
	node, err := models.Nodes(
		models.NodeWhere.Address.EQ(ip),
	).One(ctx, pg.db)
	if err != nil {
		return "", -1, err
	}

	return node.Country, node.IPVersion, nil
}

func (pg PgDb) TotalPeerCount(ctx context.Context, timestamp int64) (int64, error) {
	return models.Heartbeats(models.HeartbeatWhere.Timestamp.EQ(timestamp)).Count(ctx, pg.db)
}

func (pg PgDb) SeenNodesByTimestamp(ctx context.Context) ([]netsnapshot.NodeCount, error) {
	var result []netsnapshot.NodeCount
	err := models.NewQuery(
		qm.SQL("SELECT heartbeat.timestamp, COUNT(*) FROM heartbeat group by heartbeat.timestamp order by timestamp"),
	).Bind(ctx, pg.db, &result)
	return result, err
}

func (pg PgDb) PeerCountByUserAgents(ctx context.Context, sources string, offset, limit int) ([]netsnapshot.UserAgentInfo, int64, error) {

	where := ""
	if len(strings.Trim(sources, "")) > 0 {
		sourceList := strings.Split(sources, "|")
		sources = fmt.Sprintf("'%s'", strings.Join(sourceList, "','"))
		sources = strings.ReplaceAll(sources, "Unknown", "")
		where = fmt.Sprintf("WHERE node.user_agent IN (%s) ", sources)
	}

	sql := `SELECT network_snapshot.timestamp, node.user_agent, COUNT(node.user_agent) AS number FROM network_snapshot
		INNER JOIN heartbeat ON heartbeat.timestamp = network_snapshot.timestamp
		INNER JOIN node ON node.address = heartbeat.node_id ` + where +
		`GROUP BY network_snapshot.timestamp, node.user_agent
		ORDER BY network_snapshot.timestamp, number DESC`

	var result []struct {
		Timestamp int64  `json:"timestamp"`
		UserAgent string `json:"user_agent"`
		Number    int64  `json:"number"`
	}

	err := models.Nodes(qm.SQL(sql)).Bind(ctx, pg.db, &result)
	if err != nil {
		return nil, 0, err
	}

	count := len(result)

	if limit > -1 {
		sql += fmt.Sprintf(" OFFSET %d LIMIT %d", offset, limit)
		result = nil
		err = models.Heartbeats(qm.SQL(sql)).Bind(ctx, pg.db, &result)
		if err != nil {
			return nil, 0, err
		}
	}

	var total int64
	for _, item := range result {
		total += item.Number
	}

	userAgents := make([]netsnapshot.UserAgentInfo, len(result))
	for i, item := range result {
		userAgent := item.UserAgent
		if strings.Trim(userAgent, " ") == "" {
			userAgent = "Unknown"
		}
		userAgents[i] = netsnapshot.UserAgentInfo{
			UserAgent: userAgent,
			Nodes:     item.Number,
			Timestamp: item.Timestamp,
		}
	}

	return userAgents, int64(count), nil
}

func (pg PgDb) peerCountByUserAgentsByTime(ctx context.Context, startDate uint64, endDate uint64, sources ...string) ([]netsnapshot.UserAgentInfo, error) {

	where := fmt.Sprintf(" WHERE network_snapshot.timestamp > %d ", startDate)
	if endDate > 0 {
		where += fmt.Sprintf(" AND network_snapshot.timestamp <= %d ", endDate)
	}

	if len(sources) > 0 {
		sourceStr := fmt.Sprintf("'%s'", strings.Join(sources, "','"))
		sourceStr = strings.ReplaceAll(sourceStr, "Unknown", "")
		where += fmt.Sprintf(" AND node.user_agent IN (%s) ", sourceStr)
	}

	sql := `SELECT network_snapshot.timestamp, node.user_agent, COUNT(node.user_agent) AS nodes FROM network_snapshot
		INNER JOIN heartbeat ON heartbeat.timestamp = network_snapshot.timestamp
		INNER JOIN node ON node.address = heartbeat.node_id` + where +
		`GROUP BY network_snapshot.timestamp, node.user_agent
		ORDER BY network_snapshot.timestamp, nodes DESC`

	var result []netsnapshot.UserAgentInfo

	err := models.Nodes(qm.SQL(sql)).Bind(ctx, pg.db, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (pg PgDb) PeerCountByCountries(ctx context.Context, sources string, offset, limit int) ([]netsnapshot.CountryInfo, int64, error) {

	where := ""
	if len(strings.Trim(sources, "")) > 0 {
		sourceList := strings.Split(sources, "|")
		sources = fmt.Sprintf("'%s'", strings.Join(sourceList, "','"))
		sources = strings.ReplaceAll(sources, "Unknown", "")
		where = fmt.Sprintf("WHERE node.country IN (%s) ", sources)
	}

	sql := `SELECT network_snapshot.timestamp, node.country, COUNT(node.country) AS number FROM network_snapshot
		INNER JOIN heartbeat ON heartbeat.timestamp = network_snapshot.timestamp
		INNER JOIN node ON node.address = heartbeat.node_id ` + where +
		`GROUP BY network_snapshot.timestamp, node.country
		ORDER BY network_snapshot.timestamp, number DESC`

	var result []struct {
		Timestamp int64  `json:"timestamp"`
		Country   string `json:"country"`
		Number    int64  `json:"number"`
	}

	err := models.Heartbeats(qm.SQL(sql)).Bind(ctx, pg.db, &result)
	if err != nil {
		return nil, 0, err
	}

	count := len(result)

	if limit != -1 {
		result = nil
		sql += fmt.Sprintf(" OFFSET %d LIMIT %d", offset, limit)
		err = models.Heartbeats(qm.SQL(sql)).Bind(ctx, pg.db, &result)
		if err != nil {
			return nil, 0, err
		}
	}

	countries := make([]netsnapshot.CountryInfo, len(result))

	for i, item := range result {
		country := item.Country
		if strings.Trim(country, " ") == "" {
			country = "Unknown"
		}
		countries[i] = netsnapshot.CountryInfo{
			Country:   country,
			Nodes:     item.Number,
			Timestamp: item.Timestamp,
		}
	}

	return countries, int64(count), nil
}

func (pg PgDb) peerCountByCountriesByTime(ctx context.Context, startDate uint64, endDate uint64, sources ...string) ([]netsnapshot.CountryInfo, error) {

	where := fmt.Sprintf(" WHERE network_snapshot.timestamp > %d ", startDate)
	if endDate > 0 {
		where += fmt.Sprintf(" AND network_snapshot.timestamp <= %d ", endDate)
	}
	if len(sources) > 0 {
		sourceStr := fmt.Sprintf("'%s'", strings.Join(sources, "','"))
		sourceStr = strings.ReplaceAll(sourceStr, "Unknown", "")
		where += fmt.Sprintf(" AND node.country IN (%s) ", sourceStr)
	}

	sql := `SELECT network_snapshot.timestamp, node.country, COUNT(node.country) AS nodes FROM network_snapshot
		INNER JOIN heartbeat ON heartbeat.timestamp = network_snapshot.timestamp
		INNER JOIN node ON node.address = heartbeat.node_id ` + where +
		`GROUP BY network_snapshot.timestamp, node.country
		ORDER BY network_snapshot.timestamp, nodes DESC`

	var result []netsnapshot.CountryInfo

	err := models.Heartbeats(qm.SQL(sql)).Bind(ctx, pg.db, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (pg PgDb) PeerCountByIPVersion(ctx context.Context, timestamp int64, iPVersion int) (int64, error) {
	var result struct{ Total int64 }
	err := models.NewQuery(
		qm.Select("COUNT(h.node_id) as total"),
		qm.From(fmt.Sprintf("%s as h", models.TableNames.Heartbeat)),
		qm.InnerJoin(fmt.Sprintf("%s as n on n.address = h.node_id", models.TableNames.Node)),
		qm.Where("h.timestamp = ? and n.ip_version = ?", timestamp, iPVersion)).Bind(ctx, pg.db, &result)

	return result.Total, err
}

func (pg PgDb) LastSnapshotTime(ctx context.Context) (timestamp int64) {
	rows := pg.db.QueryRow("SELECT timestamp FROM network_snapshot WHERE height > 0 ORDER BY timestamp DESC LIMIT 1")
	_ = rows.Scan(&timestamp)
	return
}

func (pg PgDb) LastSnapshot(ctx context.Context) (*netsnapshot.SnapShot, error) {
	return pg.FindNetworkSnapshot(ctx, pg.LastSnapshotTime(ctx))
}

func (pg PgDb) AllNodeVersions(ctx context.Context) (versions []string, err error) {
	nodes, err := models.Nodes(qm.Select("distinct user_agent"), qm.OrderBy(
		fmt.Sprintf("%s desc", models.NodeColumns.UserAgent),
	)).All(ctx, pg.db)
	for _, node := range nodes {
		versions = append(versions, node.UserAgent)
	}
	return
}

func (pg PgDb) AllNodeContries(ctx context.Context) (countries []string, err error) {
	nodes, err := models.Nodes(qm.Select("distinct country"), qm.OrderBy(models.NodeColumns.Country)).All(ctx, pg.db)
	for _, node := range nodes {
		countries = append(countries, node.Country)
	}
	return
}

type snapshotSet struct {
	time           cache.ChartUints
	nodes          cache.ChartUints
	reachableNodes cache.ChartUints
	locations      map[string]cache.ChartUints
	locationDates  cache.ChartUints
	versions       map[string]cache.ChartUints
	versionDates   cache.ChartUints
}

type snapshotTableSet struct {
	locations []netsnapshot.CountryInfo
	versoins  []netsnapshot.UserAgentInfo
}

func (pg *PgDb) fetchNetworkSnapshotChart(ctx context.Context, charts *cache.Manager, page int) (interface{}, func(), bool, error) {
	var set = snapshotSet{
		locations: make(map[string]cache.ChartUints),
		versions:  make(map[string]cache.ChartUints),
	}

	startDate := charts.SnapshotTip()
	if startDate == 0 {
		rows := pg.db.QueryRow(fmt.Sprintf("SELECT %s FROM %s ORDER BY %s LIMIT 1",
			models.NetworkSnapshotColumns.Timestamp,
			models.TableNames.NetworkSnapshot, models.NetworkSnapshotColumns.Timestamp))
		err := rows.Scan(&startDate)
		if err != nil {
			if err.Error() != sql.ErrNoRows.Error() {
				log.Errorf("Error in getting min Network Snapshot date - %s", err.Error())
			}
		}
	}

	pageSize := 100000000
	done := true
	result, err := pg.SnapshotsByTime(ctx, int64(startDate), pageSize)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			return nil, func() {}, false, err
		}
	}
	if len(result) > 0 {
		done = false
	}

	var endDate uint64
	for _, rec := range result {
		if uint64(rec.Timestamp) > endDate {
			endDate = uint64(rec.Timestamp)
		}
		set.time = append(set.time, uint64(rec.Timestamp))
		set.nodes = append(set.nodes, uint64(rec.NodeCount))
		set.reachableNodes = append(set.reachableNodes, uint64(rec.ReachableNodeCount))
	}

	// Locations
	var datesMap = map[int64]struct{}{}
	var allDates []int64
	var countryMap = map[string]struct{}{}
	var allCountries []string
	var dateCountryCount = make(map[int64]map[string]int64)

	locations, err := pg.peerCountByCountriesByTime(ctx, startDate, endDate)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			return nil, func() {}, false, err
		}
	}

	if len(locations) > 0 {
		done = false
	}

	for _, item := range locations {
		if _, exists := datesMap[item.Timestamp]; !exists {
			datesMap[item.Timestamp] = struct{}{}
			allDates = append(allDates, item.Timestamp)
		}

		if _, exists := dateCountryCount[item.Timestamp]; !exists {
			dateCountryCount[item.Timestamp] = make(map[string]int64)
		}

		if _, exists := countryMap[item.Country]; !exists {
			countryMap[item.Country] = struct{}{}
			allCountries = append(allCountries, item.Country)
		}
		dateCountryCount[item.Timestamp][item.Country] = item.Nodes
	}

	for _, d := range allDates {
		for _, c := range allCountries {
			rec := dateCountryCount[d][c]
			if record, found := set.locations[c]; found {
				set.locations[c] = append(record, uint64(rec))
			} else {
				set.locations[c] = cache.ChartUints{uint64(rec)}
			}
		}
		set.locationDates = append(set.locationDates, uint64(d))
	}

	// versions
	datesMap = map[int64]struct{}{}
	allDates = []int64{}
	var userAgentMap = map[string]struct{}{}
	var allUserAgents []string
	var dateUserAgentCount = make(map[int64]map[string]int64)

	userAgents, err := pg.peerCountByUserAgentsByTime(ctx, startDate, endDate)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			return nil, func() {}, false, err
		}
	}

	if len(userAgents) > 0 {
		done = false
	}

	for _, item := range userAgents {
		if _, exists := datesMap[item.Timestamp]; !exists {
			datesMap[item.Timestamp] = struct{}{}
			allDates = append(allDates, item.Timestamp)
		}

		if _, exists := dateUserAgentCount[item.Timestamp]; !exists {
			dateUserAgentCount[item.Timestamp] = make(map[string]int64)
		}

		if _, exists := userAgentMap[item.UserAgent]; !exists {
			userAgentMap[item.UserAgent] = struct{}{}
			allUserAgents = append(allUserAgents, item.UserAgent)
		}
		dateUserAgentCount[item.Timestamp][item.UserAgent] = item.Nodes
	}

	for _, d := range allDates {
		for _, c := range allUserAgents {
			rec := dateUserAgentCount[d][c]
			if record, found := set.versions[c]; found {
				set.versions[c] = append(record, uint64(rec))
			} else {
				set.versions[c] = cache.ChartUints{uint64(rec)}
			}
		}
		set.versionDates = append(set.versionDates, uint64(d))
	}

	fmt.Println(done)
	return set, func() {}, true, nil
}

func appendSnapshotChart(charts *cache.Manager, data interface{}) error {
	var tickSets = data.(snapshotSet)
	if len(tickSets.time) == 0 {
		return nil
	}

	key := fmt.Sprintf("%s-%s", cache.Snapshot, cache.TimeAxis)
	if err := charts.AppendChartUintsAxis(key, tickSets.time); err != nil {
		return err
	}

	key = fmt.Sprintf("%s-%s", cache.Snapshot, cache.SnapshotNodes)
	if err := charts.AppendChartUintsAxis(key, tickSets.nodes); err != nil {
		return err
	}

	key = fmt.Sprintf("%s-%s", cache.Snapshot, cache.SnapshotReachableNodes)
	if err := charts.AppendChartUintsAxis(key, tickSets.reachableNodes); err != nil {
		return err
	}

	keyExists := func(arr []string, key string) bool {
		for _, item := range arr {
			if item == key {
				return true
			}
		}
		return false
	}

	key = fmt.Sprintf("%s-%s-%s", cache.Snapshot, cache.SnapshotLocations, cache.TimeAxis)
	if err := charts.AppendChartUintsAxis(key, tickSets.locationDates); err != nil {
		return err
	}

	for country, record := range tickSets.locations {
		if country == "" {
			country = "Unknown"
		}
		if !keyExists(charts.NodeLocations, country) {
			charts.NodeLocations = append(charts.NodeLocations, country)
		}
		key = fmt.Sprintf("%s-%s-%s", cache.Snapshot, cache.SnapshotLocations, country)
		if err := charts.AppendChartUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	key = fmt.Sprintf("%s-%s-%s", cache.Snapshot, cache.SnapshotNodeVersions, cache.TimeAxis)
	if err := charts.AppendChartUintsAxis(key, tickSets.versionDates); err != nil {
		return err
	}

	for userAgent, record := range tickSets.versions {
		if userAgent == "" {
			userAgent = "Unknown"
		}
		if !keyExists(charts.NodeVersion, userAgent) {
			charts.NodeVersion = append(charts.NodeVersion, userAgent)
		}
		key = fmt.Sprintf("%s-%s-%s", cache.Snapshot, cache.SnapshotNodeVersions, userAgent)
		if err := charts.AppendChartUintsAxis(key,
			record); err != nil {
			return err
		}
	}

	return nil
}

func (pg *PgDb) fetchNetworkSnapshotTable(ctx context.Context, charts *cache.Manager, page int) (interface{}, func(), bool, error) {
	var set snapshotTableSet

	// Locations
	locations, err := pg.peerCountByCountriesByTime(ctx, 0, 0)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			return nil, func() {}, false, err
		}
	}
	set.locations = locations

	// versions
	userAgents, err := pg.peerCountByUserAgentsByTime(ctx, 0, 0)
	if err != nil {
		if err.Error() != sql.ErrNoRows.Error() {
			return nil, func() {}, false, err
		}
	}
	set.versoins = userAgents
	return set, func() {}, true, nil
}

func appendSnapshotTable(charts *cache.Manager, data interface{}) error {
	set := data.(snapshotTableSet)

	txn := charts.DB.NewTransaction(true)
	defer txn.Discard()

	var oldLocations []netsnapshot.CountryInfo
	locationKey := fmt.Sprintf("%s-%s-*", cache.Snapshot, cache.SnapshotLocations)
	if err := charts.ReadValTx(locationKey, &oldLocations, txn); err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	if err := charts.SaveValTx(locationKey, set.locations, txn); err != nil {
		return err
	}

	var oldVersions []netsnapshot.UserAgentInfo
	versionKey := fmt.Sprintf("%s-%s-*", cache.Snapshot, cache.SnapshotNodeVersions)
	if err := charts.ReadValTx(versionKey, &oldVersions, txn); err != nil && err != badger.ErrKeyNotFound {
		return err
	}
	if err := charts.SaveValTx(versionKey, set.versoins, txn); err != nil {
		return err
	}

	if err := txn.Commit(); err != nil {
		return err
	}
	return nil
}

func (pg *PgDb) fetchEncodeSnapshotChart(ctx context.Context, charts *cache.Manager, dataType, _ string, binString string, extras ...string) ([]byte, error) {
	switch dataType {
	case string(cache.SnapshotNodes):
		return pg.fetchEncodeSnapshotNodesChart(ctx, charts)
	case string(cache.SnapshotNodeVersions):
		return pg.fetchEncodeSnapshotNodeVersionsChart(ctx, charts, extras...)
	case string(cache.SnapshotLocations):
		return pg.fetchEncodeSnapshotLocationsChart(ctx, charts, extras...)
	default:
		return nil, cache.UnknownChartErr
	}
}

func (pg *PgDb) fetchEncodeSnapshotNodesChart(ctx context.Context, charts *cache.Manager) ([]byte, error) {
	result, err := pg.SnapshotsByTime(ctx, 0, 0)
	if err != nil {
		return nil, err
	}

	var time, nodes, reachableNodes cache.ChartUints
	for _, rec := range result {
		time = append(time, uint64(rec.Timestamp))
		nodes = append(nodes, uint64(rec.NodeCount))
		reachableNodes = append(reachableNodes, uint64(rec.ReachableNodeCount))
	}

	return charts.Encode(nil, time, nodes, reachableNodes)
}

func (pg *PgDb) fetchEncodeSnapshotNodeVersionsChart(ctx context.Context, charts *cache.Manager, userAgentsArg ...string) ([]byte, error) {
	datesMap := map[int64]struct{}{}
	allDates := cache.ChartUints{}
	var userAgentMap = map[string]struct{}{}
	var allUserAgents []string
	var dateUserAgentCount = make(map[uint64]map[string]int64)

	userAgents, err := pg.peerCountByUserAgentsByTime(ctx, 0, 0, userAgentsArg...)
	if err != nil {
		return nil, err
	}

	for _, item := range userAgents {
		if _, exists := datesMap[item.Timestamp]; !exists {
			datesMap[item.Timestamp] = struct{}{}
			allDates = append(allDates, uint64(item.Timestamp))
		}

		if _, exists := dateUserAgentCount[uint64(item.Timestamp)]; !exists {
			dateUserAgentCount[uint64(item.Timestamp)] = make(map[string]int64)
		}

		if _, exists := userAgentMap[item.UserAgent]; !exists {
			userAgentMap[item.UserAgent] = struct{}{}
			allUserAgents = append(allUserAgents, item.UserAgent)
		}
		dateUserAgentCount[uint64(item.Timestamp)][item.UserAgent] = item.Nodes
	}

	if len(userAgentsArg) == 0 {
		userAgentsArg = allUserAgents
	}

	versions := map[string]cache.ChartUints{}
	for _, d := range allDates {
		for _, c := range userAgentsArg {
			rec := dateUserAgentCount[d][c]
			if record, found := versions[c]; found {
				versions[c] = append(record, uint64(rec))
			} else {
				versions[c] = cache.ChartUints{uint64(rec)}
			}
		}
	}
	recs := []cache.Lengther{allDates}
	for _, r := range versions {
		recs = append(recs, r)
	}

	return charts.Encode(nil, recs...)
}

func (pg *PgDb) fetchEncodeSnapshotLocationsChart(ctx context.Context, charts *cache.Manager, countriesArg ...string) ([]byte, error) {
	var datesMap = map[int64]struct{}{}
	var allDates cache.ChartUints
	var countryMap = map[string]struct{}{}
	var allCountries []string
	var dateCountryCount = make(map[uint64]map[string]int64)

	locations, err := pg.peerCountByCountriesByTime(ctx, 0, 0, countriesArg...)
	if err != nil {
		return nil, err
	}

	for _, item := range locations {
		if _, exists := datesMap[item.Timestamp]; !exists {
			datesMap[item.Timestamp] = struct{}{}
			allDates = append(allDates, uint64(item.Timestamp))
		}

		if _, exists := dateCountryCount[uint64(item.Timestamp)]; !exists {
			dateCountryCount[uint64(item.Timestamp)] = make(map[string]int64)
		}

		if _, exists := countryMap[item.Country]; !exists {
			countryMap[item.Country] = struct{}{}
			allCountries = append(allCountries, item.Country)
		}
		dateCountryCount[uint64(item.Timestamp)][item.Country] = item.Nodes
	}

	if len(countriesArg) == 0 {
		countriesArg = allCountries
	}

	var locationSet = map[string]cache.ChartUints{}
	for _, d := range allDates {
		for _, c := range countriesArg {
			rec := dateCountryCount[d][c]
			if record, found := locationSet[c]; found {
				locationSet[c] = append(record, uint64(rec))
			} else {
				locationSet[c] = cache.ChartUints{uint64(rec)}
			}
		}
	}

	recs := []cache.Lengther{allDates}
	for _, r := range locationSet {
		recs = append(recs, r)
	}

	return charts.Encode(nil, recs...)
}
