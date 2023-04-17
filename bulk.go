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
	"strings"
)

type Bulk interface {
	AddBatch(params ...interface{}) error
	Execute() (sql.Result, error)
}

func newQuerymanBulk(sqlProxy SqlProxy, stmt QueryStatement) *querymanBulk {
	b := &querymanBulk{}
	b.sqlProxy = sqlProxy
	b.stmt = stmt
	b.params = make([]interface{}, 0)
	stmt.HasCondition()
	return b
}

type querymanBulk struct {
	stmt      QueryStatement
	sqlProxy  SqlProxy
	params    []interface{}
	execCount int
}

func (b *querymanBulk) String() string {
	return fmt.Sprintf("stmt=[%s], execCount=[%d], params.len=[%d]", b.stmt.Query, b.execCount, len(b.params))
}

func (b *querymanBulk) AddBatch(params ...interface{}) (err error) {
	if len(params) == 0 {
		return nil
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("fail to execute : %s", r)
		}
	}()

	atype := reflect.TypeOf(params[0])
	val := params[0]

	// reform ptr
	if atype.Kind() == reflect.Ptr {
		atype = atype.Elem()
		if reflect.ValueOf(val).IsNil() {
			err = ErrNilPtr
			return
		}
		val = reflect.ValueOf(val).Elem().Interface()
	}

	switch atype.Kind() {
	case reflect.Interface:
		return ErrInterfaceIsNotSupported
	case reflect.Ptr:
		return ErrPtrIsNotSupported
	case reflect.Slice, reflect.Array:
		return b.addList(val)
	case reflect.Struct:
		if _, is := val.(driver.Valuer); !is {
			return b.addWithObject(val)
		}
	case reflect.Map:
		return b.addMap(val)
	}

	return b.addWithList(params)
}

func (b *querymanBulk) Execute() (sql.Result, error) {
	if b.stmt.eleType == eleTypeInsert {
		return b.executeInsert()
	} else if b.stmt.eleType == eleTypeUpdate {
		return b.executeUpdate()
	}

	return nil, fmt.Errorf("only support insert/update")
}

func (b *querymanBulk) executeInsert() (sql.Result, error) {
	bulkInsertQuery := findValuesClauseInInsert(b.stmt.Query)
	sql := bulkInsertQuery.buildMultiValueQuery(b.execCount)
	return b.sqlProxy.exec(sql, b.params...)
}

func (b *querymanBulk) executeUpdate() (sql.Result, error) {

	return nil, fmt.Errorf("not support yet (bulk update)")
}

func (b *querymanBulk) addParams(param ...interface{}) {
	for _, p := range param {
		b.params = append(b.params, p)
	}
	b.execCount = b.execCount + 1
}

func (b *querymanBulk) addList(val interface{}) error {
	if slice, ok := val.([]interface{}); ok {
		return b.addWithList(slice)
	}
	passing := flattenToList(val)
	return b.addWithList(passing)
}

func (b *querymanBulk) addMap(val interface{}) error {
	if m, ok := val.(map[string]interface{}); ok {
		return b.addWithMap(m)
	}
	passing := flattenToMap(val)
	return b.addWithMap(passing)
}

func (b *querymanBulk) addWithObject(parameter interface{}) error {
	m := flattenStructToMap(parameter)
	return b.addWithMap(m)
}

func (b *querymanBulk) addWithMap(m map[string]interface{}) error {
	passing := make([]interface{}, 0)
	for _, v := range b.stmt.columnMention {
		found, ok := m[v.Name()]
		if !ok {
			return fmt.Errorf("addWithMap : not found \"%s\" from parameter values", v)
		}
		passing = append(passing, found)
	}

	b.addParams(passing...)

	return nil
}

