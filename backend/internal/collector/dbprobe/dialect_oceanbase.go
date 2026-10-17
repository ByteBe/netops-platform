package dbprobe

import "fmt"

// OceanBase（MySQL 兼容模式）
var oceanbaseDialect = &dialect{
	driver: "mysql",
	buildDSN: func(d Instance) string {
		dbn := d.DBName
		if dbn == "" {
			dbn = "oceanbase"
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=20s&readTimeout=20s&writeTimeout=20s",
			d.User, d.Password, d.Host, d.Port, dbn)
	},
	version:  "SELECT version()",
	status:   "SELECT 'up'",
	conn:     "SELECT COUNT(*) FROM information_schema.schemata",
	size:     "SELECT COALESCE(SUM(data_length+index_length),0) FROM information_schema.tables WHERE table_schema NOT IN ('mysql','sys','information_schema','performance_schema')",
	tables:   "SELECT COUNT(*) FROM information_schema.tables",
	tx:       "SHOW GLOBAL STATUS LIKE 'Com_commit'",
	deadlock: "SHOW GLOBAL STATUS LIKE 'Innodb_deadlocks'",
	err:      "SELECT 0",
	uptime:   "SHOW GLOBAL STATUS LIKE 'Uptime'",
	log:      "SHOW VARIABLES LIKE 'log_error'",
}
