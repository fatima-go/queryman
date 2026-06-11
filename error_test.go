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
	"database/sql/driver"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// fakeNetErr net.Error 테스트 더블 (Timeout 동작 제어)
type fakeNetErr struct {
	timeout bool
}

func (e fakeNetErr) Error() string   { return "fake net error" }
func (e fakeNetErr) Timeout() bool   { return e.timeout }
func (e fakeNetErr) Temporary() bool { return false }

// TestQueryErrorFormatAndUnwrap QueryError의 메시지 포맷, Unwrap 체인,
// SQL 본문 미포함을 검증한다 (FR-2.1/2.2/2.5).
func TestQueryErrorFormatAndUnwrap(t *testing.T) {
	qe := &QueryError{
		StmtId:  "selectTrackMeta",
		Op:      "query",
		Elapsed: 30001 * time.Millisecond,
		Err:     mysql.ErrInvalidConn,
	}

	msg := qe.Error()
	if !strings.Contains(msg, "selectTrackMeta") {
		t.Errorf("Error() must contain stmt id, got: %s", msg)
	}
	if !strings.Contains(msg, "invalid connection") {
		t.Errorf("Error() must contain original error, got: %s", msg)
	}
	if !strings.Contains(msg, "30") {
		t.Errorf("Error() must contain elapsed seconds, got: %s", msg)
	}
	// Unwrap 체인으로 원본 sentinel 판별 가능해야 한다
	if !errors.Is(qe, mysql.ErrInvalidConn) {
		t.Error("errors.Is(qe, mysql.ErrInvalidConn) must hold via Unwrap")
	}
}

// TestWrapQueryError wrapQueryError의 경계: nil 통과, 센티널 비래핑,
// 이중 래핑 방지, 일반 에러 래핑을 검증한다 (FR-2.4).
func TestWrapQueryError(t *testing.T) {
	start := time.Now().Add(-time.Second)

	if got := wrapQueryError("X", "query", start, nil); got != nil {
		t.Errorf("nil error must pass through, got: %v", got)
	}

	// 센티널(ErrNoRows)은 래핑하지 않는다
	if got := wrapQueryError("X", "query", start, ErrNoRows); got != ErrNoRows {
		t.Errorf("ErrNoRows sentinel must not be wrapped, got: %v", got)
	}

	// 일반 DB 왕복 에러는 래핑한다
	wrapped := wrapQueryError("selectTrackMeta", "query", start, mysql.ErrInvalidConn)
	qe, ok := AsQueryError(wrapped)
	if !ok {
		t.Fatalf("expected wrapped QueryError, got: %v", wrapped)
	}
	if qe.StmtId != "selectTrackMeta" || qe.Op != "query" {
		t.Errorf("QueryError context not preserved: %+v", qe)
	}
	if qe.Elapsed <= 0 {
		t.Errorf("Elapsed must be positive, got: %v", qe.Elapsed)
	}

	// 이중 래핑 방지: 이미 QueryError면 그대로 반환
	again := wrapQueryError("Y", "exec", start, wrapped)
	if again != wrapped {
		t.Errorf("already-wrapped error must not be re-wrapped")
	}
}

// TestIsConnError 연결 계열 장애 판별 매트릭스를 검증한다 (FR-3.2).
func TestIsConnError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"ErrInvalidConn(readTimeout형)", mysql.ErrInvalidConn, true},
		{"ErrBadConn", driver.ErrBadConn, true},
		{"dial refused", &net.OpError{Op: "dial", Err: fakeNetErr{}}, true},
		{"dial timeout", &net.OpError{Op: "dial", Err: fakeNetErr{timeout: true}}, true},
		{"context deadline", context.DeadlineExceeded, false},
		{"plain error", errors.New("syntax error"), false},
		{"wrapped ErrInvalidConn", &QueryError{Err: mysql.ErrInvalidConn}, true},
	}
	for _, c := range cases {
		if got := IsConnError(c.err); got != c.want {
			t.Errorf("%s: IsConnError = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestIsTimeout 확정적 timeout 판별 매트릭스를 검증한다 (FR-3.2).
// DSN readTimeout(ErrInvalidConn)은 false여야 한다(원인 소실, 추정 미노출).
func TestIsTimeout(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"context deadline", context.DeadlineExceeded, true},
		{"dial timeout", &net.OpError{Op: "dial", Err: fakeNetErr{timeout: true}}, true},
		{"net non-timeout", &net.OpError{Op: "dial", Err: fakeNetErr{}}, false},
		{"ErrInvalidConn(readTimeout형)", mysql.ErrInvalidConn, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, c := range cases {
		if got := IsTimeout(c.err); got != c.want {
			t.Errorf("%s: IsTimeout = %v, want %v", c.name, got, c.want)
		}
	}
}
