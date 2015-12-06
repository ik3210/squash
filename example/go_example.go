package main

import (
	"fmt"
	"squash/go"
	"time"
)

func Example() {
	//创建一个Go，回调管道长度为10
	d := g.New(10)
	//接收结果
	var res int

	//Go 1
	d.Go(func() {
		fmt.Println("1 + 1 = ?")
		//在一个新的goroutine中执行运算
		res = 1 + 1
	}, func() {
		//打印结果
		fmt.Println(res)
	})

	//读出回调执行，即上方的打印结果函数
	d.Cb(<-d.ChanCb)

	//Go 2
	d.Go(func() {
		fmt.Print("My name is ")
	}, func() {
		fmt.Println("squash")
	})

	//没有显式调用d.Cb(<-d.ChanCb)，但d.Close()的时候会执行完全部回调函数
	d.Close()

	// Output:
	// 1 + 1 = ?
	// 2
	// My name is squash
}

func ExampleLinearContext() {
	//创建一个Go，回调管道长度为10
	d := g.New(10)

	//并发
	d.Go(func() {
		//因为有延时操作，所以先打印2后打印1
		time.Sleep(time.Second / 2)
		fmt.Println("1")
	}, nil)

	d.Go(func() {
		fmt.Println("2")
	}, nil)

	//读出的是nil，不执行回调
	d.Cb(<-d.ChanCb)
	//读出的是nil，不执行回调
	d.Cb(<-d.ChanCb)

	//创建一个线性上下文
	c := d.NewLinearContext()

	//串行
	c.Go(func() {
		//因为是线性的，所以即使有延时操作，也会按顺序执行
		time.Sleep(time.Second / 2)
		fmt.Println("1")
	}, nil)

	c.Go(func() {
		fmt.Println("2")
	}, nil)

	//没有显式调用d.Cb(<-d.ChanCb)，但d.Close()的时候会执行完全部回调函数
	d.Close()

	// Output:
	// 2
	// 1
	// 1
	// 2
}

func main() {
	Example()
	ExampleLinearContext()
}
