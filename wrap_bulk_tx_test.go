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
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestBulkExecuteWrapsDBError bulk Execute의 DB 왕복 에러가 QueryError로 래핑되는지
// 검증한다 (FR-2.4 — bulk.go는 sql.go 공통 경로를 우회하므로 별도 래핑 필요).
func TestBulkExecuteWrapsDBError(t *testing.T) {
	fake := &fakeProxy{execErr: mysql.ErrInvalidConn}
	stmt := QueryStatement{
		Id:      "InsertCity",
		Query:   "insert into city (a) values (?)",
		eleType: eleTypeInsert,
	}
	bulk := newQuerymanBulk(fake, stmt)
	bulk.params = []interface{}{1}
	bulk.execCount = 1

	_, err := bulk.Execute()
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("bulk execute error must be wrapped as QueryError, got: %v", err)
	}
	if qe.StmtId != "InsertCity" || qe.Op != "exec" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
}

// TestTransactionExecWrapsDBError 트랜잭션 exec 경로 에러가 비트랜잭션과 동일하게
// QueryError로 래핑되는지 검증한다 (FR-2.4 — DBTransaction은 SqlProxy로서 sql.go
// 공통 래핑 경로를 탄다).
func TestTransactionExecWrapsDBError(t *testing.T) {
	stub.reset()
	stub.execErr = mysql.ErrInvalidConn

	db := newStubDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %s", err)
	}
	dbtx := &DBTransaction{tx: tx, debugger: &fakeProxy{}}
	stmt := QueryStatement{
		Id:            "UpdateCity",
		Query:         "update city set name = ? where id = ?",
		eleType:       eleTypeUpdate,
		columnMention: []ColumnBind{NewColumnBind("name", 0), NewColumnBind("id", 1)},
	}

	_, err = execute(dbtx, stmt, map[string]interface{}{"name": "seoul", "id": 1})
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("transaction exec error must be wrapped as QueryError, got: %v", err)
	}
	if qe.StmtId != "UpdateCity" || qe.Op != "exec" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
}

// TestTransactionQueryWrapsDBError 트랜잭션 query 경로 에러도 QueryError로
// 래핑되는지 검증한다 (FR-2.4).
func TestTransactionQueryWrapsDBError(t *testing.T) {
	stub.reset()
	stub.queryErr = mysql.ErrInvalidConn

	db := newStubDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %s", err)
	}
	dbtx := &DBTransaction{tx: tx, debugger: &fakeProxy{}}
	stmt := QueryStatement{
		Id:            "SelectCity",
		Query:         "select * from city where id = ?",
		eleType:       eleTypeSelect,
		columnMention: []ColumnBind{NewColumnBind("id", 0)},
	}

	result := queryMultiRow(dbtx, stmt, map[string]interface{}{"id": 1})
	qe, ok := AsQueryError(result.GetError())
	if !ok {
		t.Fatalf("transaction query error must be wrapped as QueryError, got: %v", result.GetError())
	}
	if qe.StmtId != "SelectCity" || qe.Op != "query" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
}
