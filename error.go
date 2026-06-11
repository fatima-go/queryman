/*
 * Copyright (c) 2026 DREAMUS COMPANY.
 * All right reserved.
 * This software is the confidential and proprietary information of DREAMUS COMPANY.
 * You shall not disclose such Confidential Information and
 * shall use it only in accordance with the terms of the license agreement
 * you entered into with DREAMUS COMPANY.
 */

// Package queryman 의 에러 분류 유틸리티.
// DB 왕복에서 발생한 에러에 쿼리 컨텍스트(stmt id, 작업 종류, 소요 시간)를 부여하고,
// 호출자가 드라이버를 직접 import하지 않고도 연결/타임아웃 계열을 분류할 수 있게 한다.
package queryman

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
)

// QueryError DB 왕복 실패에 쿼리 컨텍스트를 부여하는 에러 타입.
// Unwrap을 통해 원본 에러(드라이버/database/sql)를 보존하므로
// errors.Is/As 체인 검사가 그대로 동작한다.
type QueryError struct {
	StmtId  string        // query XML statement id (예: selectTrackMeta)
	Op      string        // "query" | "exec" | "prepare"
	Elapsed time.Duration // 실행 시작부터 에러 발생까지 경과 시간
	Err     error         // 원본 에러
}

// Error 쿼리 컨텍스트를 포함한 메시지를 반환한다.
// 보안을 위해 SQL 본문과 바인딩 파라미터 값은 포함하지 않는다.
func (e *QueryError) Error() string {
	return fmt.Sprintf("[%s] %s failed after %s: %s",
		e.StmtId, e.Op, e.Elapsed.Round(time.Millisecond), e.Err)
}

// Unwrap 원본 에러를 반환한다.
func (e *QueryError) Unwrap() error {
	return e.Err
}

// wrapQueryError DB 왕복에서 발생한 에러만 QueryError로 래핑한다.
// nil, queryman 센티널(ErrNoRows 등), 이미 래핑된 *QueryError는 그대로 반환한다.
func wrapQueryError(stmtId, op string, start time.Time, err error) error {
	if err == nil {
		return nil
	}
	if isNonWrapSentinel(err) {
		return err
	}
	if _, ok := AsQueryError(err); ok {
		// 이미 래핑됨. 이중 래핑 방지
		return err
	}
	return &QueryError{
		StmtId:  stmtId,
		Op:      op,
		Elapsed: time.Since(start),
		Err:     err,
	}
}

// nonWrapSentinels DB 왕복 에러로 래핑하지 않는 queryman 센티널 목록.
// ErrNoRows(=sql.ErrNoRows)는 정상적인 "결과 없음" 신호이므로 래핑 대상이 아니다.
var nonWrapSentinels = []error{
	ErrNoRows,
	ErrNoInsertId,
}

// isNonWrapSentinel err가 래핑 제외 대상 센티널인지 판별한다.
func isNonWrapSentinel(err error) bool {
	for _, sentinel := range nonWrapSentinels {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// IsConnError 연결 계열 장애인지 판별한다.
// 수립된 연결의 무효화(ErrInvalidConn/ErrBadConn)뿐 아니라 연결 수립(dial) 실패
// (connection refused/unreachable/dial timeout)까지 포함한다.
// 단, DSN readTimeout은 드라이버가 원인을 소실해 ErrInvalidConn으로 도달하므로
// IsConnError=true가 되지만 IsTimeout으로는 판별되지 않는다.
func IsConnError(err error) bool {
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// IsTimeout 확정적으로 판별 가능한 timeout인지 검사한다.
// context.DeadlineExceeded와 net.Error.Timeout()(os.ErrDeadlineExceeded 계열 포함)을
// 감지한다. DSN readTimeout은 원인이 소실되므로 여기서 true가 되지 않는다(의도된 동작).
func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// AsQueryError 에러 체인에서 *QueryError 추출을 시도한다.
func AsQueryError(err error) (*QueryError, bool) {
	var qe *QueryError
	ok := errors.As(err, &qe)
	return qe, ok
}

// SetDriverLogger go-sql-driver/mysql의 내부 errLog를 서비스 로거로 연결한다.
// DSN readTimeout 초과 시 드라이버가 errLog에만 남기고 버리는 실제 원인
// (예: "read tcp ...: i/o timeout")을 같은 로그 스트림에서 관찰할 수 있게 해준다.
//
// 주의: 전역(per-driver) 설정이며 특정 쿼리와의 상관관계는 시간 근접성으로만 추정
// 가능하다. 본질적 timeout 구분 수단이 아닌 관측성 보조 수단으로만 사용한다.
// (드라이버 v1.9+의 연결 단위 Config.Logger는 DSN 문자열 wiring에서 지정할 수 없어
// 사용하지 않는다.)
func SetDriverLogger(logger mysql.Logger) error {
	return mysql.SetLogger(logger)
}
