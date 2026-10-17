package dbprobe

import "strings"

// dialect 描述一种数据库的驱动、DSN 构造与采集 SQL
type dialect struct {
	driver   string
	buildDSN func(Instance) string
	version  string
	status   string
	conn     string
	maxConn  string
	size     string
	tables   string
	tx       string
	deadlock string
	err      string
	uptime   string
	log      string
}

// lookupDialect 按类型返回对应方言（小写 key）
func lookupDialect(t string) *dialect {
	switch strings.ToLower(t) {
	case "oracle":
		return oracleDialect
	case "dm", "dameng":
		return dmDialect
	case "kingbase", "postgres", "postgresql":
		return pgDialect
	case "sqlite", "builtin":
		return sqliteDialect
	case "oceanbase":
		return oceanbaseDialect
	case "oceanbase_oracle", "oceanbase-oracle":
		return oceanbaseOracleDialect
	default:
		return mysqlDialect
	}
}
