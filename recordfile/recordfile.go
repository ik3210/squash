package recordfile

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
)

//默认值
var Comma = '\t'  //分隔符
var Comment = '#' //注释符

//索引类型
type Index map[interface{}]interface{}

//记录文件类型定义
type RecordFile struct {
	Comma      rune          //分隔符
	Comment    rune          //注释符
	typeRecord reflect.Type  //反射类型
	records    []interface{} //记录切片
	indexes    []Index       //索引切片
}

//创建一个记录文件
func New(st interface{}) (*RecordFile, error) {
	//获取st类型
	typeRecord := reflect.TypeOf(st)

	//判断st合法性，必须是个结构体
	if typeRecord == nil || typeRecord.Kind() != reflect.Struct {
		return nil, errors.New("st must be a struct")
	}

	//遍历结构体内的所有字段，检查类型是否正确
	for i := 0; i < typeRecord.NumField(); i++ {
		//获取字段
		f := typeRecord.Field(i)
		//获取字段类型
		kind := f.Type.Kind()

		switch kind {
		case reflect.Bool: //布尔型
		case reflect.Int: //整型(有符号)
		case reflect.Int8: //有符号8位
		case reflect.Int16: //有符号16位
		case reflect.Int32: //有符号32位
		case reflect.Int64: //有符号64位
		case reflect.Uint: //无符号整型
		case reflect.Uint8: //无符号8位
		case reflect.Uint16: //无符号16位
		case reflect.Uint32: //无符号32位
		case reflect.Uint64: //无符号32位
		case reflect.Float32: //32位浮点数
		case reflect.Float64: //64位浮点数
		case reflect.String: //字符串
		case reflect.Struct: //结构体
		case reflect.Array: //数组
		case reflect.Slice: //切片
		default: //非法类型
			return nil, fmt.Errorf("invalid type: %v %s", f.Name, kind)
		}

		//获取字段标签
		tag := f.Tag

		//如果是索引标签，判断类型
		if tag == "index" {
			switch kind {
			case reflect.Struct, reflect.Array, reflect.Slice: //索引字段不能是结构体、数组、切片
				return nil, fmt.Errorf("could not index %s field %v %v", kind, i, f.Name)
			}
		}
	}

	//创建一个记录文件
	rf := new(RecordFile)
	//保存Type
	rf.typeRecord = typeRecord

	return rf, nil
}

//读取记录文件
func (rf *RecordFile) Read(name string) error {
	//打开文件
	file, err := os.Open(name)

	//打开失败，返回错误
	if err != nil {
		return err
	}

	//延迟关闭文件
	defer file.Close()

	//设置分隔符
	if rf.Comma == 0 {
		rf.Comma = Comma
	}

	//设置注释符
	if rf.Comment == 0 {
		rf.Comment = Comment
	}

	//创建一个csv reader
	reader := csv.NewReader(file)
	//设置分隔符
	reader.Comma = rf.Comma
	//设置注释符
	reader.Comment = rf.Comment
	//读取所有记录
	lines, err := reader.ReadAll()

	//读取失败，返回错误
	if err != nil {
		return err
	}

	//获取Type
	typeRecord := rf.typeRecord
	//创建记录切片，第一行（中文说明字段）不用保存
	records := make([]interface{}, len(lines)-1)
	//创建索引切片
	indexes := []Index{}

	//遍历所有字段，如果字段的标签是索引标签，添加索引到索引切片
	for i := 0; i < typeRecord.NumField(); i++ {
		tag := typeRecord.Field(i).Tag

		if tag == "index" {
			indexes = append(indexes, make(Index))
		}
	}

	//遍历所有记录，一行对应一个typeRecord类型的值
	for n := 1; n < len(lines); n++ {
		//创建一个指针指向特定类型的值
		value := reflect.New(typeRecord)
		//转化指针为interface并保存在records内
		records[n-1] = value.Interface()
		//获取值本身,value是interface或pointer
		record := value.Elem()
		//获取该行记录
		line := lines[n]

		//字段数不匹配，返回错误
		if len(line) != typeRecord.NumField() {
			return fmt.Errorf("line %v, field count mismatch: %v %v", n, len(line), typeRecord.NumField())
		}

		iIndex := 0

		//遍历所有字段，保存字段值
		for i := 0; i < typeRecord.NumField(); i++ {
			//获得字段
			f := typeRecord.Field(i)
			//获得字段值（字符串）
			strField := line[i]
			//获得字段
			field := record.Field(i)

			//如果字段不可设置，继续循环
			if !field.CanSet() {
				continue
			}

			var err error

			//获得字段类型
			kind := f.Type.Kind()

			//根据字段类型，转化字段值类型并保存
			if kind == reflect.Bool { //布尔型，将字段值转化成Bool并保存
				var v bool
				v, err = strconv.ParseBool(strField)

				if err == nil {
					field.SetBool(v)
				}
			} else if kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 || kind == reflect.Int32 || kind == reflect.Int64 { //有符号整型，将字段值转化成Int并保存
				var v int64
				v, err = strconv.ParseInt(strField, 0, f.Type.Bits())

				if err == nil {
					field.SetInt(v)
				}
			} else if kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 || kind == reflect.Uint32 || kind == reflect.Uint64 { //无符号整型，将字段值转化成Uint并保存
				var v uint64
				v, err = strconv.ParseUint(strField, 0, f.Type.Bits())

				if err == nil {
					field.SetUint(v)
				}
			} else if kind == reflect.Float32 || kind == reflect.Float64 { //浮点型，将字段值转化成Float并保存
				var v float64
				v, err = strconv.ParseFloat(strField, f.Type.Bits())

				if err == nil {
					field.SetFloat(v)
				}
			} else if kind == reflect.String { //字符串，直接保存
				field.SetString(strField)
			} else if kind == reflect.Struct || kind == reflect.Array || kind == reflect.Slice { //结构体、数组、切片，用JSON表达
				//解码JSON
				err = json.Unmarshal([]byte(strField), field.Addr().Interface())
			}

			//转化类型出错
			if err != nil {
				return fmt.Errorf("parse field (row=%v, col=%v) error: %v", n, i, err)
			}

			//如果该字段是索引字段，设置索引
			if f.Tag == "index" {
				//获取索引
				index := indexes[iIndex]
				//自增索引切片的索引
				iIndex++

				//多条记录之间的索引字段重复，返回错误
				if _, ok := index[field.Interface()]; ok {
					return fmt.Errorf("index error: duplicate at (row=%v, col=%v)", n, i)
				}

				//保存索引，实际上是保存了一个指针
				index[field.Interface()] = records[n-1]
			}
		}
	}

	//设置记录字段，其实是指向typeRecord类型的值的指针切片
	rf.records = records
	//设置索引字段，一个Index的切片，Index又是一个索引字段值到该行记录的指针的映射
	rf.indexes = indexes

	return nil
}

//获取记录指针
func (rf *RecordFile) Record(i int) interface{} {
	return rf.records[i]
}

//获取记录数目
func (rf *RecordFile) NumRecord() int {
	return len(rf.records)
}

//获取索引（一个map）
func (rf *RecordFile) Indexes(i int) Index {
	if i >= len(rf.indexes) {
		return nil
	}

	return rf.indexes[i]
}

//获取记录指针
func (rf *RecordFile) Index(i interface{}) interface{} {
	//单索引
	index := rf.Indexes(0)

	//没有Index，返回空值
	if index == nil {
		return nil
	}

	//返回该行记录的指针
	return index[i]
}
