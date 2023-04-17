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
	"flag"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	xmlFilePrefix = "query."
)

const (
	sqlDropCityTable          = "DropCityTable"
	sqlCreateCityTable        = "CreateCityTable"
	sqlDropAlbumTable         = "DropAlbumTable"
	sqlCreateAlbumTable       = "CreateAlbumTable"
	sqlInsertCity             = "InsertCity"
	sqlUpdateCityWithName     = "UpdateCityWithName"
	sqlSelectCityWithName     = "SelectCityWithName"
	sqlSelectCityWithInClause = "SelectCityWithInClause"
	sqlCountCity              = "CountCity"
	sqlSelectCityWithIf       = "SelectCityWithIf"
)

const (
	statusDisconnected = iota
	statusNoTable
	statusReady
)

var (
	errNoMoreData = errors.New("no more data")
)

var querymanStatus uint8 = statusDisconnected

var sourceName string
var xmlFile string
var queryManager *QueryMan

type City struct {
	Id         int
	Name       string
	Age        int
	IsMan      bool
	Percentage float32
	CreateTime time.Time
	UpdateTime time.Time
}

// go test -v -db=local -user=local -password=angel -host=127.0.0.1:3306
func TestMain(m *testing.M) {
	prepareSourceName()

	var err error
	xmlFile, err = prepareXmlFile()
	if err != nil {
		fmt.Printf("fail to prepare sample xml file : %s", err.Error())
		return
	}

	code := m.Run()
	os.Remove(xmlFile)
	os.Exit(code)
}

func prepareSourceName() {
	dbName := flag.String("db", "mmate", "database name")
	userName := flag.String("user", "mmate", "Username")
	password := flag.String("password", "angel", "passsword")
	host := flag.String("host", "127.0.0.1:3306", "ip and port")

	flag.Parse()

	sourceName = fmt.Sprintf("%s:%s@tcp(%s)/%s?autocommit=true&timeout=10s&readTimeout=10s&loc=Asia%%2Fseoul&writeTimeout=1s&parseTime=true&charset=utf8mb4,utf8",
		*userName, *password, *host, *dbName)
}

var xmlSample = []byte(`
<?xml version="1.0" encoding="UTF-8" ?>
<query>
    <update id="DropCityTable">
        drop table if exists city
    </update>
    <update id="CreateCityTable">
create table city (
    id  bigint NOT NULL AUTO_INCREMENT,
    name varchar(64) default null,
    age  int  default 0,
    is_man  bool default true,
    percentage float default 0.0,
    create_time datetime default CURRENT_TIMESTAMP,
    update_time datetime,
    primary key (id)
)
    </update>
	<update id="DropAlbumTable">
        drop table if exists album
    </update>
	<select id="SelectAlbumCount">
		SELECT COUNT(*) FROM album
	</select>
    <update id="CreateAlbumTable">
	create table album (
    	id  int,
    	score int,
    	primary key (id)
	)
    </update>
	<insert id="InsertAlbum">
		INSERT INTO album  ( id, score ) VALUES ({Id},{Score})
	</insert>
	<insert id="UpsertAlbum">
		INSERT INTO album  ( id, score
        )
        VALUES
        (
            {Id},
            {Score}
        )
        ON DUPLICATE KEY
        UPDATE
            score = score + VALUES(score)
	</insert>
	<update id="UpdateAlbum">
		UPDATE album SET score={Score} WHERE id={Id}
	</update>
    <insert id="InsertCity">
        INSERT INTO CITY(NAME,AGE,IS_MAN,PERCENTAGE,CREATE_TIME,UPDATE_TIME) VALUES({Name},{Age},{IsMan},{Percentage},{CreateTime},{UpdateTime})
    </insert>
    <update id="UpdateCityWithName">
        UPDATE CITY SET AGE={Age} WHERE NAME={Name}
    </update>
    <select id="SelectCityWithName">
        SELECT * FROM CITY WHERE NAME like {Name}
    </select>
	<select id="SelectCityWithInClause">
        SELECT * FROM CITY WHERE Age > {Age} AND NAME IN ({Names})
    </select>
    <select id="CountCity">
        SELECT Count(*) FROM CITY
    </select>
	<select id="SelectCityWithIf">
        SELECT id, name, age
        FROM city
        WHERE is_man={IsMan}
        <if key="Name">
        AND name={Name}
        </if>
        <if key="Age">
        AND age={Age}
        </if>
    </select>
</query>
`)

