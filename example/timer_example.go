package main

import (
	"fmt"
	"squash/timer"
	"time"
)

func ExampleTimer() {
	//创建分发器
	d := timer.NewDispatcher(10)
	//注册定时器1
	d.AfterFunc(1, func() {
		fmt.Println("My name is Leaf")
	})
	//注册定时器2
	t := d.AfterFunc(1, func() {
		fmt.Println("will not print")
	})
	//停止定时器2
	t.Stop()

	//分发
	(<-d.ChanTimer).Cb()

	// Output:
	// My name is Leaf
}

func ExampleCronExpr() {
	//创建cron表达式
	cronExpr, err := timer.NewCronExpr("0 * * * *")
	//创建失败
	if err != nil {
		return
	}

	//打印下一个时间
	fmt.Println(cronExpr.Next(time.Date(
		2000, 1, 1,
		20, 10, 5,
		0, time.UTC,
	)))

	// Output:
	// 2000-01-01 21:00:00 +0000 UTC
}

func ExampleCron() {
	//创建分发器
	d := timer.NewDispatcher(10)
	//注册计划任务
	var c *timer.Cron

	c, _ = d.CronFunc("* * * * * *", func() {
		fmt.Println("My name is Leaf")
		c.Stop()
	})

	//分发
	(<-d.ChanTimer).Cb()

	// Output:
	// My name is Leaf
}

func main() {
	ExampleTimer()
	ExampleCronExpr()
	ExampleCron()
}
