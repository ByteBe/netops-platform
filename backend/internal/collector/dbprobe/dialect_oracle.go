package dbprobe

import "fmt"

// Oracle
var oracleDialect = &dialect{
	driver: "oracle",
	buildDSN: func(d Instance) string {
		svc := d.DBName
		if svc == "" {
			svc = "ORCL"
		}
		return fmt.Sprintf("oracle://%s:%s@%s:%d/%s", d.User, d.Password, d.Host, d.Port, svc)
	},
	version:   "SELECT version_full FROM v$instance",
	status:    "SELECT status FROM v$instance",
	conn:      "SELECT 1",
	size:      "SELECT NVL(SUM(bytes),0) FROM v$datafile",
	tables:    "SELECT COUNT(*) FROM user_tables",
	tx:        "SELECT COUNT(*) FROM v$transaction",
	deadlock:  "SELECT 0",
	err:       "SELECT 0",
	uptime:    "SELECT (SYSDATE-startup_time)*86400 FROM v$instance",
	log:       "",
}