func prepareXmlFile() (string, error) {
	tempDir := os.TempDir()
	clearPreviousXmlFiles(tempDir, "*.xml")

	file, _ := ioutil.TempFile(tempDir, xmlFilePrefix)
	xmlFile := file.Name() + ".xml"
	os.Rename(file.Name(), xmlFile)

	err := ioutil.WriteFile(xmlFile, []byte(xmlSample), 0644)
	if err != nil {
		return xmlFile, err
	}

	return xmlFile, nil
}

func clearPreviousXmlFiles(path string, fileset string) {
	var buffer bytes.Buffer
	buffer.WriteString(path)
	buffer.WriteRune(filepath.Separator)
	buffer.WriteString(fileset)
	matches, err := filepath.Glob(buffer.String())
	if err != nil {
		return
	}

	for _, v := range matches {
		os.Remove(v)
	}
}

// func(stmtId string, start time.Time, elapsed time.Duration)
func loggingSlowQuery(stmtId string, start time.Time, elapsed time.Duration) {
	fmt.Printf("slowQuery : stmtId=%s\n", stmtId)
}

func TestConnection(t *testing.T) {
	path := filepath.Dir(xmlFile)
	pref := NewQuerymanPreference(path, sourceName)
	pref.ConnMaxLifetime = time.Duration(time.Second * 10)
	pref.Fileset = xmlFilePrefix + "*.xml"
	pref.Debug = false
	pref.SlowQueryDuration = time.Second * 10
	pref.SlowQueryFunc = loggingSlowQuery

	man, err := NewQueryman(pref)
	if err != nil {
		t.Errorf("fail to create queryman : %s\n", err.Error())
		return
	}
	querymanStatus = statusNoTable
	queryManager = man
}

func TestDDL(t *testing.T) {
	if querymanStatus == statusDisconnected {
		t.Error("querymanager is not ready")
		return
	}

	err := dropAndCreateTable()
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	querymanStatus = statusReady
}

func dropAndCreateTable() error {
	err := dropCityTable()
	if err != nil {
		return fmt.Errorf("fail to execute(%s) : %s\n", sqlDropCityTable, err.Error())
	}

	err = createCityTable()
	if err != nil {
		return fmt.Errorf("fail to execute(%s) : %s\n", sqlCreateCityTable, err.Error())
	}

	err = dropAlbumTable()
	if err != nil {
		return fmt.Errorf("fail to execute(%s) : %s\n", sqlDropCityTable, err.Error())
	}

	err = createAlbumTable()
	if err != nil {
		return fmt.Errorf("fail to execute(%s) : %s\n", sqlCreateCityTable, err.Error())
	}

	return nil
}

func dropCityTable() error {
	_, err := queryManager.Execute()
	return err
}

func createCityTable() error {
	_, err := queryManager.Execute()
	return err
}

func dropAlbumTable() error {
	_, err := queryManager.Execute()
	return err
}

func createAlbumTable() error {
	_, err := queryManager.Execute()
	return err
}

func setup() {
	if querymanStatus < statusReady {
		panic("querymanager is not ready")
		return
	}

	err := dropAndCreateTable()
	if err != nil {
		panic(fmt.Sprintf("%s", err.Error()))
		return
	}
}

func TestQueryUnknownStatementId(t *testing.T) {
	if querymanStatus < statusReady {
		t.Error("querymanager is not ready")
		return
	}

	_, err := queryManager.Execute("UnknownSomethingStatement")
	if err == nil {
		t.Error("queryManager report statement found")
	}
}

