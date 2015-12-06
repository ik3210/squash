package main

import (
	"squash/log"
)

func main() {
	name := "squash"

	log.Debug("My name is %v", name)
	log.Release("My name is %v", name)
	log.Error("My name is %v", name)
	//log.Fatal("My name is %v", name)

	// 日志级别设为realease，低于realease的日志不会输出
	logger, err := log.New("release", "")

	if err != nil {
		return
	}

	defer logger.Close()

	logger.Debug("will not print")
	logger.Release("My name is %v", name)

	log.Export(logger)

	log.Debug("will not print")
	log.Release("My name is %v", name)
}
