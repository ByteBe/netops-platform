package storage

import (
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)

// dmDialector 达梦数据库 GORM 方言（基于 gitee.com/chunanyong/dm 纯Go驱动，MySQL兼容模式）
type dmDialector struct {
	dsn string
}

// OpenDM 创建达梦方言
func OpenDM(dsn string) gorm.Dialector {
	return &dmDialector{dsn: dsn}
}

func (d *dmDialector) Name() string { return "dm" }

func (d *dmDialector) Initialize(db *gorm.DB) error {
	sqlDB, err := sql.Open("dm", d.dsn)
	if err != nil {
		return err
	}
	db.ConnPool = sqlDB
	return nil
}

func (d *dmDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return migrator.Migrator{Config: migrator.Config{
		DB:                          db,
		Dialector:                   d,
		CreateIndexAfterCreateTable: true,
	}}
}

// DataTypeOf 字段类型映射（DM MySQL兼容）
func (d *dmDialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "TINYINT(1)"
	case schema.Int, schema.Uint:
		size := field.Size
		if field.DataType == schema.Uint {
			size++
		}
		if size <= 8 {
			return "TINYINT"
		} else if size <= 16 {
			return "SMALLINT"
		} else if size <= 24 {
			return "MEDIUMINT"
		} else if size <= 32 {
			return "INT"
		}
		return "BIGINT"
	case schema.Float:
		if field.Precision > 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DOUBLE"
	case schema.String:
		size := field.Size
		if size == 0 {
			size = 255
		}
		if size > 8188 {
			return "TEXT"
		}
		return fmt.Sprintf("VARCHAR(%d)", size)
	case schema.Time:
		return "DATETIME(3)"
	case schema.Bytes:
		return "BLOB"
	default:
		if field.Size > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Size)
		}
		return "TEXT"
	}
}

func (d *dmDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return nil
}

func (d *dmDialector) BindVarTo(writer clause.Writer, _ *gorm.Statement, _ any) {
	writer.WriteByte('?')
}

func (d *dmDialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('`')
	writer.WriteString(strings.ReplaceAll(str, "`", ""))
	writer.WriteByte('`')
}

func (d *dmDialector) Explain(sql string, vars ...any) string {
	var sb strings.Builder
	idx := strings.IndexByte(sql, '?')
	for idx != -1 && len(vars) > 0 {
		sb.WriteString(sql[:idx])
		v := vars[0]
		vars = vars[1:]
		switch vv := v.(type) {
		case string:
			sb.WriteString("'" + strings.ReplaceAll(vv, "'", "''") + "'")
		case []byte:
			sb.WriteString("'" + string(vv) + "'")
		default:
			sb.WriteString(fmt.Sprint(v))
		}
		sql = sql[idx+1:]
		idx = strings.IndexByte(sql, '?')
	}
	sb.WriteString(sql)
	return sb.String()
}
