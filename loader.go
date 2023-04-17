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
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Logger is an interface that can be implemented to provide custom log output.
type Logger interface {
	Printf(string, ...interface{})
}

type queryExecution struct {
	close  bool
	start  time.Time
	elased time.Duration
	stmtId string
}

func newQueryExecution(stmtId string, start time.Time) queryExecution {
	e := queryExecution{}
	e.close = false
	e.stmtId = stmtId
	e.start = start
	e.elased = time.Duration(time.Now().UnixNano() - start.UnixNano())
	return e
}

func (s queryExecution) String() string {
	if s.close {
		return ""
	}

	return fmt.Sprintf("[%s] elased %d milliseconds", s.stmtId, s.elased/1000000)
}

type defaultLogger struct{}

func (defaultLogger) Printf(format string, a ...interface{}) {
	// do nothing...
	log.Printf(format, a...)
}

type QuerymanPreference struct {
	queryFilePath     string
	Fileset           string
	DriverName        string
	dataSourceUrl     string
	ConnMaxLifetime   time.Duration
	MaxIdleConns      int
	MaxOpenConns      int
	Debug             bool
	DebugLogger       Logger
	SlowQueryDuration time.Duration
	SlowQueryFunc     func(stmtId string, start time.Time, elapsed time.Duration)
	fieldNameConvert  fieldNameConvertMethod
}

func NewQuerymanPreference(filepath string, dataSourceUrl string) QuerymanPreference {
	pref := QuerymanPreference{}
	pref.queryFilePath = filepath
	pref.Fileset = "*.xml"
	pref.DriverName = "mysql" // default
	pref.dataSourceUrl = dataSourceUrl
	pref.ConnMaxLifetime = time.Duration(time.Second * 60)
	pref.MaxIdleConns = 1
	pref.MaxOpenConns = 10
	pref.Debug = false
	pref.SlowQueryDuration = 0
	pref.DebugLogger = defaultLogger{}
	pref.fieldNameConvert = fieldNameConvertToCamel

	return pref
}

func NewQueryman(pref QuerymanPreference) (*QueryMan, error) {
	manager := &QueryMan{}
	manager.preference = pref
	manager.statementMap = make(map[string]QueryStatement)

	db, err := sql.Open(pref.DriverName, pref.dataSourceUrl)
	if err != nil {
		return nil, fmt.Errorf("fail to open sql : %s", err.Error())
	}

	manager.db = db
	manager.db.SetConnMaxLifetime(pref.ConnMaxLifetime)
	manager.db.SetMaxOpenConns(pref.MaxOpenConns)
	manager.db.SetMaxIdleConns(pref.MaxIdleConns)
	manager.fieldNameConverter = newFieldNameConverter(pref.fieldNameConvert)

	err = loadXmlFile(manager, pref.queryFilePath, pref.Fileset)
	if err != nil {
		return nil, fmt.Errorf("fail to load xml file : %s [path=%s,fileset=%s]", err.Error(), pref.queryFilePath, pref.Fileset)
	}

	runtime.SetFinalizer(manager, closeQueryman)

	if manager.preference.SlowQueryDuration > 0 && manager.preference.SlowQueryFunc != nil {
		manager.execRecordChan = make(chan queryExecution, int(math.MaxUint16))
		go func() {
			r := <-manager.execRecordChan
			if r.close {
				return
			}

			if r.elased > manager.preference.SlowQueryDuration {
				manager.preference.SlowQueryFunc(r.stmtId, r.start, r.elased)
			}
		}()
	}

	return manager, nil
}

func newFieldNameConverter(fieldNameConvert fieldNameConvertMethod) FieldNameConvertStrategy {
	switch fieldNameConvert {
	case fieldNameConvertToUnderstore:
		return UnderstoreConvertStrategy{}
	}

	return CamelConvertStrategy{}
}

func loadXmlFile(manager *QueryMan, filePath string, fileSet string) error {
	var buffer bytes.Buffer
	buffer.WriteString(filePath)
	buffer.WriteRune(filepath.Separator)
	buffer.WriteString(fileSet)
	matches, err := filepath.Glob(buffer.String())
	if err != nil {
		return fmt.Errorf("fail to search xml file : %s [glob=%s]", err.Error(), buffer.String())
	}

	for _, file := range matches {
		if !strings.HasSuffix(file, "xml") {
			continue
		}

		data, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("fail to read file[%s] : %s", file, err.Error())
		}

		err = loadWithSax(manager, data)
		if err != nil {
			return err
		}
	}

	return nil
}

func loadWithSax(manager *QueryMan, data []byte) error {
	stmtList = make([]QueryStatement, 0)
	buf := bytes.NewBuffer(data)
	dec := xml.NewDecoder(buf)

	for {
		t, tokenErr := dec.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			return tokenErr
		}

		switch t := t.(type) {
		case xml.StartElement:
			currentId = getAttr(t.Attr, attrId)
			currentEleType = buildElementType(t.Name.Local)
			if currentEleType.IsSql() {
				currentStmt = newQueryStatement(currentEleType)
				traverseIf(dec)
			}
		case xml.CharData:
			if len(currentId) == 0 {
				break
			}
			currentStmt.Query = currentStmt.Query + string(t)
		case xml.EndElement:
			if currentEleType.IsSql() {
				currentStmt.Query = strings.Trim(currentStmt.Query, cutset)
				currentId = ""
			}
		}
	}

	for _, v := range stmtList {
		err := manager.registStatement(v)
		if err != nil {
			return err
		}
	}

	return nil
}

func newQueryStatement(sqlType declareElementType) QueryStatement {
	stmt := QueryStatement{}
	stmt.eleType = sqlType
	stmt.Id = currentId
	stmt.clause = make([]IfClause, 0)
	stmt.columnMention = make([]ColumnBind, 0)
	return stmt
}

const (
	attrId    = "id"
	attrKey   = "key"
	attrExist = "exist"
	cutset    = "\r\t\n "
)

var (
	currentStmt    QueryStatement
	currentEleType declareElementType
	currentId      string
	stmtList       []QueryStatement
)

func getAttr(attr []xml.Attr, name string) string {
	for _, v := range attr {
		if v.Name.Local == name {
			return v.Value
		}
	}
	return ""
}

func traverseIf(dec *xml.Decoder) {
	var innerElement declareElementType
	var innerSql = ""
	var innerKey = ""
	var innerExist = "true"

	for {
		t, tokenErr := dec.Token()
		if tokenErr != nil {
			if tokenErr == io.EOF {
				break
			}
			panic(tokenErr)
		}

		switch t := t.(type) {
		case xml.StartElement:
			innerKey = getAttr(t.Attr, attrKey)
			innerExist = getAttr(t.Attr, attrExist)
			innerElement = buildElementType(t.Name.Local)
		case xml.CharData:
			if innerElement == eleTypeIf {
				innerSql = innerSql + " " + strings.Trim(string(t), cutset)
			} else {
				currentStmt.Query = currentStmt.Query + string(t)
			}
		case xml.EndElement:
			if innerElement == eleTypeIf {
				ifclause := newIfClause(innerKey, innerSql, innerExist)
				currentStmt.Query = fmt.Sprintf("%s %s", currentStmt.Query, ifclause.id)
				currentStmt.appendIf(ifclause)
				innerSql = ""
				innerElement = eleTypeUnknown
			} else if currentEleType.IsSql() {
				currentStmt.Query = strings.Trim(currentStmt.Query, cutset)
				stmtList = append(stmtList, currentStmt)
				return
			}
			currentId = ""
		}
	}
}

func closeQueryman(manager *QueryMan) {
	manager.Close()
}
