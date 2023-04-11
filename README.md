# Golang xml base DB Query Manager #

Package throosea.com/queryman implements a xml base RDBMS SQL query infrastructure for Go.
Queryman is an orm like mybatis(java) for golang.

> current : tested with mysql 5.x

below xml file is sample 

```
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
	<insert id="InsertCity">
		INSERT INTO CITY(NAME,AGE,IS_MAN,PERCENTAGE,CREATE_TIME,UPDATE_TIME) VALUES({Name},{Age},{IsMan},{Percentage},{CreateTime},{UpdateTime})
	</insert>
	<update id="UpdateCity">
		UPDATE CITY SET AGE=? WHERE IS_MAN=?
	</update>
	<update id="UpdateCityWithName">
		UPDATE CITY SET AGE={Age} WHERE NAME={Name}
	</update>
	<select id="SelectCityWithName">
		SELECT * FROM CITY WHERE NAME like {Name}
	</select>
	<select id="selectCdataSample">
		SELECT 1 FROM <![CDATA[dual]]>
    </select>
    <select id="selectDynamicQuery">
		SELECT 1 FROM city
		WHERE a={varA}
		<if key="VarB">
			AND b={varB}
		</if>
		<if key="VarK" exist="false">
			AND k={varK}
		</if>
		<if key="VarB">
			AND b={varB}
		</if>
		AND c={varC}
    </select>
</query>
```

tags : insert, update, delete, select

# Install #
```
go get throosea.com/queryman

or

govendor fetch throosea.com/queryman

or

any other vendoring tools...
```


# Working with stmt id #

If you describe SQL in xml, you should define stmt id in xml too.
e.g) <select id='SelectDual'> ... </select>

queryman functions will use function name as stmt id basically.


```
#!go

// ...

func selectDual() {
	// Query will use stmt id as 'selectDual' (function name)
	_, err := queryManager.Query()
	if err != nil {
		log.Error(err.Error())
		return
	}

	// You can specify stmt id with xxxWithStmt(stmtId string, ...)
	_, err := queryManager.QueryWithStmt("selectAnother")
	if err != nil {
		log.Error(err.Error())
		return
	}

```

> **`please note all stmt id will be compared internally CASE INSENSITIVE`**

# Example #

