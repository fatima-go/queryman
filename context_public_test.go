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
	"strings"
	"testing"
	"time"
)

// newStubManWithStmt stub DB와 미리 등록된 statement를 가진 QueryMan을 만든다.
// find()가 statementMap에서 직접 찾으므로 normalizer 초기화가 필요 없다.
func newStubManWithStmt(t *testing.T, stmt QueryStatement) *QueryMan {
	t.Helper()
	return &QueryMan{
		db:           newStubDB(t),
		statementMap: map[string]QueryStatement{strings.ToUpper(stmt.Id): stmt},
	}
}

func expiredContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	cancel()
	return ctx
}

// TestQueryWithStmtContextDeadline 공개 QueryWithStmtContext가 ctx deadline 초과를
// context.DeadlineExceeded로 확정 구분 가능하게 하는지 검증한다 (FR-4.1/4.3).
func TestQueryWithStmtContextDeadline(t *testing.T) {
	stub.reset()
	man := newStubManWithStmt(t, QueryStatement{Id: "selectTrack", Query: "select 1", eleType: eleTypeSelect})

	result := man.QueryWithStmtContext(expiredContext(), "selectTrack")
	err := result.GetError()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
	if !IsTimeout(err) {
		t.Errorf("IsTimeout must be true for ctx deadline, err: %v", err)
	}
	if _, ok := AsQueryError(err); !ok {
		t.Errorf("ctx deadline error must still be wrapped as QueryError, got: %v", err)
	}
}

// TestExecuteWithStmtContextDeadline 공개 ExecuteWithStmtContext의 ctx deadline 전파를
// 검증한다 (FR-4.1/4.3).
func TestExecuteWithStmtContextDeadline(t *testing.T) {
	stub.reset()
	man := newStubManWithStmt(t, QueryStatement{Id: "updateTrack", Query: "update track set a = 1", eleType: eleTypeUpdate})

	_, err := man.ExecuteWithStmtContext(expiredContext(), "updateTrack")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
	if !IsTimeout(err) {
		t.Errorf("IsTimeout must be true for ctx deadline, err: %v", err)
	}
}

// TestQueryRowWithStmtContextDeadline 공개 QueryRowWithStmtContext의 ctx deadline
// 전파를 검증한다 (FR-4.1/4.3).
func TestQueryRowWithStmtContextDeadline(t *testing.T) {
	stub.reset()
	man := newStubManWithStmt(t, QueryStatement{Id: "selectTrackMeta", Query: "select 1", eleType: eleTypeSelect})

	result := man.QueryRowWithStmtContext(expiredContext(), "selectTrackMeta")
	var dest int
	err := result.Scan(&dest)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
	if !IsTimeout(err) {
		t.Errorf("IsTimeout must be true for ctx deadline, err: %v", err)
	}
}
