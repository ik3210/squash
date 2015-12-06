package main

import (
	"fmt"
	"squash/recordfile"
)

func main() {
	type Record struct {
		IndexInt int       "index" //数字索引
		IndexStr string    "index" //字符串索引
		_Number  int32     //数字类型
		Str      string    //字符串类型
		Arr1     [2]int    //数组类型
		Arr2     [3][2]int //嵌套数组
		Arr3     []int     //变长数组
		St       struct {  //结构体类型
			Name string "name"
			Num  int    "num"
		}
	}

	//创建一个记录文件
	rf, err := recordfile.New(Record{})

	//创建失败，返回
	if err != nil {
		fmt.Println(err)
		return
	}

	//读取记录文件
	err = rf.Read("recordfile_example.txt")

	//读取失败，返回
	if err != nil {
		fmt.Println(err)
		return
	}

	//遍历记录文件，输出所有记录的数字索引
	for i := 0; i < rf.NumRecord(); i++ {
		r := rf.Record(i).(*Record)
		fmt.Println(r.IndexInt)
	}

	//输出数字索引为2的记录的字符串索引
	r := rf.Index(2).(*Record)
	fmt.Println(r.Str)

	//同上
	r = rf.Indexes(0)[2].(*Record)
	fmt.Println(r.Str)

	//输出字符串索引为three的记录的字段
	r = rf.Indexes(1)["three"].(*Record)
	fmt.Println(r.Str)
	fmt.Println(r.Arr1[1])
	fmt.Println(r.Arr2[2][0])
	fmt.Println(r.Arr3[0])
	fmt.Println(r.St.Name)

	// Output:
	// 1
	// 2
	// 3
	// cat
	// cat
	// book
	// 6
	// 4
	// 6
	// GreatFeng
}
