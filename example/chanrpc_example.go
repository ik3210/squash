package main

import (
	"fmt"
	"squash/chanrpc"
	"sync"
)

func main() {
	//一个rpc服务器可以对应多个rpc客户端
	//rpc客户端将调用信息传输到rpc服务器的调用信息管道
	//rpc服务器再将响应消息返回到rpc客户端各自的返回信息管道中

	//创建一个rpc服务器，调用管道的长度为10
	s := chanrpc.NewServer(10)
	//声明等待组
	var wg sync.WaitGroup
	//等待组+1
	wg.Add(1)

	//goroutine 1
	go func() {
		//注册函数
		s.Register("f0", func(args []interface{}) {})
		s.Register("f1", func(args []interface{}) interface{} {
			return 1
		})
		s.Register("fn", func(args []interface{}) []interface{} {
			return []interface{}{1, 2, 3}
		})
		s.Register("add", func(args []interface{}) interface{} {
			n1 := args[0].(int)
			n2 := args[1].(int)
			return n1 + n2
		})

		//注册完成，等待组-1
		wg.Done()

		//死循环，执行rpc调用
		for {
			//从调用信息管道里读取一个调用信息并执行
			err := s.Exec(<-s.ChanCall)

			//执行失败
			if err != nil {
				fmt.Println(err)
			}
		}
	}()

	//等待goroutine 1完成，阻塞在这里
	wg.Wait()

	//等待组+1
	wg.Add(1)

	//goroutine 2
	go func() {
		//打开一个rpc客户端
		c := s.Open(10)

		//同步调用0，无返回值
		err := c.Call0("f0")

		if err != nil {
			fmt.Println(err)
		}

		//同步调用1，1个返回值
		r1, err := c.Call1("f1")

		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(r1)
		}

		//同步调用N，N个返回值
		rn, err := c.CallN("fn")

		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(rn[0], rn[1], rn[2])
		}

		//同步调用1，1个返回值
		ra, err := c.Call1("add", 1, 2)

		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(ra)
		}

		//异步调用
		c.AsynCall("f0", func(err error) {
			if err != nil {
				fmt.Println(err)
			}
		})

		c.AsynCall("f1", func(ret interface{}, err error) {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(ret)
			}
		})

		c.AsynCall("fn", func(ret []interface{}, err error) {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(ret[0], ret[1], ret[2])
			}
		})

		c.AsynCall("add", 1, 2, func(ret interface{}, err error) {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(ret)
			}
		})

		//执行异步调用的回调
		c.Cb(<-c.ChanAsynRet)
		c.Cb(<-c.ChanAsynRet)
		c.Cb(<-c.ChanAsynRet)
		c.Cb(<-c.ChanAsynRet)

		//rpc服务器自己调用自己
		s.Go("f0")

		//goroutine 2完成
		wg.Done()
	}()

	//等待goroutine 2完成
	wg.Wait()

	// Output:
	// 1
	// 1 2 3
	// 3
	// 1
	// 1 2 3
	// 3
}