```
#!go

package main

const (
	sqlDropCityTable = "DropCityTable"
	sqlCreateCityTable = "CreateCityTable"
	sqlInsertCity = "InsertCity"
	sqlUpdateCityWithName = "UpdateCityWithName"
	sqlSelectCityWithName = "SelectCityWithName"
	sqlCountCity = "CountCity"
)

var queryManager *QueryMan

func main() {
	// prepare db source uri
	sourceName := "user:pwd@tcp(127.0.0.1:3306)/mydb?timeout=10s"

	// xml file dir
	xmlFileDir := "/my/somewhere"

	// create preference
	querymanPref := NewQuerymanPreference(xmlFileDir, sourceName)
	querymanPref.Fileset = "myquery*.xml"

	// create queryman
	man, err := NewQueryman(querymanPref)
	if err != nil {
		log.Printf("fail to open queryman : %s\n", err.Error())
		return
	}
	queryManager = man

	// do work
	executeWithStmtId()
	queryWithStmtId()
	executeWithFunctionName()
	queryWithFunctionName()
	executeWithParameters()
	insertWithSlice()
	insertWithStruct()
	insertWithMap()
	insertNestedSlice()
	transactionInsert()
	queryToStruct()
	queryWithMap()
	queryRowStruct()
	queryRow()
}

type City struct {
	Id		int
	Name	string
	Age		int
	IsMan	bool
	Percentage float32
	CreateTime time.Time
	UpdateTime time.Time
}

// execute with sql statement id
func executeWithStmtId() error {
	_, err := queryManager.ExecuteWithStmt(sqlDropCityTable)
	if err != nil {
		return fmt.Errorf("fail to execute(%s) : %s\n", sqlDropCityTable, err.Error())
	}

	_, err = queryManager.ExecuteWithStmt(sqlCreateCityTable)
	if err != nil {
		return fmt.Errorf("fail to execute(%s) : %s\n", sqlCreateCityTable, err.Error())
	}

	return nil
}

// query with sql statement id
func queryWithStmtId() {
	city := &City{}
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, "some param")
	if result.GetError() != nil {
		log.Printf("%s", result.GetError())
		return
	}

	defer result.Close()

	if !result.Next() {
		log.Printf("errNoMoreData")
		return
	}

	// queryman Scan into struct
	err := result.Scan(city)
	if err != nil {
		log.Printf("fail to scan : %s", err.Error())
		return
	}
}

// query statement id will be function name
// queryman try to find 'executeWithFunctionName' in query xml
// e.g) <insert id='executeWithFunctionName'> ... </insert>
func executeWithFunctionName() error {
	_, err := queryManager.Execute()
	if err != nil {
		return fmt.Errorf("fail to execute : %s\n", err.Error())
	}

	return nil
}

// query statement id will be function name
// queryman try to find 'queryWithFunctionName' in query xml
// e.g) <select id='queryWithFunctionName'> ... </insert>
func queryWithFunctionName() {
	result := queryManager.Query("first param")
	if result.GetError() != nil {
		log.Printf("%s", result.GetError())
		return
	}

	defer result.Close()
}

// queryman try to find 'executeWithParameters' in query xml
// e.g) <insert id='executeWithParameters'> ... </insert>
func executeWithParameters() {
	// queryManager.execute will use stmt id 'executeWithParameters'
	result, err := queryManager.Execute("first param", 42, true, 40.0, time.Now(), nil)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if id != 1 {
		log.Printf("invalid last insert id : %d", id)
		return
	}
}

// stmt id : insertSlice
// using slice as parameter
func insertWithSlice() {
	args := make([]interface{}, 0)
	args = append(args, "sample city")
	args = append(args, 42)
	args = append(args, true)
	args = append(args, 40.0)
	args = append(args, time.Now())
	args = append(args, nil)
	_, err := queryManager.Execute(args)
	if err != nil {
		log.Printf(err.Error())
		return
	}
}

// stmt id : insertWithStruct
// using Struct as parameter
func insertWithStruct() {
	city := createCity()

	_, err := queryManager.Execute(city)
	if err != nil {
		log.Printf(err.Error())
		return
	}
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

// stmt id : insertWithMap
// using Map as parameter
func insertWithMap() {
	args := make(map[string]interface{})
	args["Name"] = "map name"
	args["Age"] = nil
	args["IsMan"] = true
	args["Percentage"] = 19.21
	args["CreateTime"] = time.Now()
	args["UpdateTime"] = time.Now()

	_, err := queryManager.Execute(args)
	if err != nil {
		log.Printf(err.Error())
		return
	}
}

// stmt id : insertNestedSlice
// using Nested Slice as parameter
func insertNestedSlice() {
	params := make([][]interface{}, 0)

	insertingCount := 5
	for i:=0; i<insertingCount; i++ {
		args := make([]interface{}, 0)
		args = append(args, "sample city")
		args = append(args, 42)
		args = append(args, true)
		args = append(args, 40.0)
		args = append(args, time.Now())
		args = append(args, nil)
		params = append(params, args)
	}

	// internally queryman calls 'pstmt.Execute()' 5 times
	result, err := queryManager.Execute(params)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if int(affected) != insertingCount {
		log.Printf("invalid affected count : %d", insertingCount)
		return
	}

	if pstmtResult, ok := result.(ExecMultiResult); ok  {
		if len(pstmtResult.GetInsertIdList()) != insertingCount {
			log.Printf("inserted id count is not valid. %d", len(pstmtResult.GetInsertIdList()))
		}
	} else {
		log.Printf("result type is not ExecMultiResult")
	}
}

// in transaction, everything is same
func transactionInsert() {
	city := createCity()
	tx, err := queryManager.Begin()
	defer tx.Rollback()
	result, err := tx.ExecuteWithStmt(sqlInsertCity, city)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if id != 1 || affected != 1 {
		log.Printf("invalid result : id=%d, affected=%d", id, affected)
		return
	}
}

// queryman scan row result to struct
func queryToStruct() {
	city := &City{}
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, "first param")
	if result.GetError() != nil {
		log.Printf("%s", result.GetError())
		return
	}

	defer result.Close()

	if !result.Next() {
		log.Printf("errNoMoreData")
		return
	}

	err := result.Scan(city)
	if err != nil {
		log.Printf("fail to scan : %s", err.Error())
		return
	}
}


func queryRow() {
	// insert sample
	_, err := queryManager.ExecuteWithStmt(sqlInsertCity, "sample_city", 42, true, 40.0, time.Now(), time.Now())
	if err != nil {
		log.Printf(err.Error())
		return
	}

	count := 0
	err = queryManager.QueryRow(sqlCountCity).Scan(&count)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if count != 1 {
		log.Printf("invalid city count %d", count)
		return
	}
}

func queryRowStruct() {
	type NullableCity struct {
		Id      sql.NullInt64
		Name    sql.NullString
		Age     sql.NullInt64
		IsMan   sql.NullBool
		Percentage sql.NullFloat64
		CreateTime mysql.NullTime
		UpdateTime mysql.NullTime
	}

	city := NullableCity{}

	err := queryManager.QueryRowWithStmt(sqlSelectCityWithName, "unexported_field").Scan(&city)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if city.Age.Int64 != 42 {
		log.Printf("selecting mismatch")
	}
}

func queryWithMap() {
	m := make(map[string]string)
	m["Name"] = "map_name"
	result := queryManager.QueryWithStmt(sqlSelectCityWithName, m)
	if result.GetError() != nil {
		log.Printf(result.GetError().Error())
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
		log.Printf("errNoMoreData")
		return
	}
	err := result.Scan(&id, &name, &age, &isMan, &percentage, &createTime, &updateTime)
	if err != nil {
		log.Printf(err.Error())
		return
	}
}

```

