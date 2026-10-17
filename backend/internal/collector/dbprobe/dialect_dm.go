package dbprobe

import "fmt"

// 达梦 DM8
var dmDialect = &dialect{
	driver: "dm",
	buildDSN: func(d Instance) string {
		schema := d.DBName
		if schema == "" {
			schema = d.User
		}
		return fmt.Sprintf("dm://%s:%s@%s:%d?schema=%s", d.User, d.Password, d.Host, d.Port, schema)
	},
	version:   "SELECT banner FROM v$version WHERE ROWNUM=1",
	status:    "SELECT status FROM v$instance",
	conn:      "SELECT 1",
	size:      "SELECT NVL(SUM(total_size),0) FROM v$datafile",
	tables:    "SELECT COUNT(*) FROM user_tables",
	tx:        "SELECT COUNT(*) FROM v$transaction",
	deadlock:  "SELECT 0",
	err:       "SELECT 0",
	uptime:    "SELECT (SYSDATE()-startup_time)*86400 FROM v$instance",
	log:       "",
}
