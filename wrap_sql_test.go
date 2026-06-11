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
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestExecWrapsDBError exec 경로의 DB 왕복 에러가 QueryError로 래핑되는지 검증한다 (FR-2.4).
func TestExecWrapsDBError(t *testing.T) {
	fake := &fakeProxy{execErr: mysql.ErrInvalidConn}
	stmt := QueryStatement{
		Id:            "UpdateCity",
		Query:         "update city set name = ? where id = ?",
		eleType:       eleTypeUpdate,
		columnMention: []ColumnBind{NewColumnBind("name", 0), NewColumnBind("id", 1)},
	}

	_, err := execWithMap(fake, stmt, map[string]interface{}{"name": "seoul", "id": 1})
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("exec error must be wrapped as QueryError, got: %v", err)
	}
	if qe.StmtId != "UpdateCity" || qe.Op != "exec" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
	if !errors.Is(err, mysql.ErrInvalidConn) {
		t.Error("wrapped error must preserve original via Unwrap")
	}
}

// TestQueryWrapsDBError query 경로의 DB 왕복 에러가 QueryError로 래핑되는지 검증한다 (FR-2.4).
func TestQueryWrapsDBError(t *testing.T) {
	fake := &fakeProxy{queryErr: mysql.ErrInvalidConn}
	stmt := QueryStatement{
		Id:            "SelectCity",
		Query:         "select * from city where id = ?",
		eleType:       eleTypeSelect,
		columnMention: []ColumnBind{NewColumnBind("id", 0)},
	}

	result := queryWithList(fake, stmt, []interface{}{1})
	qe, ok := AsQueryError(result.GetError())
	if !ok {
		t.Fatalf("query error must be wrapped as QueryError, got: %v", result.GetError())
	}
	if qe.StmtId != "SelectCity" || qe.Op != "query" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
}

// TestPreExecutionErrorNotWrapped 실행 전 검증 에러(바인딩 불일치)는 래핑되지 않음을
// 검증한다 (FR-2.4 — DB 왕복 에러만 래핑).
func TestPreExecutionErrorNotWrapped(t *testing.T) {
	fake := &fakeProxy{}
	stmt := QueryStatement{
		Id:            "UpdateCity",
		Query:         "update city set name = ? where id = ?",
		eleType:       eleTypeUpdate,
		columnMention: []ColumnBind{NewColumnBind("name", 0), NewColumnBind("id", 1)},
	}

	// columnMention 2개인데 args 1개 -> 실행 전 binding 불일치 에러
	_, err := execWithList(fake, stmt, []interface{}{"only-one"})
	if err == nil {
		t.Fatal("expected binding mismatch error")
	}
	if _, ok := AsQueryError(err); ok {
		t.Errorf("pre-execution validation error must NOT be wrapped, got QueryError: %v", err)
	}
}

// TestPstmtExecWrapsDBError prepared statement 실행(pstmt.Exec) 에러가 QueryError로
// 래핑되는지 검증한다 (FR-2.4 — pstmt 경로).
func TestPstmtExecWrapsDBError(t *testing.T) {
	stub.reset()
	stub.execErr = driver.ErrBadConn

	proxy := &countingProxy{db: newStubDB(t)}
	stmt := QueryStatement{
		Id:            "InsertCity",
		Query:         "insert into city (a) values (?)",
		eleType:       eleTypeInsert,
		columnMention: []ColumnBind{NewColumnBind("a", 0)},
	}
	args := []interface{}{[]interface{}{1}}

	_, err := execWithNestedList(proxy, stmt, args)
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("pstmt exec error must be wrapped as QueryError, got: %v", err)
	}
	if qe.StmtId != "InsertCity" || qe.Op != "exec" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
}
