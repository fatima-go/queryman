/*
 * Copyright (c) 2026 DREAMUS COMPANY.
 * All right reserved.
 * This software is the confidential and proprietary information of DREAMUS COMPANY.
 * You shall not disclose such Confidential Information and
 * shall use it only in accordance with the terms of the license agreement
 * you entered into with DREAMUS COMPANY.
 */

package queryman

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
	"testing"
	"time"
)

// stubState database/sql 백엔드 드라이버 동작을 제어하고 호출 횟수를 집계한다.
// 실제 MySQL 없이 prepare/exec/query 경로를 검증하기 위한 stdlib 기반 stub이다.
type stubState struct {
	mu         sync.Mutex
	execErr    error
	queryErr   error
	rowsErr    error // rows 반복(Next) 단계에서 반환할 에러 (스트리밍 중 timeout 재현)
	execCalls  int
	queryCalls int
}

func (s *stubState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execErr = nil
	s.queryErr = nil
	s.rowsErr = nil
	s.execCalls = 0
	s.queryCalls = 0
}

func (s *stubState) execCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.execCalls
}

var stub = &stubState{}

type stubDriver struct{}

func (stubDriver) Open(name string) (driver.Conn, error) { return stubConn{}, nil }

type stubConn struct{}

func (stubConn) Prepare(query string) (driver.Stmt, error) { return stubStmt{}, nil }
func (stubConn) Close() error                              { return nil }
func (stubConn) Begin() (driver.Tx, error)                 { return stubTx{}, nil }

type stubStmt struct{}

func (stubStmt) Close() error  { return nil }
func (stubStmt) NumInput() int { return -1 }

func (stubStmt) Exec(args []driver.Value) (driver.Result, error) {
	stub.mu.Lock()
	stub.execCalls++
	e := stub.execErr
	stub.mu.Unlock()
	if e != nil {
		return nil, e
	}
	return driver.RowsAffected(1), nil
}

func (stubStmt) Query(args []driver.Value) (driver.Rows, error) {
	stub.mu.Lock()
	stub.queryCalls++
	e := stub.queryErr
	stub.mu.Unlock()
	if e != nil {
		return nil, e
	}
	return stubRows{}, nil
}

type stubTx struct{}

func (stubTx) Commit() error   { return nil }
func (stubTx) Rollback() error { return nil }

type stubRows struct{}

func (stubRows) Columns() []string { return []string{} }
func (stubRows) Close() error      { return nil }
func (stubRows) Next(dest []driver.Value) error {
	stub.mu.Lock()
	e := stub.rowsErr
	stub.mu.Unlock()
	if e != nil {
		return e
	}
	return io.EOF
}

func init() {
	sql.Register("queryman-stub", stubDriver{})
}

// newStubDB stub 드라이버 기반 *sql.DB를 연다.
func newStubDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("queryman-stub", "stub")
	if err != nil {
		t.Fatalf("fail to open stub db: %s", err)
	}
	return db
}

// countingProxy stub *sql.DB에 위임하면서 prepare 호출 횟수를 집계하는 SqlProxy다.
// prepare 호출 수 == doExec 라운드 수이므로(database/sql 내부 재시도는 우리의 prepare를
// 다시 호출하지 않음) queryman 레벨의 재실행 여부를 정확히 측정할 수 있다.
type countingProxy struct {
	db           *sql.DB
	prepareCalls int
}

func (c *countingProxy) exec(query string, args ...interface{}) (sql.Result, error) {
	return c.db.Exec(query, args...)
}
func (c *countingProxy) query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}
func (c *countingProxy) queryRow(query string, args ...interface{}) *sql.Row {
	return c.db.QueryRow(query, args...)
}
func (c *countingProxy) prepare(query string) (*sql.Stmt, error) {
	c.prepareCalls++
	return c.db.Prepare(query)
}
func (c *countingProxy) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}
func (c *countingProxy) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}
func (c *countingProxy) prepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	c.prepareCalls++
	return c.db.PrepareContext(ctx, query)
}
func (c *countingProxy) isTransaction() bool                             { return false }
func (c *countingProxy) debugEnabled() bool                              { return false }
func (c *countingProxy) debugPrint(format string, params ...interface{}) {}
func (c *countingProxy) recordExcution(stmtId string, start time.Time)   {}