func (b *querymanBulk) addWithList(args []interface{}) error {
	atype := reflect.TypeOf(args[0])
	val := args[0]

	// reform ptr
	if atype.Kind() == reflect.Ptr {
		atype = atype.Elem()

		if reflect.ValueOf(args[0]).IsNil() {
			return ErrNilPtr
		}
		val = reflect.ValueOf(val).Elem().Interface()
	}

	// check nested list
	switch atype.Kind() {
	case reflect.Slice:
		return b.addWithNestedList(args)
	case reflect.Struct:
		if _, is := val.(driver.Valuer); !is {
			return b.addWithStructList(args)
		}
	case reflect.Map:
		return b.addWithNestedMap(args)
	}

	if len(b.stmt.columnMention) > len(args) {
		return fmt.Errorf("binding parameter count mismatch. defined=%d, args=%d", len(b.stmt.columnMention), len(args))
	}

	return nil
}

func (b *querymanBulk) addWithNestedList(args []interface{}) error {
	// all data in the list should be 'slice' or 'array'
	for i, v := range args {
		if reflect.TypeOf(v).Kind() != reflect.Slice && reflect.TypeOf(v).Kind() != reflect.Array {
			return fmt.Errorf("nested listing structure should have slice type data only. %d=%s", i, reflect.TypeOf(v).String())
		}
		if len(b.stmt.columnMention) > reflect.ValueOf(v).Len() {
			return fmt.Errorf("binding parameter count mismatch. defined=%d, args[%d]=%d", len(b.stmt.columnMention), i, reflect.ValueOf(v).Len())
		}
	}

	for _, v := range args {
		passing := flattenToList(v)
		b.addParams(passing...)
	}

	return nil
}

func (b *querymanBulk) addWithStructList(args []interface{}) error {
	for _, v := range args {
		atype := reflect.TypeOf(v)
		val := v

		// reform ptr
		if atype.Kind() == reflect.Ptr {
			atype = atype.Elem()
			if reflect.ValueOf(v).IsNil() {
				return ErrNilPtr
			}
			val = reflect.ValueOf(v).Elem().Interface()
		}

		m := flattenStructToMap(val)
		passing := make([]interface{}, 0)

		for _, v := range b.stmt.columnMention {
			found, ok := m[v.Name()]
			if !ok {
				return fmt.Errorf("addWithStructList : not found \"%s\" from parameter values", v)
			}
			passing = append(passing, found)
		}
		b.addParams(passing...)
	}

	return nil
}

func (b *querymanBulk) addWithNestedMap(args []interface{}) error {
	// all data in the list should be 'map'
	for i, v := range args {
		if reflect.TypeOf(v).Kind() != reflect.Map {
			return fmt.Errorf("nested listing structure should have map type data only. %d=%s", i, reflect.TypeOf(v).String())
		}
		if len(b.stmt.columnMention) > reflect.ValueOf(v).Len() {
			return fmt.Errorf("binding parameter count mismatch. defined=%d, args[%d]=%d", len(b.stmt.columnMention), i, reflect.ValueOf(v).Len())
		}
	}

	for _, v := range args {
		m, ok := v.(map[string]interface{})
		if !ok {
			return ErrInvalidMapType
		}

		passing := make([]interface{}, 0)
		for _, v2 := range b.stmt.columnMention {
			found, ok := m[v2.Name()]
			if !ok {
				return fmt.Errorf("not found \"%s\" from map", v)
			}
			passing = append(passing, found)
		}

		b.addParams(passing...)
	}

	return nil
}

func findValuesClauseInInsert(sql string) BulkInsertQuery {
	str := strings.ToLower(sql)
	v := strings.Index(str, "values")
	left := strings.Index(str[v+6:], "(")
	start := v + left + 6
	right := strings.Index(str[start:], ")")
	right = right + 1

	bulk := BulkInsertQuery{}
	bulk.prefix = sql[:start]
	bulk.values = sql[start : start+right]
	bulk.suffix = sql[start+right:]
	return bulk
}

type BulkInsertQuery struct {
	prefix string
	values string
	suffix string
}

func (b BulkInsertQuery) String() string {
	return fmt.Sprintf("prefix=[%s]\nvalues=[%s]\nsuffix=[%s]", b.prefix, b.values, b.suffix)
}

func (b BulkInsertQuery) buildMultiValueQuery(size int) string {
	sql := b.prefix + " " + b.values
	if size < 2 {
		return sql + " " + b.suffix
	}

	for i := 1; i < size; i++ {
		sql = sql + "," + b.values
	}
	return sql + " " + b.suffix
}
