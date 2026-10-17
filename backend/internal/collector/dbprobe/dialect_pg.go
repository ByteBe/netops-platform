package dbprobe

import "fmt"

// PostgreSQL / KingbaseES
var pgDialect = &dialect{
	driver: "pgx",
	buildDSN: func(d Instance) string {
		dbn := d.DBName
		if dbn == "" {
			dbn = "postgres"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			d.Host, d.Port, d.User, d.Password, dbn)
	},
	version:   "SELECT version()",
	status:    "SELECT CASE WHEN pg_is_in_recovery() THEN 'recovery' ELSE 'up' END",
	conn:      "SELECT COUNT(*) FROM pg_database",
	size:      "SELECT pg_database_size(current_database())",
	tables:    "SELECT COUNT(*) FROM pg_tables WHERE schemaname='public'",
	tx:        "SELECT xact_commit+xact_rollback FROM pg_stat_database WHERE datname=current_database()",
	deadlock:  "SELECT deadlocks FROM pg_stat_database WHERE datname=current_database()",
	err:       "SELECT COALESCE(SUM(aborted),0) FROM pg_stat_database",
	uptime:    "SELECT EXTRACT(EPOCH FROM now()-pg_postmaster_start_time())",
	log:       "SELECT setting FROM pg_settings WHERE name='log_directory'",
}
