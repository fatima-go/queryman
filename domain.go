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
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	eleTypeUnknown = iota
	eleTypeInsert
	eleTypeUpdate
	eleTypeSelect
	eleTypeIf
)

type declareElementType uint8

func (d declareElementType) String() string {
	switch d {
	case eleTypeInsert:
		return "INSERT"
	case eleTypeUpdate:
		return "UPDATE"
	case eleTypeSelect:
		return "SELECT"
	case eleTypeIf:
		return "IF"
	}
	return "UNKNOWN"
}

func (d declareElementType) IsSql() bool {
	if d == eleTypeInsert || d == eleTypeUpdate || d == eleTypeSelect {
		return true
	}
	return false
}

func buildElementType(stmt string) declareElementType {
	switch strings.ToLower(stmt) {
	case "select":
		return eleTypeSelect
	case "insert":
		return eleTypeInsert
	case "update":
		return eleTypeUpdate
	case "delete":
		return eleTypeUpdate
	case "if":
		return eleTypeIf
	}
	return eleTypeUnknown
}

var (
	ErrInterfaceIsNotSupported    = errors.New("not supported type : interface")
	ErrPtrIsNotSupported          = errors.New("not supported type : ptr")
	ErrNeedStructSliceParam       = errors.New("parameter not supported type : struct slice/array only")
	ErrInvalidMapKeyType          = errors.New("map key should be string")
	ErrInvalidMapType             = errors.New("map only accepted [string]interface{} type")
	ErrExecutionInvalidSqlType    = errors.New("invalid execution for sql. only insert or update permitted")
	ErrQueryInvalidSqlType        = errors.New("invalid query for sql. only select permitted")
	ErrQueryInsufficientParameter = errors.New("insufficient query parameter for select result")
	ErrQueryNeedsPtrParameter     = errors.New("when you select in query, you have to pass parameter as ptr")
	ErrNilPtr                     = errors.New("destination pointer is nil")
	ErrNoRows                     = errors.New("sql: no rows in result set")
	ErrNoInsertId                 = errors.New("sql: no insert id")
)

type SqlProxy interface {
	exec(query string, args ...interface{}) (sql.Result, error)
	query(query string, args ...interface{}) (*sql.Rows, error)
	queryRow(query string, args ...interface{}) *sql.Row
	prepare(query string) (*sql.Stmt, error)
	isTransaction() bool
	SqlDebugger
}

type SqlDebugger interface {
	debugEnabled() bool
	debugPrint(string, ...interface{})
	recordExcution(stmtId string, start time.Time)
}

type QueryStatementFinder interface {
	find(id string) (QueryStatement, error)
}

type QueryStatement struct {
	eleType       declareElementType
	Id            string     `xml:"id,attr"`
	Query         string     `xml:",cdata"`
	clause        []IfClause `xml:"if"`
	columnMention []ColumnBind
	HoldedQuery   string
}

func (q QueryStatement) hasArrayBind() bool {
	if q.columnMention == nil {
		return false
	}

	for _, v := range q.columnMention {
		if v.bindType == columnBindTypeArray {
			return true
		}
	}

	return false
}

func (q QueryStatement) firstArgsIsArray() bool {
	if q.columnMention != nil && len(q.columnMention) > 0 {
		if q.columnMention[0].bindType == columnBindTypeArray {
			return true
		}
	}
	return false
}

func (q QueryStatement) String() string {
	return fmt.Sprintf("eleType=[%s], id=[%s], query=[%s], caluse=[%v], columns=[%v], hold=[%s]",
		q.eleType, q.Id, q.Query, q.clause, q.columnMention, q.HoldedQuery)
}

const (
	columnBindTypeNormal = iota
	columnBindTypeArray
)

type columnBindType uint8

func (c columnBindType) String() string {
	switch c {
	case columnBindTypeNormal:
		return "NORMAL"
	case columnBindTypeArray:
		return "ARRAY"
	}
	return "UNKNOWN"
}

