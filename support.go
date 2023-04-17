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
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
)

const (
	delimStartCharacter = '{'
	delimStartString    = "{"
	delimStopString     = "}"
)

const (
	fieldNameConvertToUnderstore = iota
	fieldNameConvertToCamel
)

type fieldNameConvertMethod uint8

type FieldNameConvertStrategy interface {
	convertFieldName(name string) string
}

type UnderstoreConvertStrategy struct {
}

func (u UnderstoreConvertStrategy) convertFieldName(name string) string {
	// TODO
	return name
}

type CamelConvertStrategy struct {
}

func (u CamelConvertStrategy) convertFieldName(name string) string {
	var buffer bytes.Buffer
	needUpper := true
	for _, c := range name {
		if needUpper {
			buffer.WriteRune(unicode.ToUpper(c))
			needUpper = false
			continue
		}

		if c == '_' {
			needUpper = true
			continue
		}

		buffer.WriteRune(c)
	}

	return buffer.String()
}

/*
MySQL               PostgreSQL            Oracle
=====               ==========            ======
WHERE col = ?       WHERE col = $1        WHERE col = :col
VALUES(?, ?, ?)     VALUES($1, $2, $3)    VALUES(:val1, :val2, :val3)
*/

func newNormalizer(driverName string) QueryNormalizer {
	normalizer := &UserQueryNormalizer{}

	switch strings.ToLower(driverName) {
	case "postgresql":
		normalizer.strategy = &PostgreSQLPlaceholderStrategy{}
	case "oci8":
		normalizer.strategy = &OraclePlaceholderStrategy{}
	default:
		normalizer.strategy = &MysqlPlaceholderStrategy{}
	}

	return normalizer
}

type SqlVariablePlaceholderStrategy interface {
	getNextMark() string
	clone() SqlVariablePlaceholderStrategy
}

type MysqlPlaceholderStrategy struct {
}

func (m *MysqlPlaceholderStrategy) getNextMark() string {
	return "?"
}

func (m *MysqlPlaceholderStrategy) clone() SqlVariablePlaceholderStrategy {
	return &MysqlPlaceholderStrategy{}
}

type PostgreSQLPlaceholderStrategy struct {
	paramIndex int
}

func (p *PostgreSQLPlaceholderStrategy) getNextMark() string {
	val := fmt.Sprintf("$%d", p.paramIndex)
	p.paramIndex++
	return val
}

func (p *PostgreSQLPlaceholderStrategy) clone() SqlVariablePlaceholderStrategy {
	n := &PostgreSQLPlaceholderStrategy{}
	n.paramIndex = 0
	return n
}

type OraclePlaceholderStrategy struct {
	paramIndex int
}

func (o *OraclePlaceholderStrategy) getNextMark() string {
	val := fmt.Sprintf(":val%d", o.paramIndex)
	o.paramIndex++
	return val
}

func (p *OraclePlaceholderStrategy) clone() SqlVariablePlaceholderStrategy {
	n := &OraclePlaceholderStrategy{}
	n.paramIndex = 0
	return n
}

type UserQueryNormalizer struct {
	strategy SqlVariablePlaceholderStrategy
}

// var holdByte byte = '`'
var holdByte byte = 0x0

func (n *UserQueryNormalizer) normalize(stmt *QueryStatement) error {
	stmt.Query = strings.Trim(stmt.Query, " \r\n\t")
	stmt.columnMention = make([]ColumnBind, 0)
	if len(stmt.Query) < 3 {
		return fmt.Errorf("invalid query : %s", stmt.Query)
	}

	var hold bytes.Buffer

	queryLen := len(stmt.Query)
	for i := 0; i < queryLen; i++ {
		ch := stmt.Query[i]
		if ch != delimStartCharacter {
			hold.WriteByte(ch)
			continue
		}

		if i >= queryLen-2 {
			return fmt.Errorf("incompleted variable closer : %s", stmt.Query)
		}
		stopIndex := strings.Index(stmt.Query[i+1:], delimStopString)
		if stopIndex < 1 {
			return fmt.Errorf("incompleted variable closer : %s", stmt.Query)
		}

		v := stmt.Query[i+1 : i+1+stopIndex]
		if strings.Index(v, delimStartString) >= 0 {
			return fmt.Errorf("invalid variable declare format : %s", stmt.Query)
		}

		if isInClause(stmt.Query[:i]) {
			stmt.columnMention = append(stmt.columnMention, NewColumnBindArray(v, hold.Len()+1))
		} else {
			stmt.columnMention = append(stmt.columnMention, NewColumnBind(v, hold.Len()+1))
		}
		i = i + stopIndex + 1
		hold.WriteByte(holdByte)
	}

	stmt.HoldedQuery = hold.String()
	stmt.Query = n.resolveHolding(stmt.HoldedQuery)
	return nil
}

func (n *UserQueryNormalizer) resolveHolding(query string) string {
	var buffer bytes.Buffer

	stgy := n.strategy.clone()
	queryLen := len(query)
	for i := 0; i < queryLen; i++ {
		ch := query[i]
		if ch != holdByte {
			buffer.WriteByte(ch)
			continue
		}

		buffer.WriteString(stgy.getNextMark())
	}

	return buffer.String()
}

func isInClause(sqlPrefix string) bool {
	s := strings.Replace(sqlPrefix, " ", "", -1)
	s = strings.Replace(s, "\n", "", -1)
	s = strings.Replace(s, "\r", "", -1)
	if len(s) < 3 {
		return false
	}
	if strings.ToUpper(s[len(s)-3:]) == "IN(" {
		return true
	}
	return false
}

type StructureScanner struct {
	scanIndex     int
	fieldNameList []string
	source        *reflect.Value
}

func newStructureScanner(converter FieldNameConvertStrategy, columns []string, val *reflect.Value) *StructureScanner {
	ss := &StructureScanner{}
	ss.scanIndex = 0
	ss.fieldNameList = make([]string, len(columns))
	for i := 0; i < len(columns); i++ {
		ss.fieldNameList[i] = converter.convertFieldName(strings.ToLower(columns[i]))
	}
	ss.source = val
	return ss
}

func (ss *StructureScanner) cloneScannerList() []interface{} {
	scanners := make([]interface{}, len(ss.fieldNameList))
	for i := 0; i < len(ss.fieldNameList); i++ {
		scanners[i] = ss
	}
	return scanners
}

// Scan implements the Scanner interface.
func (ss *StructureScanner) Scan(value interface{}) error {
	fieldName := ss.fieldNameList[ss.scanIndex]
	ss.scanIndex++

	targetField := ss.source.FieldByName(fieldName)
	if !targetField.IsValid() || !targetField.CanInterface() {
		return fmt.Errorf("field %s is not exist or settable", fieldName)
	}

	dest := targetField.Addr().Interface()
	if scanner, ok := dest.(sql.Scanner); ok {
		return scanner.Scan(value)
	}

	switch value.(type) {
	case nil:
		return nil // do nothing...
	}

	return convertAssign(dest, value)
}

func currentTimeMillis() int {
	return int(time.Now().UnixNano() / 1000000)
}

func elapsedTimeMillis(startMillis int) int {
	return currentTimeMillis() - startMillis
}
