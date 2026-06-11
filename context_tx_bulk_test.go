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
	"errors"
	"testing"
)

// stubFinder QueryStatementFinder 테스트 더블
type stubFinder struct {
	stmt QueryStatement
}

func (f stubFinder) find(id string) (QueryStatement, error) { return f.stmt, nil }

// TestBeginTxPropagatesCancellation BeginTx가 ctx를 db.BeginTx까지 전파하는지 검증한다 (FR-4.4).
func TestBeginTxPropagatesCancellation(t *testing.T) {
	stub.reset()
	man := &QueryMan{db: newStubDB(t)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := man.BeginTx(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("BeginTx must propagate ctx cancellation, got: %v", err)
	}
}

// TestNestedBulkPathContextAware nested bulk(prepared statement) 실행 경로가
// context-aware로 동작하여 취소가 전파됨을 검증한다 (FR-4.4 — pstmt.ExecContext).
func TestNestedBulkPathContextAware(t *testing.T) {
	stub.reset()
	proxy := newContextBoundProxy(&QueryMan{db: newStubDB(t)}, expiredContext())
	stmt := QueryStatement{
		Id:            "InsertCity",
		Query:         "insert into city (a) values (?)",
		eleType:       eleTypeInsert,
		columnMention: []ColumnBind{NewColumnBind("a", 0)},
	}
	args := []interface{}{[]interface{}{1}}

	_, err := execWithNestedList(proxy, stmt, args)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("nested bulk path must be context-aware, got: %v", err)
	}
}

// TestBulkExecuteContextDeadline bulk ExecuteContext의 ctx deadline 전파를 검증한다 (FR-4.4).
func TestBulkExecuteContextDeadline(t *testing.T) {
	stub.reset()
	man := &QueryMan{db: newStubDB(t)}
	stmt := QueryStatement{
		Id:      "InsertCity",
		Query:   "insert into city (a) values (?)",
		eleType: eleTypeInsert,
	}
	bulk := newQuerymanBulk(man, stmt)
	bulk.params = []interface{}{1}
	bulk.execCount = 1

	_, err := bulk.ExecuteContext(expiredContext())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bulk ExecuteContext must propagate ctx deadline, got: %v", err)
	}
}

// TestTransactionQueryWithStmtContextDeadline 트랜잭션 객체의 *Context 변형이 ctx
// deadline을 전파하는지 검증한다 (FR-4.4).
func TestTransactionQueryWithStmtContextDeadline(t *testing.T) {
	stub.reset()
	db := newStubDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %s", err)
	}
	stmt := QueryStatement{Id: "selectCity", Query: "select 1", eleType: eleTypeSelect}
	dbtx := &DBTransaction{tx: tx, debugger: &fakeProxy{}, queryFinder: stubFinder{stmt: stmt}}

	result := dbtx.QueryWithStmtContext(expiredContext(), "selectCity")
	if !errors.Is(result.GetError(), context.DeadlineExceeded) {
		t.Fatalf("transaction QueryWithStmtContext must propagate ctx deadline, got: %v", result.GetError())
	}
}