type ColumnBind struct {
	name     string
	holdPos  int
	bindType columnBindType
}

func NewColumnBind(name string, pos int) ColumnBind {
	b := ColumnBind{}
	b.name = name
	b.holdPos = pos
	b.bindType = columnBindTypeNormal
	return b
}

func NewColumnBindArray(name string, pos int) ColumnBind {
	b := ColumnBind{}
	b.name = name
	b.holdPos = pos
	b.bindType = columnBindTypeArray
	return b
}

func (c ColumnBind) Name() string {
	return c.name
}

func (c ColumnBind) String() string {
	return fmt.Sprintf("%s/%d/%s", c.name, c.holdPos, c.bindType)
}

func (stmt QueryStatement) clone() QueryStatement {
	clone := QueryStatement{}
	clone.eleType = stmt.eleType
	clone.Id = stmt.Id
	clone.Query = stmt.Query
	clone.HoldedQuery = stmt.HoldedQuery
	clone.clause = make([]IfClause, 0)
	for _, v := range stmt.clause {
		clone.clause = append(clone.clause, v)
	}
	clone.columnMention = make([]ColumnBind, 0)
	for _, v := range stmt.columnMention {
		clone.columnMention = append(clone.columnMention, v)
	}
	return clone
}

func (stmt QueryStatement) Debug(param ...interface{}) string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("[%s] %s", stmt.Id, stmt.Query))
	debugParam := DebugPrintParams(stmt.Id, param)
	if len(debugParam) > 0 {
		buffer.WriteString(debugParam)
	}
	return buffer.String()
}

func DebugPrintParams(id string, params []interface{}) string {
	if len(params) == 0 {
		return ""
	}

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("\n[%s] params : ", id))
	for _, v := range params {
		switch reflect.TypeOf(v).Kind() {
		case reflect.Ptr:
			if reflect.ValueOf(v).IsNil() {
				buffer.WriteString("[nil] ")
				break
			}
			buffer.WriteString(fmt.Sprintf("[%v] ", reflect.ValueOf(v).Elem().Interface()))
		default:
			buffer.WriteString(fmt.Sprintf("[%v] ", v))
		}
	}

	return buffer.String()
}

func (stmt QueryStatement) HasCondition() bool {
	if len(stmt.clause) > 0 {
		return true
	}
	return false
}

// if condition 처리를 통해 SQL 을 재구성한다
func (stmt QueryStatement) RefineStatement(params map[string]interface{}) (QueryStatement, error) {
	refined := stmt.clone()
	for _, v := range stmt.clause {
		if params == nil {
			refined.Query = strings.Replace(refined.Query, v.id, "", -1)
			continue
		}

		_, ok := params[v.key]
		if v.exist {
			if ok {
				refined.Query = strings.Replace(refined.Query, v.id, v.query, -1)
			} else {
				refined.Query = strings.Replace(refined.Query, v.id, "", -1)
			}
		} else {
			if !ok {
				refined.Query = strings.Replace(refined.Query, v.id, v.query, -1)
			} else {
				refined.Query = strings.Replace(refined.Query, v.id, "", -1)
			}
		}
	}
	err := queryNormalizer.normalize(&refined)
	return refined, err
}

func (stmt *QueryStatement) appendIf(clause IfClause) {
	stmt.clause = append(stmt.clause, clause)
}

type IfClause struct {
	id    string
	key   string
	query string
	exist bool
}

func newIfClause(key string, sql string, exist string) IfClause {
	c := IfClause{}
	c.id = fmt.Sprintf("%s%d%s", ifClauseWrappingKey, generateIfClauseSeq(), ifClauseWrappingKey)
	c.key = key
	c.query = sql
	c.exist = true
	if len(exist) > 0 && strings.ToLower(exist) != "true" {
		c.exist = false
	}

	return c
}

var ifClauseSeq = 0

const ifClauseWrappingKey = "\x00"

func generateIfClauseSeq() int {
	defer func() {
		ifClauseSeq = ifClauseSeq + 1
	}()

	return ifClauseSeq
}
