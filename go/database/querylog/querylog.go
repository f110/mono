package querylog

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

type Logger interface {
	Log(d time.Duration, v string)
}

var (
	log             Logger
	minimumDuration time.Duration
)

func init() {
	sql.Register("querylog", Driver{})
}

func Init(l *zap.Logger) {
	log = &loggerWithZap{Logger: l}
}

func SetMinimumDuration(d time.Duration) {
	minimumDuration = d
}

type loggerWithZap struct {
	*zap.Logger
}

func (l *loggerWithZap) Log(d time.Duration, v string) {
	l.Info("QueryLog", zap.Duration("duration", d), zap.String("query", v))
}

type Driver struct{}

func (d Driver) Open(name string) (driver.Conn, error) {
	defer loggingQueryTime(time.Now(), name)

	conn, err := mysql.MySQLDriver{}.Open(name)
	if err == nil {
		return &Conn{internal: conn}, err
	}

	return nil, err
}

type Conn struct {
	internal driver.Conn
}

func (conn *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return conn.internal.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func (conn *Conn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := conn.internal.Prepare(query)
	if err == nil {
		return &Stmt{internal: stmt, query: query}, err
	}

	return nil, err
}

func (conn *Conn) Close() error {
	return conn.internal.Close()
}

func (conn *Conn) Begin() (driver.Tx, error) {
	return conn.internal.Begin()
}

type Stmt struct {
	query    string
	internal driver.Stmt
}

func (stmt *Stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	defer stmt.loggingQueryTime(time.Now())

	return stmt.internal.(driver.StmtQueryContext).QueryContext(ctx, args)
}

func (stmt *Stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	defer stmt.loggingQueryTime(time.Now())

	return stmt.internal.(driver.StmtExecContext).ExecContext(ctx, args)
}

func (stmt *Stmt) Close() error {
	return stmt.internal.Close()
}

func (stmt *Stmt) NumInput() int {
	return stmt.internal.NumInput()
}

func (stmt *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return stmt.internal.Exec(args)
}

func (stmt *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return stmt.internal.Query(args)
}

func (stmt *Stmt) loggingQueryTime(t1 time.Time) {
	loggingQueryTime(t1, stmt.query)
}

func loggingQueryTime(t1 time.Time, v string) {
	if log != nil {
		d := time.Now().Sub(t1)
		if minimumDuration <= d {
			log.Log(d, v)
		}
	}
}
