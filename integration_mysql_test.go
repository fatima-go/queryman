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
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// 통합 테스트는 실 MySQL이 필요하다. 환경변수로 접속 정보를 받아 구성하며,
// 미설정/미접속 시 t.Skip 한다. 단위 테스트(fake/stub)가 핵심 검증을 담당하고,
// 이 테스트는 readTimeout/rows 스트리밍/context 시나리오의 실환경 재현 보조다.
//
// 실행 예:
//
//	QUERYMAN_TEST_HOST=127.0.0.1:3306 QUERYMAN_TEST_USER=u \
//	QUERYMAN_TEST_PASSWORD=p QUERYMAN_TEST_DB=test go test -run TestIntegration -v
func integrationDSN(t *testing.T, extraParams string) string {
	t.Helper()
	host := os.Getenv("QUERYMAN_TEST_HOST")
	if host == "" {
		t.Skip("integration test skipped: set QUERYMAN_TEST_HOST to run against a real MySQL")
	}
	user := os.Getenv("QUERYMAN_TEST_USER")
	pass := os.Getenv("QUERYMAN_TEST_PASSWORD")
	db := os.Getenv("QUERYMAN_TEST_DB")
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", user, pass, host, db, extraParams)
}

func openIntegrationMan(t *testing.T, dsn string, stmts map[string]QueryStatement) *QueryMan {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Skipf("integration test skipped: open failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("integration test skipped: ping failed: %v", err)
	}
	return &QueryMan{db: db, statementMap: stmts}
}

// TestIntegrationReadTimeoutWrapsInvalidConn DSN readTimeout 초과 시 ErrInvalidConn이
// QueryError로 래핑되고 elapsed가 readTimeout 이상임을 검증한다.
func TestIntegrationReadTimeoutWrapsInvalidConn(t *testing.T) {
	dsn := integrationDSN(t, "readTimeout=1s&timeout=5s")
	man := openIntegrationMan(t, dsn, map[string]QueryStatement{
		"SLEEP2": {Id: "sleep2", Query: "select sleep(2)", eleType: eleTypeSelect},
	})
	defer man.db.Close()

	var v int
	err := man.QueryRowWithStmt("sleep2").Scan(&v)
	if err == nil {
		t.Fatal("expected readTimeout error")
	}
	if !errors.Is(err, mysql.ErrInvalidConn) {
		t.Errorf("expected ErrInvalidConn in chain, got: %v", err)
	}
	if !IsConnError(err) {
		t.Errorf("IsConnError must be true, err: %v", err)
	}
	qe, ok := AsQueryError(err)
	if !ok {
		t.Fatalf("expected QueryError, got: %v", err)
	}
	if qe.Elapsed < time.Second {
		t.Errorf("elapsed must be >= readTimeout(1s), got: %v", qe.Elapsed)
	}
}

// TestIntegrationContextTimeoutIsDeadlineExceeded context deadline을 readTimeout보다
// 짧게 두면 context.DeadlineExceeded로 확정 구분됨을 검증한다.
func TestIntegrationContextTimeoutIsDeadlineExceeded(t *testing.T) {
	dsn := integrationDSN(t, "readTimeout=10s&timeout=5s")
	man := openIntegrationMan(t, dsn, map[string]QueryStatement{
		"SLEEP2": {Id: "sleep2", Query: "select sleep(2)", eleType: eleTypeSelect},
	})
	defer man.db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var v int
	err := man.QueryRowWithStmtContext(ctx, "sleep2").Scan(&v)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
	if !IsTimeout(err) {
		t.Errorf("IsTimeout must be true, err: %v", err)
	}
}

// TestIntegrationRowsStreamingTimeout 다중행 스트리밍 중 readTimeout이 rows 단계에서
// 발생해 QueryError로 래핑됨을 검증한다. 행 사이 지연을 위해 큰 결과셋을 사용한다.
func TestIntegrationRowsStreamingTimeout(t *testing.T) {
	dsn := integrationDSN(t, "readTimeout=1s&timeout=5s")
	man := openIntegrationMan(t, dsn, map[string]QueryStatement{
		// 첫 행은 즉시, 이후 SLEEP으로 스트리밍 중 지연을 유발
		"STREAM": {Id: "stream", Query: "select 1 as n union all select sleep(2)", eleType: eleTypeSelect},
	})
	defer man.db.Close()

	result := man.QueryWithStmt("stream")
	if result.GetError() != nil {
		// 첫 응답 단계에서 났다면 그것도 래핑되어 있어야 한다
		if _, ok := AsQueryError(result.GetError()); !ok {
			t.Fatalf("query-stage error must be wrapped: %v", result.GetError())
		}
		return
	}
	defer result.Close()

	var streamErr error
	for result.Next() {
		var n int
		if err := result.Scan(&n); err != nil {
			streamErr = err
			break
		}
	}
	if streamErr == nil {
		t.Skip("streaming timeout not reproduced in this environment")
	}
	if _, ok := AsQueryError(streamErr); !ok {
		t.Errorf("rows-stage error must be wrapped as QueryError, got: %v", streamErr)
	}
}