func TestInsertBareParams(t *testing.T) {
	setup()

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, "bare param", 42, true, 40.0, time.Now(), nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertSlice(t *testing.T) {
	setup()

	args := make([]interface{}, 0)
	args = append(args, "slice name")
	args = append(args, 42)
	args = append(args, true)
	args = append(args, 40.0)
	args = append(args, time.Now())
	args = append(args, nil)
	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, args)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertSlicePtr(t *testing.T) {
	setup()

	args := make([]interface{}, 0)
	args = append(args, "slice ptr")
	args = append(args, 42)
	args = append(args, true)
	args = append(args, 40.0)
	args = append(args, time.Now())
	args = append(args, nil)
	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, &args)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertObject(t *testing.T) {
	setup()

	city := createCity()

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, city)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertObjectPtr(t *testing.T) {
	setup()

	city := createCity()
	city.Name = "ptr test"
	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, &city)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertMap(t *testing.T) {
	setup()

	args := make(map[string]interface{})
	args["Name"] = "map name"
	args["Age"] = nil
	args["IsMan"] = true
	args["Percentage"] = 19.21
	args["CreateTime"] = time.Now()
	args["UpdateTime"] = time.Now()

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, args)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertNullableSlice(t *testing.T) {
	setup()

	args := make([]interface{}, 0)
	args = append(args, sql.NullString{String: "test_city"})
	args = append(args, sql.NullInt64{})
	args = append(args, sql.NullBool{})
	args = append(args, sql.NullFloat64{})
	args = append(args, time.Now())
	args = append(args, nil)

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, args)
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 {
		t.Errorf("invalid last insert id : %d", id)
		return
	}
}

