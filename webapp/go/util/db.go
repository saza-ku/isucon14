package util

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const (
	defaultDriverName = "mysql"
)

func defaultConnector(dsn string) (*sql.DB, error) {
	return sql.Open(defaultDriverName, dsn)
}

// NewIsuconDB はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
// 再起動試験対策済み。
func NewIsuconDB(config *mysql.Config) (*sqlx.DB, error) {
	return NewIsuconDBFromConnector(defaultConnector, config.FormatDSN())
}

// NewIsuconDBFromDSN はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
// 再起動試験対策済み。
func NewIsuconDBFromDSN(dsn string) (*sqlx.DB, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}
	return NewIsuconDBFromConnector(defaultConnector, config.FormatDSN())
}

// NewIsuconDBFromConnector はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
func NewIsuconDBWithDriverName(driverName string, dsn string) (*sqlx.DB, error) {
	return NewIsuconDBFromConnector(func(dsn string) (*sql.DB, error) {
		return sql.Open(driverName, dsn)
	}, dsn)
}

// NewIsuconDBFromConnector はISUCON用にカスタマイズされたsqlxのDBクライアントを返します。
// connector でDBの接続処理をカスタマイズできます。
func NewIsuconDBFromConnector(connector func(dsn string) (*sql.DB, error), dsn string) (*sqlx.DB, error) {
	config, err := mysql.ParseDSN(dsn)
	if err == nil {
		// MySQLの場合、ISUCONにおける必須の設定項目たち
		config.ParseTime = true
		config.InterpolateParams = true
		dsn = config.FormatDSN()
	}

	stdDb, err := connector(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open to DB: %w", err)
	}

	dbx := sqlx.NewDb(stdDb, defaultDriverName)

	// コネクション数はデフォルトでは無制限になっている。
	// 数十から数百くらいで要調整。
	dbx.SetMaxOpenConns(20)
	dbx.SetMaxIdleConns(20)
	dbx.SetConnMaxLifetime(5 * time.Minute)

	// 再起動試験対策
	// Pingして接続が確立するまで待つ
	for {
		if err := dbx.Ping(); err == nil {
			break
		} else {
			fmt.Println(err)
			time.Sleep(time.Second * 2)
		}
	}
	fmt.Println("ISUCON DB ready")

	return dbx, nil
}

// CreateIndexIfNotExists はMySQLのインデックスが存在しない場合に、インデックスを作成します。
// 既に存在する場合はエラーを無視します。
// ISUCONではinitializeのタイミングで、DROP TABLEするのではなくTRUNCATEする場合があります。
// その場合はインデックスは消されず残ってしまうので、Duplicateエラーが発生します。
func CreateIndexIfNotExists(db *sqlx.DB, query string) error {
	_, err := db.Exec(query)

	// 既に存在する場合はエラーになるが、それ以外のエラーはそのまま返す
	var mysqlErr *mysql.MySQLError
	if err != nil {
		if errors.As(err, &mysqlErr) {
			if mysqlErr.Number == 1061 {
				fmt.Println("detected already existing index, but it's ok")
				return nil
			}
		}
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// OverrideAddr はDSNのアドレスを上書きします。
// addr は 127.0.0.1:3306 のような形式で指定してください。
func OverrideAddr(basDSN string, addr string) (*mysql.Config, error) {
	mysqlCfg, err := mysql.ParseDSN(basDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}
	mysqlCfg.Addr = addr
	return mysqlCfg, nil
}
