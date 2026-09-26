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

// oracleDialector Oracle 数据库 GORM 方言（基于 github.com/sijms/go-ora/v2 纯Go驱动）
type oracleDialector struct {
	dsn string
}

// OpenOracle 创建 Oracle 方言
func OpenOracle(dsn string) gorm.Dialector {
	return &oracleDialector{dsn: dsn}
}

func (d *oracleDialector) Name() string { return "oracle" }

func (d *oracleDialector) Initialize(db *gorm.DB) error {
	sqlDB, err := sql.Open("oracle", d.dsn)
	if err != nil {
		return err
	}
	db.ConnPool = sqlDB
	return nil
}

func (d *oracleDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return migrator.Migrator{Config: migrator.Config{
		DB:        db,
		Dialector: d,
	}}
}

// DataTypeOf 字段类型映射（Oracle）
func (d *oracleDialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "NUMBER(1)"
	case schema.Int, schema.Uint:
		return "NUMBER(19)"
	case schema.Float:
		if field.Precision > 0 {
			return fmt.Sprintf("NUMBER(%d,%d)", field.Precision, field.Scale)
		}
		return "BINARY_DOUBLE"
	case schema.String:
		size := field.Size
		if size == 0 {
			size = 255
		}
		if size > 4000 {
			return "CLOB"
		}
		return fmt.Sprintf("VARCHAR2(%d)", size)
	case schema.Time:
		return "TIMESTAMP(3)"
	case schema.Bytes:
		return "BLOB"
	default:
		return "CLOB"
	}
}

func (d *oracleDialector) DefaultValueOf(*schema.Field) clause.Expression {
	return nil
}

func (d *oracleDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, _ any) {
	// Oracle 使用 :N 绑定变量
	writer.WriteByte(':')
	writer.WriteString(fmt.Sprintf("%d", len(stmt.Vars)))
}

func (d *oracleDialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('"')
	writer.WriteString(strings.ReplaceAll(str, `"`, ""))
	writer.WriteByte('"')
}

func (d *oracleDialector) Explain(sql string, vars ...any) string {
	var sb strings.Builder
	for i, v := range vars {
		idx := strings.Index(sql, fmt.Sprintf(":%d", i+1))
		if idx < 0 {
			continue
		}
		sb.WriteString(sql[:idx])
		switch vv := v.(type) {
		case string:
			sb.WriteString("'" + strings.ReplaceAll(vv, "'", "''") + "'")
		case []byte:
			sb.WriteString("'" + string(vv) + "'")
		default:
			sb.WriteString(fmt.Sprint(v))
		}
		sql = sql[idx+len(fmt.Sprintf(":%d", i+1)):]
	}
	sb.WriteString(sql)
	return sb.String()
}
