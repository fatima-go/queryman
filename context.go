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
)

// contextBoundProxy SqlProxy를 감싸 exec/query/prepare 호출을 바인딩된 ctx 기반의
// execContext/queryContext/prepareContext로 라우팅한다. 이를 통해 sql.go의 실행
// 로직을 수정하지 않고도 호출자의 context를 드라이버까지 전파한다.
type contextBoundProxy struct {
	SqlProxy
	ctx context.Context
}

func newContextBoundProxy(proxy SqlProxy, ctx context.Context) *contextBoundProxy {
	return &contextBoundProxy{SqlProxy: proxy, ctx: ctx}
}

func (c *contextBoundProxy) exec(query string, args ...interface{}) (sql.Result, error) {
	return c.SqlProxy.execContext(c.ctx, query, args...)
}

func (c *contextBoundProxy) query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.SqlProxy.queryContext(c.ctx, query, args...)
}

func (c *contextBoundProxy) prepare(query string) (*sql.Stmt, error) {
	return c.SqlProxy.prepareContext(c.ctx, query)
}

// proxyContext proxy에 바인딩된 context를 추출한다. context-bound가 아니면
// context.Background()를 반환한다. prepared statement 실행 경로(pstmt.ExecContext)에서
// 호출자 ctx를 얻기 위해 사용한다.
func proxyContext(proxy SqlProxy) context.Context {
	if bound, ok := proxy.(*contextBoundProxy); ok {
		return bound.ctx
	}
	return context.Background()
}