# Dynamic SQL #

queryman supports '<if>' tag for dynamic sql.
'if' tag has 'exist' attribute present bool. 
> **`if 'exist' arrtibute omitted, default value is TRUE`**
> if you want to use dynamic sql, 
> **`you have to pass parameters as MAP`**


```
<?xml version="1.0" encoding="UTF-8" ?>
<query>
	<select id="loadAllTokens">
        SELECT token
        FROM member
        WHERE token IS NOT NULL
        <if key="OSType">
            AND os_type={OSType}
        </if>
        <if key="OSType" exist="false">
            AND os_type IS NOT NULL
        </if>
    </select>
</query>
```

```
#!go

// ...

func loadAllTokens(item PushItem) {
	// ...

	m := make(map[string]interface{})
	if item.SendTargetOsType != OsTypeAll {
		m["OSType"] = item.SendTargetOsType
	}

    // manipulated query will be 'SELECT token FROM member WHERE token IS NOT NULL AND os_type='some value'
	result := database.Query(m)

	...
}
```

# Queryman Preference Properties #

You can set logging preference. below is preference properties

name     | type   | default | remark
---------:| :----- | :----- | :-----
Fileset  |  string | "*.xml" | file set
DriverName | string | "mysql" | database driver name
ConnMaxLifetime | time.Duration | 60s | max connection life time while idling
MaxIdleConns | int | 1 | max idle db connections
MaxOpenConns | int | 10 | max open db connections
Debug | bool | false | debugging mode
DebugLogger | queryman.Logger | queryman.defaultLogger | debug logger
SlowQueryDuration | time.Duration | 0 | slow query checking time duration
SlowQueryFunc | func | nil | slow query notification func

# Queryman Preference Sample #


```
#!go

// ...

func SlowQuerySampleFunc() {
	path := filepath.Dir(xmlFile)
	pref := NewQuerymanPreference(path, sourceName)
	pref.ConnMaxLifetime = time.Duration(time.Second * 10)
	pref.Fileset = "query*.xml"
	pref.SlowQueryMillis = time.Second * 10
	pref.SlowQueryFunc = loggingSlowQuery

	man, err := NewQueryman(pref)
	if err != nil {
		t.Errorf("fail to create queryman : %s\n", err.Error())
		return
	}

	...
}

func loggingSlowQuery(stmtId string, start time.Time, elapsed time.Duration)	{
	fmt.Printf("slowQuery : id=%s, start=%v, elased=%d milliseconds\n", stmtId, start, elapsed  / 1000000)
}

func DebugSampleFunc() {
	path := filepath.Dir(xmlFile)
	pref := NewQuerymanPreference(path, sourceName)
	pref.ConnMaxLifetime = time.Duration(time.Second * 10)
	pref.Fileset = "query*.xml"
	pref.Debug = true
	pref.DebugLogger = myCustomLogger{}

	man, err := NewQueryman(pref)
	if err != nil {
		t.Errorf("fail to create queryman : %s\n", err.Error())
		return
	}

	...
}

type myCustomLogger struct{}

func (myCustomLogger) Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}


```




