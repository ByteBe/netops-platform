package dbprobe

// 内置 SQLite
var sqliteDialect = &dialect{
	driver: "sqlite",
	buildDSN: func(d Instance) string {
		return d.Host
	},
	version:  "SELECT sqlite_version()",
	status:   "SELECT 'up'",
	conn:     "SELECT 1",
	size:     "SELECT page_count*page_size FROM pragma_page_count(), pragma_page_size()",
	tables:   "SELECT COUNT(*) FROM sqlite_master WHERE type='table'",
	tx:       "SELECT 0",
	deadlock: "SELECT 0",
	err:      "SELECT 0",
	uptime:   "SELECT 0",
	log:      "",
}
