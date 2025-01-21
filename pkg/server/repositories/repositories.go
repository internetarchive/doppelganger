package repositories

import (
	"fmt"
	"log/slog"

	"github.com/gocql/gocql"
	"github.com/internetarchive/doppelganger/pkg/server/config"
	"github.com/scylladb/gocqlx/v3"
	"github.com/scylladb/gocqlx/v3/table"
)

var (
	scyllaSession gocqlx.Session
	scyllaTable   *table.Table
)

// Init initializes the ScyllaDB keyspace and table, then sets up the session.
func Init(cfg *config.Config) (err error) {
	// Ensure the keyspace exists
	if err := ensureKeyspaceExists(cfg.DB.ScyllaHosts, cfg.DB.ScyllaReplicationClass, int32(cfg.DB.ScyllaReplicationFactor), cfg.DB.ScyllaKeyspace); err != nil {
		return err
	}

	// Setup the ScyllaDB connection
	cluster := gocql.NewCluster(cfg.DB.ScyllaHosts...)
	cluster.Keyspace = cfg.DB.ScyllaKeyspace
	cluster.Consistency = gocql.Quorum

	// Enable token aware host selection policy
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

	// Init ScyllaDB session
	scyllaSession, err = gocqlx.WrapSession(cluster.CreateSession())
	if err != nil {
		return err
	}

	// Init (create if not exists) the records table
	if err := initScyllaTable(); err != nil {
		return err
	}

	return nil
}

func ensureKeyspaceExists(scyllaHosts []string, replicationClass string, replicationFactor int32, keyspace string) (err error) {
	// Init ScyllaDB cluster
	cluster := gocql.NewCluster(scyllaHosts...)
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	cluster.NumConns = 4096

	// Init ScyllaDB session
	session, err := gocqlx.WrapSession(cluster.CreateSession())
	if err != nil {
		return err
	}
	defer session.Close()

	// Make sure the Keyspace exists
	err = session.ExecStmt(fmt.Sprintf(`
        CREATE KEYSPACE IF NOT EXISTS %s
        WITH replication = {
            'class': '%s',
            'replication_factor': %d
        }
    `, keyspace, replicationClass, replicationFactor))
	if err != nil {
		slog.Error("error when creating keyspace", slog.String("error", err.Error()))
		return err
	}

	return nil
}

func initScyllaTable() (err error) {
	// Create the table
	if err := scyllaSession.Query(`CREATE TABLE IF NOT EXISTS records (
            id text PRIMARY KEY,
            date text,
            uri text
        )`, []string{}).Exec(); err != nil {
		slog.Error("error when creating table", slog.String("error", err.Error()))
		return err
	}

	// Create the table object
	scyllaTable = table.New(table.Metadata{
		Name: "records",
		Columns: []string{
			"id",
			"date",
			"uri",
		},
	})

	return nil
}
