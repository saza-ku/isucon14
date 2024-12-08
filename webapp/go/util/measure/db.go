package measure

import (
	"database/sql"
	"fmt"

	"util"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
)

const (
	driverName = "mysql"
)

func defaultConnector(dsn string) (*sql.DB, error) {
	return otelsql.Open(driverName, dsn, otelsql.WithDBName(driverName))
}

// NewIsuconDB はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
// 再起動試験対策済み。
func NewIsuconDB(config *mysql.Config) (*sqlx.DB, error) {
	return util.NewIsuconDBFromConnector(defaultConnector, config.FormatDSN())
}

// NewIsuconDBFromDSN はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
// 再起動試験対策済み。
func NewIsuconDBFromDSN(dsn string) (*sqlx.DB, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}
	return util.NewIsuconDBFromConnector(defaultConnector, config.FormatDSN())
}

// NewIsuconDBWithDriverName はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
func NewIsuconDBWithDriverName(driverName string, dsn string) (*sqlx.DB, error) {
	return util.NewIsuconDBFromConnector(func(dsn string) (*sql.DB, error) {
		return otelsql.Open(driverName, dsn, otelsql.WithDBName(driverName))
	}, dsn)
}
