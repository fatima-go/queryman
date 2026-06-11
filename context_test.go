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

// TestExecContextPropagatesCancellation execContext가 ctx를 database/sql까지 전파해
// 취소가 반영되는지 검증한다 (FR-4.1/4.3).
func TestExecContextPropagatesCancellation(t *testing.T) {
	stub.reset()
	man := &QueryMan{db: newStubDB(t)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	_, err := man.execContext(ctx, "insert into t values (1)")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("execContext must propagate ctx cancellation, got: %v", err)
	}
}

// TestQueryContextPropagatesCancellation queryContext가 ctx를 전파하는지 검증한다.
func TestQueryContextPropagatesCancellation(t *testing.T) {
	stub.reset()
	man := &QueryMan{db: newStubDB(t)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := man.queryContext(ctx, "select 1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("queryContext must propagate ctx cancellation, got: %v", err)
	}
}

// TestExecDelegatesViaBackground 기존 exec(컨텍스트 없는 경로)가 Background 위임으로
// 정상 동작하는지 검증한다 (FR-4.2 — 기존 메서드 무변경 동치).
func TestExecDelegatesViaBackground(t *testing.T) {
	stub.reset()
	man := &QueryMan{db: newStubDB(t)}

	if _, err := man.exec("insert into t values (1)"); err != nil {
		t.Fatalf("exec via Background delegation must succeed, got: %v", err)
	}
}
