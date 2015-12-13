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

var (
	Comma   = '\t' //默认分隔符
	Comment = '#'  //默认注释符
)

//索引
type Index map[interface{}]interface{}

//记录文件
type RecordFile struct {
	Comma      rune          //分隔符
	Comment    rune          //注释符
	typeRecord reflect.Type  //反射类型
	records    []interface{} //记录切片
	indexes    []Index       //索引切片
}

//根据指定的结构体，创建一个记录文件
func New(st interface{}) (*RecordFile, error) {
	//获取类型
	typeRecord := reflect.TypeOf(st)
	//检查类型合法性（必须是个结构体）
	if typeRecord == nil || typeRecord.Kind() != reflect.Struct {
		return nil, errors.New("st must be a struct")
	}

	//检查结构体中的字段类型合法性
	for i := 0; i < typeRecord.NumField(); i++ {
		//获取字段
		f := typeRecord.Field(i)
		//获取字段类型
		kind := f.Type.Kind()

		switch kind {
		case reflect.Bool:
		case reflect.Int:
		case reflect.Int8:
		case reflect.Int16:
		case reflect.Int32:
		case reflect.Int64:
		case reflect.Uint:
		case reflect.Uint8:
		case reflect.Uint16:
		case reflect.Uint32:
		case reflect.Uint64:
		case reflect.Float32:
		case reflect.Float64:
		case reflect.String:
		case reflect.Struct:
		case reflect.Array:
		case reflect.Slice:
		default: //非法类型
			return nil, fmt.Errorf("invalid type: %v %s", f.Name, kind)
		}

		//获取字段标签
		tag := f.Tag
		//标签是"index"，检查字段类型合法性（索引字段不能是结构体、数组、切片）
		if tag == "index" {
			switch kind {
			case reflect.Struct, reflect.Array, reflect.Slice:
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
	//打开失败
	if err != nil {
		return err
	}

	//延迟关闭文件
	defer file.Close()

	//分隔符未设置，采用默认分隔符
	if rf.Comma == 0 {
		rf.Comma = Comma
	}

	//注释符未注释，采用默认注释符
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
	//读取失败
	if err != nil {
		return err
	}

	//获取记录所对应的结构体
	typeRecord := rf.typeRecord
	//创建记录切片（记录文件的第一行是中文说明字段，不用保存）
	records := make([]interface{}, len(lines)-1)
	//创建索引切片
	indexes := []Index{}

	//根据记录所对应的结构体，预先创建相应位置的索引到索引切片
	for i := 0; i < typeRecord.NumField(); i++ {
		tag := typeRecord.Field(i).Tag
		if tag == "index" {
			indexes = append(indexes, make(Index))
		}
	}

	//将读取的所有记录，保存到所对应的结构体中
	for n := 1; n < len(lines); n++ {
		//创建一个记录所对应的结构体
		value := reflect.New(typeRecord)
		//保存到records中
		records[n-1] = value.Interface()
		//获取所创建结构体的结构，用来实际保存记录
		record := value.Elem()
		//获取记录
		line := lines[n]
		//记录的字段数和所创建结构体的字段数不匹配
		if len(line) != typeRecord.NumField() {
			return fmt.Errorf("line %v, field count mismatch: %v %v", n, len(line), typeRecord.NumField())
		}

		iIndex := 0

		//遍历所有字段，保存字段值
		for i := 0; i < typeRecord.NumField(); i++ {
			//获得记录字段对应的结构字段
			f := typeRecord.Field(i)
			//获得要保存的字段值（字符串）
			strField := line[i]
			//获得实际用来保存记录的字段
			field := record.Field(i)
			//字段不可设置
			if !field.CanSet() {
				continue
			}

			var err error
			//获得字段类型
			kind := f.Type.Kind()
			//将要保存的字段值，转化为对应的类型再保存
			if kind == reflect.Bool { //布尔型
				var v bool
				v, err = strconv.ParseBool(strField)
				if err == nil {
					field.SetBool(v)
				}
			} else if kind == reflect.Int || kind == reflect.Int8 || kind == reflect.Int16 || kind == reflect.Int32 || kind == reflect.Int64 { //有符号整型
				var v int64
				v, err = strconv.ParseInt(strField, 0, f.Type.Bits())
				if err == nil {
					field.SetInt(v)
				}
			} else if kind == reflect.Uint || kind == reflect.Uint8 || kind == reflect.Uint16 || kind == reflect.Uint32 || kind == reflect.Uint64 { //无符号整型
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

			//字段标签是"index"，设置索引
			if f.Tag == "index" {
				//获取当前索引字段在索引切片中所对应的元素
				index := indexes[iIndex]
				iIndex++

				//多条记录之间的索引字段值重复
				if _, ok := index[field.Interface()]; ok {
					return fmt.Errorf("index error: duplicate at (row=%v, col=%v)", n, i)
				}

				//将索引字段值索引到当前记录
				index[field.Interface()] = records[n-1]
			}
		}
	}

	//保存记录切片
	rf.records = records
	//保存索引切片
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

//获取指定位置的索引
func (rf *RecordFile) Indexes(i int) Index {
	if i >= len(rf.indexes) {
		return nil
	}

	return rf.indexes[i]
}

//根据字段值，获取对应的记录
func (rf *RecordFile) Index(i int, inf interface{}) interface{} {
	index := rf.Indexes(i)
	if index == nil {
		return nil
	}

	return index[inf]
}
