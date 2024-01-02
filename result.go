/*
 * Copyright 2023 github.com/fatima-go
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @project fatima-core
 * @author jin
 * @date 23. 4. 14. 오후 6:09
 */

package queryman

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
)

type QueryResult struct {
	pstmt              *sql.Stmt
	err                error
	rows               *sql.Rows
	fieldNameConverter FieldNameConvertStrategy
}

func newQueryResultError(err error) *QueryResult {
	queryResult := &QueryResult{}
	queryResult.err = err
	return queryResult
}

func newQueryResult(stmt *sql.Stmt, rows *sql.Rows) *QueryResult {
	queryResult := &QueryResult{}
	queryResult.pstmt = stmt
	queryResult.rows = rows
	return queryResult
}

//func newQueryResult(stmt *sql.Stmt, rows *sql.Rows) *QueryResult {
//	queryResult := &QueryResult{}
//	queryResult.pstmt = stmt
//	queryResult.rows = rows
//	return queryResult
//}

func (r *QueryResult) Next() bool {
	return r.rows.Next()
}

func (r *QueryResult) GetRows() *sql.Rows {
	return r.rows
}

func (r *QueryResult) GetError() (err error) {
	return r.err
}

func (r *QueryResult) Scan(v ...interface{}) (err error) {
	if r.err != nil {
		return err
	}

	if r.rows.Err() != nil {
		return r.rows.Err()
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fail to scan : %s", r)
		}
	}()

	atype := reflect.TypeOf(v[0])

	if atype.Kind() != reflect.Ptr {
		return ErrQueryNeedsPtrParameter
	}

	if reflect.ValueOf(v[0]).IsNil() {
		return ErrNilPtr
	}

	atype = atype.Elem()
	val := reflect.ValueOf(v[0]).Elem()

	switch atype.Kind() {
	case reflect.Interface:
		return ErrInterfaceIsNotSupported
	case reflect.Ptr:
		return ErrPtrIsNotSupported
	case reflect.Struct:
		if _, is := val.Interface().(driver.Valuer); !is {
			return r.scanToStruct(&val)
		}
	}

	return r.rows.Scan(v...)
}

func (r *QueryResult) scanToStruct(val *reflect.Value) error {
	if r.rows.Err() != nil {
		return r.rows.Err()
	}

	columns, err := r.rows.Columns()
	if err != nil {
		return err
	}

	ss := newStructureScanner(r.fieldNameConverter, columns, val)

	return r.rows.Scan(ss.cloneScannerList()...)
}

func (r *QueryResult) Close() error {
	defer func() {
		r.rows = nil
		if r.pstmt != nil {
			r.pstmt.Close()
			r.pstmt = nil
		}
	}()

	if r.rows != nil {
		return r.rows.Close()
	}

	return nil
}

type QueryRowResult struct {
	transaction        bool
	pstmt              *sql.Stmt
	err                error
	rows               *sql.Rows
	fieldNameConverter FieldNameConvertStrategy
}

func newQueryRowResultError(err error) *QueryRowResult {
	queryResult := &QueryRowResult{}
	queryResult.err = err
	return queryResult
}

func newQueryRowResult(stmt *sql.Stmt, rows *sql.Rows) *QueryRowResult {
	queryResult := &QueryRowResult{}
	queryResult.pstmt = stmt
	queryResult.rows = rows
	queryResult.transaction = false
	return queryResult
}

func (r *QueryRowResult) SetTransaction() {
	r.transaction = true
}

func (r *QueryRowResult) Scan(v ...interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fail to scan : %s", r)
		}
	}()

	defer func() {
		if r.rows != nil {
			r.rows.Close()
			r.rows = nil
		}
		if !r.transaction && r.pstmt != nil {
			r.pstmt.Close()
			r.pstmt = nil
		}
	}()

	if r.err != nil {
		return r.err
	}

	if r.rows.Err() != nil {
		return r.rows.Err()
	}

	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return err
		}
		return ErrNoRows
	}

	atype := reflect.TypeOf(v[0])

	if atype.Kind() != reflect.Ptr {
		return ErrQueryNeedsPtrParameter
	}

	if reflect.ValueOf(v[0]).IsNil() {
		return ErrNilPtr
	}

	atype = atype.Elem()
	val := reflect.ValueOf(v[0]).Elem()

	switch atype.Kind() {
	case reflect.Interface:
		return ErrInterfaceIsNotSupported
	case reflect.Ptr:
		return ErrPtrIsNotSupported
	case reflect.Struct:
		if _, is := val.Interface().(driver.Valuer); !is {
			return r.scanToStruct(&val)
		}
	}

	return r.rows.Scan(v...)
}

func (r *QueryRowResult) scanToStruct(val *reflect.Value) error {
	columns, err := r.rows.Columns()
	if err != nil {
		return err
	}

	ss := newStructureScanner(r.fieldNameConverter, columns, val)

	return r.rows.Scan(ss.cloneScannerList()...)
}

type ExecMultiResult struct {
	idList      []int64
	rowAffected int64
}

func (p *ExecMultiResult) addInsertId(id int64) {
	if p.idList == nil {
		p.idList = make([]int64, 0)
	}

	p.idList = append(p.idList, id)
}

func (p ExecMultiResult) GetInsertIdList() []int64 {
	return p.idList
}

func (p ExecMultiResult) LastInsertId() (int64, error) {
	if p.idList == nil || len(p.idList) == 0 {
		return 0, ErrNoInsertId
	}

	return p.idList[0], nil
}

func (p ExecMultiResult) RowsAffected() (int64, error) {
	return p.rowAffected, nil
}
