package timer

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Field name   | Mandatory? | Allowed values | Allowed special characters
// ----------   | ---------- | -------------- | --------------------------
// Seconds      | No         | 0-59           | * / , -
// Minutes      | Yes        | 0-59           | * / , -
// Hours        | Yes        | 0-23           | * / , -
// Day of month | Yes        | 1-31           | * / , -
// Month        | Yes        | 1-12           | * / , -
// Day of week  | Yes        | 0-6            | * / , -

//cron表达式
type CronExpr struct {
	sec   uint64
	min   uint64
	hour  uint64
	dom   uint64
	month uint64
	dow   uint64
}

//创建cron表达式
func NewCronExpr(expr string) (cronExpr *CronExpr, err error) {
	//用空格分割表达式
	fields := strings.Fields(expr)

	//数组长度为5或者6（Seconds不是强制设置的）
	if len(fields) != 5 && len(fields) != 6 {
		err = fmt.Errorf("invalid expr %v: expected 5 or 6 fields, got %v", expr, len(fields))
		return
	}

	//未设置Seconds，自己在最前面添加一个0
	if len(fields) == 5 {
		fields = append([]string{"0"}, fields...)
	}

	//创建cron表达式
	cronExpr = new(CronExpr)

	/*解析字段开始*/
	//Seconds
	cronExpr.sec, err = parseCronField(fields[0], 0, 59)
	if err != nil {
		goto onError
	}

	//Minutes
	cronExpr.min, err = parseCronField(fields[1], 0, 59)
	if err != nil {
		goto onError
	}

	//Hours
	cronExpr.hour, err = parseCronField(fields[2], 0, 23)
	if err != nil {
		goto onError
	}

	//Day of month
	cronExpr.dom, err = parseCronField(fields[3], 1, 31)
	if err != nil {
		goto onError
	}

	//Month
	cronExpr.month, err = parseCronField(fields[4], 1, 12)
	if err != nil {
		goto onError
	}

	//Day of week
	cronExpr.dow, err = parseCronField(fields[5], 0, 6)
	if err != nil {
		goto onError
	}
	/*解析字段结束*/

	return

onError:
	err = fmt.Errorf("invalid expr %v: %v", expr, err)
	return
}

//解析cron字段
func parseCronField(field string, min int, max int) (cronField uint64, err error) {
	//用逗号分割字段
	fields := strings.Split(field, ",")

	// 6种形式：
	// 1. *
	// 2. num
	// 3. num-num
	// 4. */num
	// 5. num/num (means num-max/num)
	// 6. num-num/num
	for _, field := range fields {
		//用斜杠分割，获取范围和增幅
		rangeAndIncr := strings.Split(field, "/")
		//分割项数不大于2
		if len(rangeAndIncr) > 2 {
			err = fmt.Errorf("too many slashes: %v", field)
			return
		}

		//用中划线分割范围，获得范围的起始值和结束值
		startAndEnd := strings.Split(rangeAndIncr[0], "-")
		//分割项数不大于2
		if len(startAndEnd) > 2 {
			err = fmt.Errorf("too many hyphens: %v", rangeAndIncr[0])
			return
		}

		//用于存储起始值和结束值
		var start, end int

		if startAndEnd[0] == "*" { //形式1或4
			if len(startAndEnd) != 1 {
				err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
				return
			}

			//起始值等于最小值，结束值等于最大值
			start = min
			end = max
		} else {
			//起始值转化为整数
			start, err = strconv.Atoi(startAndEnd[0])
			//转化失败
			if err != nil {
				err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
				return
			}

			if len(startAndEnd) == 1 { //形式2或5
				if len(rangeAndIncr) == 2 { //有增幅，结束值等于最大值
					end = max
				} else { //没有增幅，结束值等于起始值
					end = start
				}
			} else { //形式3或6
				//结束值转化为整数
				end, err = strconv.Atoi(startAndEnd[1])
				//转化失败
				if err != nil {
					err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
					return
				}
			}
		}

		//起始值不能大于结束值
		if start > end {
			err = fmt.Errorf("invalid range: %v", rangeAndIncr[0])
			return
		}

		//起始值不能小于最小值
		if start < min {
			err = fmt.Errorf("out of range [%v, %v]: %v", min, max, rangeAndIncr[0])
			return
		}

		//结束值不能大于最大值
		if end > max {
			err = fmt.Errorf("out of range [%v, %v]: %v", min, max, rangeAndIncr[0])
			return
		}

		//用于存储增幅
		var incr int

		if len(rangeAndIncr) == 1 { //没有增幅，设置增幅为1
			incr = 1
		} else { //有增幅
			//增幅转化为整数
			incr, err = strconv.Atoi(rangeAndIncr[1])
			//转化失败
			if err != nil {
				err = fmt.Errorf("invalid increment: %v", rangeAndIncr[1])
				return
			}

			//增幅不能小于等于0
			if incr <= 0 {
				err = fmt.Errorf("invalid increment: %v", rangeAndIncr[1])
				return
			}
		}

		if incr == 1 { //没有增幅
			cronField |= ^(math.MaxUint64 << uint(end+1)) & (math.MaxUint64 << uint(start))
			//比如start和end都等于2
			//^(math.MaxUint64 << uint(end+1))等于：
			//0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0111
			//(math.MaxUint64 << uint(start))等于：
			//1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1100
			//&操作后等于：
			//0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0100
			//从左起第2位被标记

			//比如start等于0，end等于6
			//^(math.MaxUint64 << uint(end+1))等于：
			//0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0111 1111
			//(math.MaxUint64 << uint(start))等于：
			//1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111
			//&操作后等于：
			//0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0111 1111
			//从左起第0位到第6位被标记
		} else { //根据增幅计算关键值再移位
			for i := start; i <= end; i += incr {
				cronField |= 1 << uint(i)
			}
		}
	}

	return
}

