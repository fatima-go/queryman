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
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

// TestErrNoRowsAliasesSqlErrNoRows ErrNoRows가 sql.ErrNoRows 별칭으로 동작하는지
// 검증한다 (FR-6.3 — errors.Is 호환 + 기존 == 비교 보존).
func TestErrNoRowsAliasesSqlErrNoRows(t *testing.T) {
	if !errors.Is(ErrNoRows, sql.ErrNoRows) {
		t.Fatal("ErrNoRows must satisfy errors.Is(ErrNoRows, sql.ErrNoRows)")
	}
	if ErrNoRows != sql.ErrNoRows {
		t.Fatal("ErrNoRows must be identical to sql.ErrNoRows for legacy == comparison")
	}
}

// TestQueryWithListDebugPrintsBeforeExecution debugPrint가 쿼리 실행 전에
// 호출되는지 검증한다 (FR-6.2 — 장애 시 어떤 쿼리가 나갔는지 즉시 확인).
func TestQueryWithListDebugPrintsBeforeExecution(t *testing.T) {
	fake := &fakeProxy{debugOn: true, queryErr: errors.New("boom")}
	stmt := QueryStatement{
		Id:            "X",
		Query:         "select * from t where a = ?",
		eleType:       eleTypeSelect,
		columnMention: []ColumnBind{NewColumnBind("a", 0)},
	}

	queryWithList(fake, stmt, []interface{}{1})

	debugIdx, queryIdx := indexOf(fake.events, "debug"), indexOf(fake.events, "query")
	if debugIdx < 0 || queryIdx < 0 {
		t.Fatalf("expected both debug and query events, got: %v", fake.events)
	}
	if debugIdx > queryIdx {
		t.Fatalf("debugPrint must be called before query execution, events: %v", fake.events)
	}
}

// TestQueryWithMapDebugPrintsBeforeExecution map 경로에서도 debugPrint가 실행 전에
// 호출되는지 검증한다 (FR-6.2).
func TestQueryWithMapDebugPrintsBeforeExecution(t *testing.T) {
	fake := &fakeProxy{debugOn: true, queryErr: errors.New("boom")}
	stmt := QueryStatement{
		Id:            "X",
		Query:         "select * from t where a = ?",
		eleType:       eleTypeSelect,
		columnMention: []ColumnBind{NewColumnBind("a", 0)},
	}

	queryWithMap(fake, stmt, map[string]interface{}{"a": 1})

	debugIdx, queryIdx := indexOf(fake.events, "debug"), indexOf(fake.events, "query")
	if debugIdx < 0 || queryIdx < 0 {
		t.Fatalf("expected both debug and query events, got: %v", fake.events)
	}
	if debugIdx > queryIdx {
		t.Fatalf("debugPrint must be called before query execution, events: %v", fake.events)
	}
}

// TestNestedListExecNoQuerymanRetry 중첩 리스트 실행이 driver.ErrBadConn 에서도
// queryman 레벨 재실행(doExec 2회차) 없이 1회 라운드만 수행됨을 검증한다
// (FR-1.2 — 죽은 ErrBadConn 재시도 제거, ErrInvalidConn 재시도 금지).
// prepare 호출 횟수 == doExec 라운드 수이므로 1이어야 한다.
func TestNestedListExecNoQuerymanRetry(t *testing.T) {
	stub.reset()
	stub.execErr = driver.ErrBadConn

	proxy := &countingProxy{db: newStubDB(t)}
	stmt := QueryStatement{
		Id:            "InsertX",
		Query:         "insert into t (a) values (?)",
		eleType:       eleTypeInsert,
		columnMention: []ColumnBind{NewColumnBind("a", 0)},
	}
	args := []interface{}{[]interface{}{1}, []interface{}{2}}

	_, err := execWithNestedList(proxy, stmt, args)
	if err == nil {
		t.Fatal("expected error from nested list exec")
	}
	if proxy.prepareCalls != 1 {
		t.Fatalf("expected exactly 1 doExec round (no queryman retry), prepare calls: %d", proxy.prepareCalls)
	}
}

func indexOf(events []string, target string) int {
	for i, e := range events {
		if e == target {
			return i
		}
	}
	return -1
}