func TestInsertNestedSlice(t *testing.T) {
	setup()

	params := make([][]interface{}, 0)

	insertingCount := 5

	for i := 0; i < insertingCount; i++ {
		args := make([]interface{}, 0)
		args = append(args, "slice name")
		args = append(args, 42)
		args = append(args, true)
		args = append(args, 40.0)
		args = append(args, time.Now())
		args = append(args, nil)
		params = append(params, args)
	}

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, params)
	if err != nil {
		t.Error(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if int(affected) != insertingCount {
		t.Errorf("invalid affected count : %d", insertingCount)
		return
	}

	if pstmtResult, ok := result.(ExecMultiResult); ok {
		if len(pstmtResult.GetInsertIdList()) != insertingCount {
			t.Errorf("inserted id count is not valid. %d", len(pstmtResult.GetInsertIdList()))
		}
	} else {
		t.Error("result type is not ExecMultiResult")
	}
}

func TestInsertNestedMap(t *testing.T) {
	setup()

	params := make([]map[string]interface{}, 0)
	insertingCount := 5
	for i := 0; i < insertingCount; i++ {
		args := make(map[string]interface{})
		args["Name"] = "nested map"
		args["Age"] = nil
		args["IsMan"] = true
		args["Percentage"] = 19.21
		args["CreateTime"] = time.Now()
		args["UpdateTime"] = time.Now()
		params = append(params, args)
	}

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, params)
	if err != nil {
		t.Error(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if int(affected) != insertingCount {
		t.Errorf("invalid affected count : %d", insertingCount)
		return
	}

	if pstmtResult, ok := result.(ExecMultiResult); ok {
		if len(pstmtResult.GetInsertIdList()) != insertingCount {
			t.Errorf("inserted id count is not valid. %d", len(pstmtResult.GetInsertIdList()))
		}
	} else {
		t.Error("result type is not ExecMultiResult")
	}
}

func TestInsertNestedObject(t *testing.T) {
	setup()

	params := make([]interface{}, 0)
	insertingCount := 5
	for i := 0; i < insertingCount; i++ {
		params = append(params, createCity())
	}

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, params)
	if err != nil {
		t.Error(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if int(affected) != insertingCount {
		t.Errorf("invalid affected count : %d", insertingCount)
		return
	}

	if pstmtResult, ok := result.(ExecMultiResult); ok {
		if len(pstmtResult.GetInsertIdList()) != insertingCount {
			t.Errorf("inserted id count is not valid. %d", len(pstmtResult.GetInsertIdList()))
		}
	} else {
		t.Error("result type is not ExecMultiResult")
	}
}

func TestInsertNestedObjectPtr(t *testing.T) {
	setup()

	params := make([]interface{}, 0)
	insertingCount := 5
	for i := 0; i < 5; i++ {
		city := createCity()
		params = append(params, &city)
	}

	result, err := queryManager.ExecuteWithStmt(sqlInsertCity, params)
	if err != nil {
		t.Error(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if int(affected) != insertingCount {
		t.Errorf("invalid affected count : %d", insertingCount)
		return
	}

	if pstmtResult, ok := result.(ExecMultiResult); ok {
		if len(pstmtResult.GetInsertIdList()) != insertingCount {
			t.Errorf("inserted id count is not valid. %d", len(pstmtResult.GetInsertIdList()))
		}
	} else {
		t.Error("result type is not ExecMultiResult")
	}
}

func TestTransactionInsert(t *testing.T) {
	setup()

	city := createCity()
	tx, err := queryManager.Begin()
	defer tx.Rollback()
	result, err := tx.ExecuteWithStmt(sqlInsertCity, city)
	if err != nil {
		t.Error(err.Error())
		return
	}

	err = tx.Commit()
	if err != nil {
		t.Error(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Error(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Error(err.Error())
		return
	}

	if id != 1 || affected != 1 {
		t.Errorf("invalid result : id=%d, affected=%d", id, affected)
		return
	}
}

func TestQueryButNoMoreData(t *testing.T) {
	setup()

	result := queryManager.QueryWithStmt(sqlSelectCityWithName, "slice name") // time is null
	if result.GetError() != nil {
		t.Error(result.GetError())
		return
	}

	defer result.Close()

	if !result.Next() {
		return
	}

	t.Error("should be no more data")
}

func TestQueryInClauseDelete(t *testing.T) {
	setup()

	queryManager.ExecuteWithStmt(sqlInsertCity, "seoul", 42, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "pusan", 43, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "sejong", 44, true, 40.0, time.Now(), nil)

	sqlStr := "DELETE FROM CITY WHERE NAME IN ( {Names} )"
	names := make([]string, 0)
	names = append(names, "seoul")
	names = append(names, "pusan")
	result, err := queryManager.ExecuteWithStmt(sqlStr, names)
	if err != nil {
		t.Error(err)
		return
	}

	affected, _ := result.RowsAffected()
	if affected != 2 {
		t.Errorf("row affected = %d", affected)
		return
	}
}
func TestQueryInClauseWithList1(t *testing.T) {
	setup()

	queryManager.ExecuteWithStmt(sqlInsertCity, "seoul", 42, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "pusan", 43, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "sejong", 44, true, 40.0, time.Now(), nil)

	age := 10
	sqlStr := "SELECT * FROM CITY WHERE Age > {Age} AND NAME IN ( {Names} )"
	names := make([]string, 0)
	names = append(names, "seoul")
	names = append(names, "pusan")
	result := queryManager.QueryWithStmt(sqlStr, age, names)
	if result.GetError() != nil {
		t.Error(result.GetError())
		return
	}

	defer result.Close()

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	list := make([]NullableCity, 0)
	city := NullableCity{}
	for result.Next() {
		err := result.Scan(&city)
		if err != nil {
			t.Error(err.Error())
			return
		}
		list = append(list, city)
	}

	log.Printf("list : %d\n", len(list))
	for i, v := range list {
		log.Printf("[%d] %v\n", i, v)
	}
}

func TestQueryInClauseWithList2(t *testing.T) {
	setup()

	queryManager.ExecuteWithStmt(sqlInsertCity, "seoul", 42, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "pusan", 43, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "sejong", 44, true, 40.0, time.Now(), nil)

	age := 10
	sqlStr := "SELECT * FROM CITY WHERE NAME IN ({Names}) AND Age > {Age}"
	names := make([]string, 0)
	names = append(names, "seoul")
	names = append(names, "pusan")
	result := queryManager.QueryWithStmt(sqlStr, names, age)
	if result.GetError() != nil {
		t.Error(result.GetError())
		return
	}

	defer result.Close()

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	list := make([]NullableCity, 0)
	city := NullableCity{}
	for result.Next() {
		err := result.Scan(&city)
		if err != nil {
			t.Error(err.Error())
			return
		}
		list = append(list, city)
	}

	log.Printf("list : %d\n", len(list))
	for i, v := range list {
		log.Printf("[%d] %v\n", i, v)
	}
}

func TestQueryInClauseWithMap(t *testing.T) {
	setup()

	queryManager.ExecuteWithStmt(sqlInsertCity, "seoul", 42, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "pusan", 43, true, 40.0, time.Now(), nil)
	queryManager.ExecuteWithStmt(sqlInsertCity, "sejong", 44, true, 40.0, time.Now(), nil)

	m := make(map[string]interface{})
	sqlStr := "SELECT * FROM CITY WHERE NAME IN ({Names}) AND Age > {Age}"
	names := make([]string, 0)
	names = append(names, "seoul")
	names = append(names, "pusan")
	m["Names"] = names
	m["Age"] = 10
	result := queryManager.QueryWithStmt(sqlStr, m)
	if result.GetError() != nil {
		t.Error(result.GetError())
		return
	}

	defer result.Close()

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	list := make([]NullableCity, 0)
	city := NullableCity{}
	for result.Next() {
		err := result.Scan(&city)
		if err != nil {
			t.Error(err.Error())
			return
		}
		list = append(list, city)
	}

	log.Printf("list : %d\n", len(list))
	for i, v := range list {
		log.Printf("[%d] %v\n", i, v)
	}
}

func TestQueryOneObject(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "bare param", 42, true, 40.0, time.Now(), nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	city := &City{}
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, "bare param") // time is null
	if result.GetError() != nil {
		t.Error(result.GetError())
		return
	}

	defer result.Close()

	if !result.Next() {
		t.Error(errNoMoreData)
		return
	}

	err = result.Scan(city)
	if err != nil {
		t.Errorf("fail to scan : %s", err.Error())
		return
	}
}

func TestQueryRowBare(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "sample_city", 42, true, 40.0, time.Now(), time.Now())
	if err != nil {
		t.Error(err.Error())
		return
	}

	count := 0
	err = queryManager.QueryRowWithStmt(sqlCountCity).Scan(&count)
	if err != nil {
		t.Error(err.Error())
		return
	}

	if count != 1 {
		t.Errorf("invalid city count %d", count)
		return
	}
}

func TestQueryRowStruct(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "unexported_field", 42, true, 40.0, time.Now(), time.Now())
	if err != nil {
		t.Error(err.Error())
		return
	}

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	city := NullableCity{}
	err = queryManager.QueryRowWithStmt(sqlSelectCityWithName, "unexported_field").Scan(&city)
	if err != nil {
		t.Error(err.Error())
		return
	}

	if city.Age.Int64 != 42 {
		t.Errorf("selecting mismatch")
	}
}

func TestQueryBare(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "unexported_field", 42, true, 40.0, time.Now(), time.Now())
	if err != nil {
		t.Error(err.Error())
		return
	}

	type HasUnexportedFieldCity struct {
		Name string
		help string
	}

	city := &City{}
	city.Name = "initial city name"
	sample := HasUnexportedFieldCity{Name: "unexported_field"}
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, sample)
	if result.GetError() != nil {
		t.Errorf(result.GetError().Error())
		return
	}

	defer result.Close()
	var id int
	var name string
	var age int
	var isMan bool
	var percentage float32
	var createTime time.Time
	var updateTime time.Time

	if !result.Next() {
		t.Error(errNoMoreData)
		return
	}
	err = result.Scan(&id, &name, &age, &isMan, &percentage, &createTime, &updateTime)
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestQueryWithMap(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "map_name", 42, true, 40.0, time.Now(), time.Now())
	if err != nil {
		t.Error(err.Error())
		return
	}

	m := make(map[string]string)
	m["Name"] = "map_name"
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, m)
	if result.GetError() != nil {
		t.Errorf(result.GetError().Error())
		return
	}

	defer result.Close()
	var id int
	var name string
	var age int
	var isMan bool
	var percentage float32
	var createTime time.Time
	var updateTime time.Time

	if !result.Next() {
		t.Error(errNoMoreData)
		return
	}
	err = result.Scan(&id, &name, &age, &isMan, &percentage, &createTime, &updateTime)
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestQueryNullAndSkipSetting(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "nullable", nil, nil, nil, nil, nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	type NullableCity struct {
		Id         int
		Name       string
		Age        int
		IsMan      bool
		Percentage float32
		CreateTime time.Time
		UpdateTime time.Time
	}

	city := NullableCity{}
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, "nullable")
	if result.GetError() != nil {
		t.Errorf(result.GetError().Error())
		return
	}

	defer result.Close()

	if !result.Next() {
		t.Error(errNoMoreData)
		return
	}

	err = result.Scan(&city)
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestQueryNullScanning(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "nullable", nil, nil, nil, nil, nil)
	if err != nil {
		t.Error(err.Error())
		return
	}

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	city := NullableCity{}
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, "%null%")
	if result.GetError() != nil {
		t.Errorf(result.GetError().Error())
		return
	}

	defer result.Close()

	if !result.Next() {
		t.Error(errNoMoreData)
		return
	}

	err = result.Scan(&city)
	if err != nil {
		t.Error(err.Error())
		return
	}
}