//匹配day-of-month和day-of-week
func (e *CronExpr) matchDay(t time.Time) bool {
	//day-of-month标志位（1-31）都设置了
	//1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1110
	if e.dom == 0xfffffffe {
		return 1<<uint(t.Weekday())&e.dow != 0
	}

	//day-of-week标志位（0-6）都设置了
	//1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111 1111
	if e.dow == 0x7f {
		return 1<<uint(t.Day())&e.dom != 0
	}

	//不确定哪个能够匹配到
	return 1<<uint(t.Weekday())&e.dow != 0 || 1<<uint(t.Day())&e.dom != 0
}

//计算下一次时间
func (e *CronExpr) Next(t time.Time) time.Time {
	//计算下一秒时间
	t = t.Truncate(time.Second).Add(time.Second)
	//保存当前年份
	year := t.Year()
	//标志是否已初始化（新建一个时间就是初始化）
	initFlag := false

retry:
	//Year
	//跨年，返回零值
	if t.Year() > year+1 {
		return time.Time{}
	}

	//Month
	for 1<<uint(t.Month())&e.month == 0 {
		//没有初始化，标志为已初始化，新建一个时间
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
		}

		//加一个月
		t = t.AddDate(0, 1, 0)
		//已经到了1月（已经遍历了从当前月份到12月份），跳出循环，继续匹配从1月到12月份
		if t.Month() == time.January {
			goto retry
		}
	}

	//Day
	for !e.matchDay(t) {
		if !initFlag {
			initFlag = true
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		}

		t = t.AddDate(0, 0, 1)
		if t.Day() == 1 {
			goto retry
		}
	}

	//Hours
	for 1<<uint(t.Hour())&e.hour == 0 {
		if !initFlag {
			initFlag = true
			t = t.Truncate(time.Hour)
		}

		t = t.Add(time.Hour)
		if t.Hour() == 0 {
			goto retry
		}
	}

	//Minutes
	for 1<<uint(t.Minute())&e.min == 0 {
		if !initFlag {
			initFlag = true
			t = t.Truncate(time.Minute)
		}

		t = t.Add(time.Minute)
		if t.Minute() == 0 {
			goto retry
		}
	}

	//Seconds
	for 1<<uint(t.Second())&e.sec == 0 {
		//程序开头已经截断到秒了
		if !initFlag {
			initFlag = true
		}

		t = t.Add(time.Second)
		if t.Second() == 0 {
			goto retry
		}
	}

	return t
}
