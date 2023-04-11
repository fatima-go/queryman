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
	"time"
)

type DBTransaction struct {
	tx                 *sql.Tx
	queryFinder        QueryStatementFinder
	fieldNameConverter FieldNameConvertStrategy
	debugger           SqlDebugger
}

func (t *DBTransaction) Rollback() error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Rollback panic", r)
			return
		}
	}()
	return t.tx.Rollback()
}

func (t *DBTransaction) Commit() error {
	return t.tx.Commit()
}

func newTransaction(debugger SqlDebugger, tx *sql.Tx, queryFinder QueryStatementFinder, fieldNameConverter FieldNameConvertStrategy) *DBTransaction {
	dbTransaction := DBTransaction{}
	dbTransaction.debugger = debugger
	dbTransaction.tx = tx
	dbTransaction.queryFinder = queryFinder
	dbTransaction.fieldNameConverter = fieldNameConverter
	return &dbTransaction
}

func (t *DBTransaction) exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

func (t *DBTransaction) query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

func (t *DBTransaction) queryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(query, args...)
}

func (t *DBTransaction) prepare(query string) (*sql.Stmt, error) {
	return t.tx.Prepare(query)
}

func (t *DBTransaction) isTransaction() bool {
	return true
}

func (t *DBTransaction) debugEnabled() bool {
	return t.debugger.debugEnabled()
}

func (t *DBTransaction) debugPrint(format string, params ...interface{}) {
	t.debugger.debugPrint(format, params...)
}

func (t *DBTransaction) recordExcution(stmtId string, start time.Time) {
	t.debugger.recordExcution(stmtId, start)
}

func (t *DBTransaction) CreateBulk() (Bulk, error) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return t.CreateBulkWithStmt(funcName)
}

func (t *DBTransaction) CreateBulkWithStmt(stmtIdOrUserQuery string) (Bulk, error) {
	stmt, err := t.queryFinder.find(stmtIdOrUserQuery)
	if err != nil {
		return nil, err
	}

	if stmt.eleType != eleTypeInsert && stmt.eleType != eleTypeUpdate {
		return nil, ErrExecutionInvalidSqlType
	}

	bulk := newQuerymanBulk(t, stmt)
	return bulk, nil
}

func (t *DBTransaction) Execute(v ...interface{}) (sql.Result, error) {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return t.ExecuteWithStmt(funcName, v...)
}

func (t *DBTransaction) ExecuteWithStmt(id string, v ...interface{}) (sql.Result, error) {
	stmt, err := t.queryFinder.find(id)
	if err != nil {
		return nil, err
	}

	if stmt.eleType != eleTypeInsert && stmt.eleType != eleTypeUpdate {
		return nil, ErrExecutionInvalidSqlType
	}

	return execute(t, stmt, v...)
}

func (t *DBTransaction) Query(v ...interface{}) *QueryResult {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return t.QueryWithStmt(funcName, v...)
}

func (t *DBTransaction) QueryWithStmt(id string, v ...interface{}) *QueryResult {
	stmt, err := t.queryFinder.find(id)
	if err != nil {
		return newQueryResultError(err)
	}

	if stmt.eleType != eleTypeSelect {
		return newQueryResultError(ErrQueryInvalidSqlType)
	}

	queryedRow := queryMultiRow(t, stmt, v...)
	queryedRow.fieldNameConverter = t.fieldNameConverter
	return queryedRow
}

func (t *DBTransaction) QueryRow(v ...interface{}) *QueryRowResult {
	pc, _, _, _ := runtime.Caller(1)
	funcName := findFunctionName(pc)
	return t.QueryRowWithStmt(funcName, v...)
}

func (t *DBTransaction) QueryRowWithStmt(id string, v ...interface{}) *QueryRowResult {
	stmt, err := t.queryFinder.find(id)
	if err != nil {
		return newQueryRowResultError(err)
	}

	if stmt.eleType != eleTypeSelect {
		return newQueryRowResultError(ErrQueryInvalidSqlType)
	}

	var queryRowResult *QueryRowResult
	queryResult := queryMultiRow(t, stmt, v...)
	if queryResult.err != nil {
		queryRowResult = newQueryRowResultError(queryResult.err)
	} else {
		queryRowResult = newQueryRowResult(queryResult.pstmt, queryResult.rows)
	}

	queryResult.pstmt = nil
	queryResult.rows = nil
	queryRowResult.fieldNameConverter = t.fieldNameConverter
	queryRowResult.SetTransaction()
	return queryRowResult
}
