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
	"time"
)

// fakeProxy SqlProxy 구현을 DB 없이 대체하는 테스트 더블이다.
// exec/query/prepare 호출 시 주입된 에러를 반환하고, debug/query/exec 호출 순서를
// events에 기록하여 호출 순서(예: debugPrint가 실행 전에 호출되는지)를 검증할 수 있다.
type fakeProxy struct {
	execErr  error
	queryErr error
	rows     *sql.Rows
	debugOn  bool
	tx       bool

	events  []string // "debug" | "query" | "exec" | "prepare" 호출 순서 기록
	lastCtx context.Context
	ctxUsed bool // *Context 경로가 호출되었는지
}

func (f *fakeProxy) exec(query string, args ...interface{}) (sql.Result, error) {
	f.events = append(f.events, "exec")
	if f.execErr != nil {
		return nil, f.execErr
	}
	return driverResultStub{}, nil
}

func (f *fakeProxy) query(query string, args ...interface{}) (*sql.Rows, error) {
	f.events = append(f.events, "query")
	return f.rows, f.queryErr
}

func (f *fakeProxy) queryRow(query string, args ...interface{}) *sql.Row {
	f.events = append(f.events, "queryRow")
	return nil
}

func (f *fakeProxy) prepare(query string) (*sql.Stmt, error) {
	f.events = append(f.events, "prepare")
	return nil, f.execErr
}

func (f *fakeProxy) execContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	f.lastCtx = ctx
	f.ctxUsed = true
	return f.exec(query, args...)
}

func (f *fakeProxy) queryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	f.lastCtx = ctx
	f.ctxUsed = true
	return f.query(query, args...)
}

func (f *fakeProxy) prepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	f.lastCtx = ctx
	f.ctxUsed = true
	return f.prepare(query)
}

func (f *fakeProxy) isTransaction() bool { return f.tx }

func (f *fakeProxy) debugEnabled() bool { return f.debugOn }

func (f *fakeProxy) debugPrint(format string, params ...interface{}) {
	f.events = append(f.events, "debug")
}

func (f *fakeProxy) recordExcution(stmtId string, start time.Time) {}

// driverResultStub sql.Result 테스트 더블
type driverResultStub struct{}

func (driverResultStub) LastInsertId() (int64, error) { return 0, nil }
func (driverResultStub) RowsAffected() (int64, error) { return 0, nil }
