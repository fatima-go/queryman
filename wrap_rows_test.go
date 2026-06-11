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
	"errors"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// TestQueryRowResultScanWrapsStreamingError 단일행 조회의 rows 반복 단계에서 발생한
// 에러(스트리밍 중 timeout/연결 무효화)가 QueryError로 래핑되는지 검증한다 (FR-2.4).
func TestQueryRowResultScanWrapsStreamingError(t *testing.T) {
	stub.reset()
	stub.rowsErr = mysql.ErrInvalidConn

	db := newStubDB(t)
	rows, err := db.Query("select name from track where id = ?", 1)
	if err != nil {
		t.Fatalf("query: %s", err)
	}
	qrr := newQueryRowResult(nil, rows, "selectTrackMeta", time.Now())

	var dest string
	err = qrr.Scan(&dest)
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("rows-stage error must be wrapped as QueryError, got: %v", err)
	}
	if qe.StmtId != "selectTrackMeta" || qe.Op != "query" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
	if !errors.Is(err, mysql.ErrInvalidConn) {
		t.Error("wrapped rows-stage error must preserve original via Unwrap")
	}
}

// TestQueryRowResultScanNoRowsNotWrapped 결과 없음(ErrNoRows)은 rows 단계에서도
// 래핑되지 않고 센티널 그대로 반환됨을 검증한다 (FR-2.4).
func TestQueryRowResultScanNoRowsNotWrapped(t *testing.T) {
	stub.reset() // rowsErr nil -> Next returns io.EOF (결과 없음)

	db := newStubDB(t)
	rows, err := db.Query("select name from track where id = ?", 1)
	if err != nil {
		t.Fatalf("query: %s", err)
	}
	qrr := newQueryRowResult(nil, rows, "selectTrackMeta", time.Now())

	var dest string
	err = qrr.Scan(&dest)
	if !errors.Is(err, ErrNoRows) {
		t.Fatalf("no rows must return ErrNoRows sentinel, got: %v", err)
	}
	if _, ok := AsQueryError(err); ok {
		t.Errorf("ErrNoRows must NOT be wrapped as QueryError")
	}
}

// TestQueryResultScanWrapsRowsError 다중행 조회의 rows.Err()가 Scan에서 QueryError로
// 래핑되는지 검증한다 (FR-2.4).
func TestQueryResultScanWrapsRowsError(t *testing.T) {
	stub.reset()
	stub.rowsErr = mysql.ErrInvalidConn

	db := newStubDB(t)
	rows, err := db.Query("select id from track")
	if err != nil {
		t.Fatalf("query: %s", err)
	}
	qr := newQueryResult(nil, rows, "selectTrackList", time.Now())

	// Next()가 스트리밍 에러로 false를 반환한 뒤 Scan에서 rows.Err()가 노출된다
	qr.Next()
	var dest int
	err = qr.Scan(&dest)
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("rows-stage error must be wrapped as QueryError, got: %v", err)
	}
	if qe.StmtId != "selectTrackList" || qe.Op != "query" {
		t.Errorf("unexpected QueryError context: %+v", qe)
	}
}
