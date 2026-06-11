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
)

// TestQueryResultScanReturnsStoredError 에러 상태의 QueryResult에서 Scan 호출 시
// 저장된 에러가 반환되는지 검증한다 (FR-1.1 — named return shadowing 버그 회귀 방지).
func TestQueryResultScanReturnsStoredError(t *testing.T) {
	sentinel := errors.New("stored query error")
	result := newQueryResultError(sentinel)

	var dest int
	err := result.Scan(&dest)
	if err == nil {
		t.Fatal("Scan must return the stored error, but returned nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("Scan must return the stored error, got: %v", err)
	}
}

// TestQueryResultNextOnErrorStateReturnsFalse 에러 상태(rows == nil)의 QueryResult에서
// Next 호출 시 패닉 없이 false를 반환하는지 검증한다 (FR-6.1).
func TestQueryResultNextOnErrorStateReturnsFalse(t *testing.T) {
	result := newQueryResultError(errors.New("any error"))

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Next on error-state QueryResult must not panic, but panicked: %v", r)
		}
	}()

	if result.Next() {
		t.Fatal("Next on error-state QueryResult must return false")
	}
}
