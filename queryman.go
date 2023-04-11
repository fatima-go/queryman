//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project queryman
// @author 1100282
// @date 2017. 4. 21. PM 2:47
//

package queryman

import (
	"database/sql"
	"fmt"
	"runtime"
	"strings"
	"time"
)

var queryNormalizer QueryNormalizer

type QueryNormalizer interface {
	normalize(stmt *QueryStatement) error
	resolveHolding(query string) string
}

type QueryMan struct {
	db                 *sql.DB
	preference         QuerymanPreference
	statementMap       map[string]QueryStatement
	fieldNameConverter FieldNameConvertStrategy
	execRecordChan     chan queryExecution
}

func (man *QueryMan) GetSqlCount() int {
	return len(man.statementMap)
}

func (man *QueryMan) GetMaxConnCount() int {
	return man.preference.MaxOpenConns
}

func (man *QueryMan) registStatement(queryStatement QueryStatement) error {
	queryStatement, err := man.buildStatement(queryStatement)
	if err != nil {
		return err
	}

	id := strings.ToUpper(queryStatement.Id)
	if _, exists := man.statementMap[id]; exists {
		return fmt.Errorf("duplicated user statement id : %s", id)
	}

	man.statementMap[id] = queryStatement

	if man.preference.Debug {
		man.preference.DebugLogger.Printf("stmt [%s] loaded", id)
	}

	return nil
}

func (man *QueryMan) buildStatement(queryStatement QueryStatement) (QueryStatement, error) {
	if queryNormalizer == nil {
		queryNormalizer = newNormalizer(man.preference.DriverName)
		if queryNormalizer == nil {
			return queryStatement, fmt.Errorf("not found normalizer for %s", man.preference.DriverName)
		}
	}

	if !queryStatement.HasCondition() {
		err := queryNormalizer.normalize(&queryStatement)
		if err != nil {
			return queryStatement, err
		}
	}

	return queryStatement, nil
}

func (man *QueryMan) Close() error {
	if man.execRecordChan != nil {
		man.execRecordChan <- queryExecution{close: true}
		close(man.execRecordChan)
	}

	return man.db.Close()
}

func (man *QueryMan) exec(query string, args ...interface{}) (sql.Result, error) {
	return man.db.Exec(query, args...)
}

func (man *QueryMan) query(query string, args ...interface{}) (*sql.Rows, error) {
	return man.db.Query(query, args...)
}

func (man *QueryMan) queryRow(query string, args ...interface{}) *sql.Row {
	return man.db.QueryRow(query, args...)
}

func (man *QueryMan) prepare(query string) (*sql.Stmt, error) {
	return man.db.Prepare(query)
}

func (man *QueryMan) isTransaction() bool {
	return false
}

func (man *QueryMan) debugEnabled() bool {
	return man.preference.Debug
}

func (man *QueryMan) debugPrint(format string, params ...interface{}) {
	if man.preference.Debug {
		man.preference.DebugLogger.Printf(format, params...)
	}
}

func (man *QueryMan) recordExcution(stmtId string, start time.Time) {
	if man.execRecordChan != nil {
		man.execRecordChan <- newQueryExecution(stmtId, start)
	}

}

func (man *QueryMan) find(id string) (QueryStatement, error) {
	stmt, ok := man.statementMap[strings.ToUpper(id)]
	if !ok {
		if isUserQuery(id) {
			return buildUserQueryStatement(man, id)
		}
		return stmt, fmt.Errorf("not found query statement for id : %s", id)
	}

	return stmt, nil
}

func isUserQuery(query string) bool {
	if strings.Index(query, " ") > 0 {
		return true
	}
	if strings.Index(query, "\t") > 0 {
		return true
	}
	if strings.Index(query, "\n") > 0 {
		return true
	}
	if strings.Index(query, "\r") > 0 {
		return true
	}
	return false
}

func buildUserQueryStatement(manager *QueryMan, query string) (QueryStatement, error) {
	stmt := QueryStatement{}
	stmt.eleType = getDeclareSqlType(query)
	stmt.Id = query
	stmt.Query = query

	return manager.buildStatement(stmt)
}

