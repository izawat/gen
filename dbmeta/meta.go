package dbmeta

import (
	"database/sql"
	"fmt"
	"strings"
	"strconv"

	"github.com/jimsmart/schema"
)

type ModelInfo struct {
	PackageName     string
	StructName      string
	ShortStructName string
	TableName       string
	Fields          []string
}

type ColumnLength struct {
	ColumnName string
	Length     int64
}

// commonInitialisms is a set of common initialisms.
// Only add entries that are highly unlikely to be non-initialisms.
// For instance, "ID" is fine (Freudian code is rare), but "AND" is not.
var commonInitialisms = map[string]bool{
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SSH":   true,
	"TLS":   true,
	"TTL":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
}

var intToWordMap = []string{
	"zero",
	"one",
	"two",
	"three",
	"four",
	"five",
	"six",
	"seven",
	"eight",
	"nine",
}

// Constants for return types of golang
const (
	golangByteArray  = "[]byte"
	gureguNullInt    = "null.Int"
	sqlNullInt       = "sql.NullInt64"
	golangInt        = "int"
	golangInt64      = "int64"
	gureguNullFloat  = "null.Float"
	sqlNullFloat     = "sql.NullFloat64"
	golangFloat      = "float"
	golangFloat32    = "float32"
	golangFloat64    = "float64"
	gureguNullString = "null.String"
	sqlNullString    = "sql.NullString"
	gureguNullTime   = "null.Time"
	golangTime       = "time.Time"
)

// GenerateStruct generates a struct for the given table.
func GenerateStruct(db *sql.DB, tableName string, structName string, pkgName string, jsonAnnotation bool, gormAnnotation bool, gureguTypes bool, validateV9Annotation bool) *ModelInfo {
	cols, _ := schema.Table(db, tableName)
	colLengths := columnLength(db, tableName, cols)
	primaryKeys := primaryKey(db, tableName)
	fields := generateFieldsTypes(db, cols, primaryKeys, colLengths, 0, jsonAnnotation, gormAnnotation, gureguTypes, validateV9Annotation)

	//fields := generateMysqlTypes(db, columnTypes, 0, jsonAnnotation, gormAnnotation, gureguTypes)

	var modelInfo = &ModelInfo{
		PackageName:     pkgName,
		StructName:      structName,
		TableName:       tableName,
		ShortStructName: strings.ToLower(string(structName[0])),
		Fields:          fields,
	}

	return modelInfo
}

// fetch table indexes (MySQL only)
func primaryKey(db *sql.DB, tableName string) []string {
	var primaryKeys []string
	rows, err := db.Query(`
			select
				index_name,
				column_name
			from
				information_schema.statistics
			where
				table_name = ?
			`, tableName)
    if err != nil {
        panic(err.Error())
	}
	for rows.Next() {
		var indexName string
		var columnName string
		err := rows.Scan(&(indexName), &(columnName))
		if err != nil {
			panic(err.Error())
		}
		if indexName == "PRIMARY" {
			primaryKeys = append(primaryKeys, columnName)
		}

	}
	// fmt.Println(primaryKeys)
	return primaryKeys
}

func columnLength(db *sql.DB, tableName string, cols []*sql.ColumnType) []ColumnLength {
	columnLengths := []ColumnLength{}
	rows, err := db.Query(`
			select
				column_name,
				column_type
			from
				information_schema.columns
			where
				table_name = ?
			`, tableName)
    if err != nil {
        panic(err.Error())
	}
	for rows.Next() {
		var columnName string
		var columnType string
		err := rows.Scan(&(columnName), &(columnType))
		if err != nil {
			panic(err.Error())
		}
		// fmt.Println(columnName)
		// fmt.Println(columnType)
		for _, c := range cols {
			columnNameLower := strings.ToLower(columnName)
			if strings.ToLower(c.Name()) == columnNameLower {
				startIndex := strings.Index(columnType, "(") + 1
				lastIndex := strings.LastIndex(columnType, ")")
				if startIndex < 0 || lastIndex < 0 {
					break
				}
				len, _ := strconv.ParseInt(columnType[startIndex:lastIndex], 10, 64)
				columnLength := ColumnLength{columnNameLower, len}
				columnLengths = append(columnLengths, columnLength)
			}
		}
	}
	return columnLengths
}

// Generate fields string
func generateFieldsTypes(db *sql.DB, columns []*sql.ColumnType, primaryKeys []string, columnLengths []ColumnLength, depth int, jsonAnnotation bool, gormAnnotation bool, gureguTypes bool, validateV9Annotation bool) []string {

	//sort.Strings(keys)

	var fields []string
	var field = ""
	for _, c := range columns {
		nullable, _ := c.Nullable()
		colType := c.DatabaseTypeName()
		key := c.Name()
		valueType := sqlTypeToGoType(strings.ToLower(colType), nullable, gureguTypes)
		if valueType == "" { // unknown type
			continue
		}
		fieldName := FmtFieldName(stringifyFirstChar(key))

		var annotations []string
		if gormAnnotation == true {
			if Contains(primaryKeys, strings.ToLower(key)) {
				annotations = append(annotations, fmt.Sprintf("gorm:\"column:%s;primary_key\"", key))
			} else {
				annotations = append(annotations, fmt.Sprintf("gorm:\"column:%s\"", key))
			}
		}
		if validateV9Annotation == true {
			validateRules := []string{}
			if !nullable {
				if strings.ToLower(key) != "created_at" && strings.ToLower(key) != "updated_at" {
					validateRules = append(validateRules, "required")
				}
			}
			if strings.ToLower(colType) == "varchar" {
				for _, colInfo := range columnLengths {
					if colInfo.ColumnName == strings.ToLower(key) {
						validateRules = append(validateRules, fmt.Sprintf("max=%d", colInfo.Length))
					}
				}
			}
			annotations = append(annotations, fmt.Sprintf("validate:\"%s\"", strings.Join(validateRules, ",")))
		}
		if jsonAnnotation == true {
			annotations = append(annotations, fmt.Sprintf("json:\"%s\"", key))
		}
		if len(annotations) > 0 {
			field = fmt.Sprintf("%s %s `%s`",
				fieldName,
				valueType,
				strings.Join(annotations, " "))

		} else {
			field = fmt.Sprintf("%s %s",
				fieldName,
				valueType)
		}

		fields = append(fields, field)
		// fmt.Println(annotations)
	}
	return fields
}

func sqlTypeToGoType(mysqlType string, nullable bool, gureguTypes bool) string {
	switch mysqlType {
	case "tinyint", "int", "smallint", "mediumint":
		if nullable {
			if gureguTypes {
				return gureguNullInt
			}
			return sqlNullInt
		}
		return golangInt
	case "bigint":
		if nullable {
			if gureguTypes {
				return gureguNullInt
			}
			return sqlNullInt
		}
		return golangInt64
	case "char", "enum", "varchar", "longtext", "mediumtext", "text", "tinytext":
		if nullable {
			if gureguTypes {
				return gureguNullString
			}
			return sqlNullString
		}
		return "string"
	case "date", "datetime", "time", "timestamp":
		if nullable && gureguTypes {
			return gureguNullTime
		}
		return golangTime
	case "decimal", "double":
		if nullable {
			if gureguTypes {
				return gureguNullFloat
			}
			return sqlNullFloat
		}
		return golangFloat64
	case "float":
		if nullable {
			if gureguTypes {
				return gureguNullFloat
			}
			return sqlNullFloat
		}
		return golangFloat32
	case "binary", "blob", "longblob", "mediumblob", "varbinary":
		return golangByteArray
	}
	return ""
}
