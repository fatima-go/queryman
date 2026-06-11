# release #

## v1.2.0 ##
"invalid connection" 에러 세분화 (enhance-invalid-connection)

추가:
- `QueryError` 타입(StmtId/Op/Elapsed/Err, `Unwrap` 지원) — DB 왕복 에러에 쿼리
  컨텍스트 부여. 에러 메시지가 `[stmtId] op failed after elapsed: 원본` 형식으로 변경
- 분류 헬퍼 `IsConnError`/`IsTimeout`/`AsQueryError`
- 실행 API의 `*Context` 변형(`ExecuteContext`/`QueryContext`/`QueryRowContext` 및
  `*WithStmtContext`, `BeginTx`, `Bulk.ExecuteContext`) — 기존 메서드는 무변경
- `SetDriverLogger` (드라이버 errLog를 서비스 로거로 연결하는 관측성 보조)

수정:
- `QueryResult.Scan()`이 저장된 에러 대신 nil을 반환하던 버그 수정
- 동작하지 않던 `driver.ErrBadConn` 수동 재시도 블록 제거(non-idempotent 위험 제거)

호환성 주의:
- `GetError().Error()` **전체 문자열 `==` 비교**에 의존하던 코드는 메시지 형식 변경으로
  깨질 수 있다(`strings.Contains`는 영향 없음, 원본은 `Unwrap`/`errors.Is`로 접근 가능)
- `ErrNoRows`를 `sql.ErrNoRows` 별칭으로 변경(`errors.Is(err, sql.ErrNoRows)` 성립,
  기존 `err == queryman.ErrNoRows` 비교 보존, 변수 정체성은 변경됨)
- `SqlProxy` 인터페이스에 `*Context` 메서드 3개, `Bulk` 인터페이스에 `ExecuteContext`
  추가(내부 구현만 존재하므로 일반 사용에는 영향 없음)
- 의존성: go 1.24.0, github.com/go-sql-driver/mysql v1.10.0

## v1.1.2 ##
[DB 처리 파라미터의 포인터 출력](https://github.com/fatima-go/queryman/issues/6)

## v1.1.1 ##
- LICENSE.md 추가

## v1.1.0 ##
initial
