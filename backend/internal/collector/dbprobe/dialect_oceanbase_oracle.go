package dbprobe

import "fmt"

// OceanBase Oracle 模式（OBCI 兼容 Oracle 协议）
var oceanbaseOracleDialect = &dialect{
	driver: "oracle",
	buildDSN: func(d Instance) string {
		svc := d.DBName
		if svc == "" {
			svc = "SYS"
		}
		return fmt.Sprintf("oracle://%s:%s@%s:%d/%s", d.User, d.Password, d.Host, d.Port, svc)
	},
	version:  "SELECT version FROM v$instance",
	status:   "SELECT status FROM v$instance",
	conn:     "SELECT 1",
	size:     "SELECT NVL(SUM(bytes),0) FROM dba_data_files",
	tables:   "SELECT COUNT(*) FROM user_tables",
	tx:       "SELECT COUNT(*) FROM v$transaction",
	deadlock: "SELECT 0",
	err:      "SELECT 0",
	uptime:   "SELECT (SYSDATE-startup_time)*86400 FROM v$instance",
	log:      "",
}
