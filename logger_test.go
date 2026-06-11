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
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// capturingLogger mysql.Logger 테스트 더블
type capturingLogger struct {
	msgs []string
}

func (l *capturingLogger) Print(v ...interface{}) {
	l.msgs = append(l.msgs, fmt.Sprint(v...))
}

// TestSetDriverLogger SetDriverLogger가 mysql.SetLogger에 위임되어 커스텀 로거를
// 수용하고 nil은 거부하는지 검증한다 (FR-5.1).
func TestSetDriverLogger(t *testing.T) {
	// nil 로거는 드라이버가 거부한다
	if err := SetDriverLogger(nil); err == nil {
		t.Error("SetDriverLogger(nil) must return error")
	}

	// 유효한 로거는 수용된다
	lg := &capturingLogger{}
	if err := SetDriverLogger(lg); err != nil {
		t.Errorf("SetDriverLogger(valid) must succeed, got: %v", err)
	}

	// 드라이버 로그가 커스텀 로거로 전달된다
	mysql.SetLogger(lg)
	lg2 := &capturingLogger{}
	if err := SetDriverLogger(lg2); err != nil {
		t.Fatalf("reset logger: %v", err)
	}
	// 테스트 격리를 위해 NopLogger로 복원
	if err := SetDriverLogger(&mysql.NopLogger{}); err != nil {
		t.Fatalf("restore NopLogger: %v", err)
	}
}