func TestSelectCityWithIf(t *testing.T) {
	setup()

	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "map_name", 42, true, 40.0, time.Now(), time.Now())
	if err != nil {
		t.Error(err.Error())
		return
	}

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	city := NullableCity{}
	m := make(map[string]interface{})
	m["IsMan"] = true

	m["Name"] = "map_name_not_found"
	err = queryManager.QueryRowWithStmt(sqlSelectCityWithIf, m).Scan(&city)
	if err == nil {
		t.Fatalf("should be no rows")
	}

	m["Name"] = "map_name"
	err = queryManager.QueryRowWithStmt(sqlSelectCityWithIf, m).Scan(&city)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !(city.Age.Valid && city.Age.Int64 == 42) {
		t.Fatalf("invalid age")
	}

	m["Age"] = 42
	err = queryManager.QueryRowWithStmt(sqlSelectCityWithIf, m).Scan(&city)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !(city.Age.Valid && city.Age.Int64 == 42) {
		t.Fatalf("invalid age")
	}
}

func TestSelectNoRows(t *testing.T) {
	setup()

	type NullableCity struct {
		Id         sql.NullInt64
		Name       sql.NullString
		Age        sql.NullInt64
		IsMan      sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	city := NullableCity{}
	m := make(map[string]interface{})
	m["IsMan"] = true
	m["Name"] = "map_name_not_found"
	err := queryManager.QueryRowWithStmt(sqlSelectCityWithIf, m).Scan(&city)
	if err == nil {
		t.Fatalf("should be no rows")
	}

	fmt.Printf("%v\n", err)
}

func createCity() City {
	city := City{}
	city.Name = "jin.freestyle@gmail.com"
	city.Age = 142
	city.IsMan = true
	city.Percentage = 43.4
	city.CreateTime = time.Now()
	city.UpdateTime = time.Now()
	return city
}

func TestUpsertAlbum(t *testing.T) {
	setup()

	list := make([]AlbumData, 0)

	list = append(list, AlbumData{Id: 100, Score: 10})
	list = append(list, AlbumData{Id: 200, Score: 31})
	list = append(list, AlbumData{Id: 300, Score: 9})
	list = append(list, AlbumData{Id: 400, Score: 8})
	list = append(list, AlbumData{Id: 500, Score: 7})
	list = append(list, AlbumData{Id: 100, Score: 12})

	affected, err := upsertAlbum(list)
	if err != nil {
		t.Fatalf("fail to UpsertAlbum : %s", err.Error())
	}
	if affected != 7 {
		t.Fatalf("with %d, but %d", 7, affected)
	}
}

func upsertAlbum(list []AlbumData) (int, error) {
	b, err := queryManager.CreateBulk()
	if err != nil {
		return 0, err
	}

	err = b.AddBatch(list)
	if err != nil {
		return 0, err
	}

	result, err := b.Execute()
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

type AlbumData struct {
	Id    int
	Score int
}

func TestBatchInsert(t *testing.T) {
	setup()

	list := make([]AlbumData, 0)

	list = append(list, AlbumData{Id: 100, Score: 110})
	list = append(list, AlbumData{Id: 200, Score: 131})
	list = append(list, AlbumData{Id: 300, Score: 19})
	list = append(list, AlbumData{Id: 400, Score: 18})
	list = append(list, AlbumData{Id: 500, Score: 17})

	affected, err := insertAlbum(list)
	if err != nil {
		t.Fatalf("fail to insertAlbum : %s", err.Error())
	}
	if affected != 5 {
		t.Fatalf("with %d, but %d", 5, affected)
	}

	count := selectAlbumCount()
	if count != 5 {
		t.Fatalf("with %d, but %d", 5, count)
	}

	plist := make([]*AlbumData, 0)

	plist = append(plist, &AlbumData{Id: 1100, Score: 110})
	plist = append(plist, &AlbumData{Id: 1200, Score: 131})

	bulk, err := queryManager.CreateBulkWithStmt("insertAlbum")
	if err != nil {
		t.Fatalf("fail to insertAlbum : %s", err.Error())
	}
	bulk.AddBatch(&AlbumData{Id: 1100, Score: 110})
	bulk.AddBatch(&AlbumData{Id: 1200, Score: 131})
	result, err := bulk.Execute()
	if err != nil {
		t.Fatalf("fail to insertAlbum : %s", err.Error())
	}
	a, err := result.RowsAffected()
	affected = int(a)
	if affected != 2 {
		t.Fatalf("with %d, but %d", 2, affected)
	}

	count = selectAlbumCount()
	if count != 7 {
		t.Fatalf("with %d, but %d", 7, count)
	}
}

func TestBatchInsertWithMap(t *testing.T) {
	setup()

	bulk, err := queryManager.CreateBulkWithStmt("insertAlbum")
	if err != nil {
		t.Fatalf("fail to insertAlbum : %s", err.Error())
	}
	for i := 0; i <= 10; i++ {
		m := make(map[string]interface{})
		m["Id"] = i + 100
		m["Score"] = i + 100 + 5
		bulk.AddBatch(m)
	}

	result, err := bulk.Execute()
	if err != nil {
		t.Fatalf("fail to TestBatchInsertWithMap : %s", err.Error())
	}
	r, err := result.RowsAffected()
	affected := int(r)
	if affected != 11 {
		t.Fatalf("with %d, but %d", 11, affected)
	}

	count := selectAlbumCount()
	if count != 11 {
		t.Fatalf("with %d, but %d", 11, count)
	}
}

func updateAlbum(list []AlbumData) error {
	b, err := queryManager.CreateBulk()
	if err != nil {
		return err
	}

	err = b.AddBatch(list)
	if err != nil {
		return err
	}

	_, err = b.Execute()
	return err
}

func insertAlbum(list []AlbumData) (int, error) {
	b, err := queryManager.CreateBulk()
	if err != nil {
		return 0, err
	}

	err = b.AddBatch(list)
	if err != nil {
		return 0, err
	}

	result, err := b.Execute()
	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	return int(affected), err
}

func updateAlbumPtr(list []*AlbumData) error {
	b, err := queryManager.CreateBulkWithStmt("updateAlbum")
	if err != nil {
		return err
	}

	err = b.AddBatch(list)
	if err != nil {
		return err
	}

	_, err = b.Execute()
	return err
}

func selectAlbumCount() int {
	count := 0
	err := queryManager.QueryRow().Scan(&count)
	if err != nil {
		log.Printf("selectAlbumCount error : %s", err.Error())
		return 0
	}
	return count
}