func getDeclareSqlType(query string) declareElementType {
	prefix := strings.Trim(query, " \r\n\t")[:10]
	prefix = strings.ToUpper(prefix)
	if strings.HasPrefix(prefix, "SELECT") {
		return eleTypeSelect
	} else if strings.HasPrefix(prefix, "INSERT") {
		return eleTypeInsert
	}
	return eleTypeUpdate
}

func (man *QueryMan) CreateBulk() (Bulk, error) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return man.CreateBulkWithStmt(funcName)
}

func (man *QueryMan) CreateBulkWithStmt(stmtIdOrUserQuery string) (Bulk, error) {
	stmt, err := man.find(stmtIdOrUserQuery)
	if err != nil {
		return nil, err
	}

	if stmt.eleType != eleTypeInsert && stmt.eleType != eleTypeUpdate {
		return nil, ErrExecutionInvalidSqlType
	}

	bulk := newQuerymanBulk(man, stmt)
	return bulk, nil
}

func (man *QueryMan) Execute(v ...interface{}) (sql.Result, error) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return man.ExecuteWithStmt(funcName, v...)
}

func (man *QueryMan) ExecuteWithStmt(stmtIdOrUserQuery string, v ...interface{}) (sql.Result, error) {
	stmt, err := man.find(stmtIdOrUserQuery)
	if err != nil {
		return nil, err
	}

	if stmt.eleType != eleTypeInsert && stmt.eleType != eleTypeUpdate {
		return nil, ErrExecutionInvalidSqlType
	}

	return execute(man, stmt, v...)
}

func (man *QueryMan) Query(v ...interface{}) *QueryResult {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return man.QueryWithStmt(funcName, v...)
}

func (man *QueryMan) QueryWithStmt(stmtIdOrUserQuery string, v ...interface{}) *QueryResult {
	stmt, err := man.find(stmtIdOrUserQuery)
	if err != nil {
		return newQueryResultError(err)
	}

	if stmt.eleType != eleTypeSelect {
		return newQueryResultError(ErrQueryInvalidSqlType)
	}

	queryedRow := queryMultiRow(man, stmt, v...)
	queryedRow.fieldNameConverter = man.fieldNameConverter
	return queryedRow
}

func (man *QueryMan) QueryRow(v ...interface{}) *QueryRowResult {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return man.QueryRowWithStmt(funcName, v...)
}

func (man *QueryMan) QueryRowWithStmt(stmtIdOrUserQuery string, v ...interface{}) *QueryRowResult {
	stmt, err := man.find(stmtIdOrUserQuery)
	if err != nil {
		return newQueryRowResultError(err)
	}

	if stmt.eleType != eleTypeSelect {
		return newQueryRowResultError(ErrQueryInvalidSqlType)
	}

	var queryRowResult *QueryRowResult
	queryResult := queryMultiRow(man, stmt, v...)
	if queryResult.err != nil {
		queryResult.Close()
		queryRowResult = newQueryRowResultError(queryResult.err)
	} else {
		queryRowResult = newQueryRowResult(queryResult.pstmt, queryResult.rows)
	}

	queryResult.pstmt = nil
	queryResult.rows = nil
	queryRowResult.fieldNameConverter = man.fieldNameConverter
	return queryRowResult
}

func (man *QueryMan) Begin() (*DBTransaction, error) {
	tx, err := man.db.Begin()
	if err != nil {
		return nil, err
	}

	runtime.SetFinalizer(tx, closeTransaction)
	return newTransaction(man, tx, man, man.fieldNameConverter), nil
}

// you have to commit before closing transaction
func closeTransaction(tx *sql.Tx) {
	tx.Rollback()
}

func findFunctionName(pc uintptr) string {
	var funcName = runtime.FuncForPC(pc).Name()
	var found = strings.LastIndexByte(funcName, '.')
	if found < 0 {
		return funcName
	}
	return funcName[found+1:]
}
