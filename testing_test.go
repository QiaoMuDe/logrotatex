package logrotatex

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// assert 函数用于在条件为 false 时记录给定的消息。
func assert(condition bool, t testing.TB, msg string, v ...interface{}) {
	// 调用 assertUp 函数，将调用栈层级设为 1
	assertUp(condition, t, 1, msg, v...)
}

// assertUp 函数与 assert 类似，但用于辅助函数内部，确保失败报告的文件和行号对应调用栈中更高层级。
func assertUp(condition bool, t testing.TB, caller int, msg string, v ...interface{}) {
	if !condition {
		// 获取调用者的文件路径、行号等信息
		_, file, line, _ := runtime.Caller(caller + 1)
		// 将文件基名和行号添加到参数列表中
		v = append([]interface{}{filepath.Base(file), line}, v...)
		// 打印错误信息
		fmt.Printf("%s:%d: "+msg+"\n", v...)
		// 标记测试失败并立即终止当前测试
		t.FailNow()
	}
}

// equals 函数根据 reflect.DeepEqual 测试两个值是否相等。
func equals(exp, act interface{}, t testing.TB) {
	// 调用 equalsUp 函数，将调用栈层级设为 1
	equalsUp(exp, act, t, 1)
}

// equalsUp 函数与 equals 类似，但用于辅助函数内部，确保失败报告的文件和行号对应调用栈中更高层级。
func equalsUp(exp, act interface{}, t testing.TB, caller int) {
	if !reflect.DeepEqual(exp, act) {
		// 获取调用者的文件路径、行号等信息
		_, file, line, _ := runtime.Caller(caller + 1)
		// 打印预期值和实际值的信息
		fmt.Printf("%s:%d: exp: %v (%T), got: %v (%T)\n",
			filepath.Base(file), line, exp, exp, act, act)
		// 标记测试失败并立即终止当前测试
		t.FailNow()
	}
}

// isNil 函数在给定值不为 nil 时报告失败。注意，不能为 nil 的值总是会使此检查失败。
func isNil(obtained interface{}, t testing.TB) {
	// 调用 isNilUp 函数，将调用栈层级设为 1
	isNilUp(obtained, t, 1)
}

// isNilUp 函数与 isNil 类似，但用于辅助函数内部，确保失败报告的文件和行号对应调用栈中更高层级。
func isNilUp(obtained interface{}, t testing.TB, caller int) {
	if !_isNil(obtained) {
		// 获取调用者的文件路径、行号等信息
		_, file, line, _ := runtime.Caller(caller + 1)
		// 打印期望为 nil 但实际得到的值的信息
		fmt.Printf("%s:%d: expected nil, got: %v\n", filepath.Base(file), line, obtained)
		// 标记测试失败并立即终止当前测试
		t.FailNow()
	}
}

// notNil 函数在给定值为 nil 时报告失败。
func notNil(obtained interface{}, t testing.TB) {
	// 调用 notNilUp 函数，将调用栈层级设为 1
	notNilUp(obtained, t, 1)
}

// notNilUp 函数与 notNil 类似，但用于辅助函数内部，确保失败报告的文件和行号对应调用栈中更高层级。
func notNilUp(obtained interface{}, t testing.TB, caller int) {
	if _isNil(obtained) {
		// 获取调用者的文件路径、行号等信息
		_, file, line, _ := runtime.Caller(caller + 1)
		// 打印期望为非 nil 但实际得到的值的信息
		fmt.Printf("%s:%d: expected non-nil, got: %v\n", filepath.Base(file), line, obtained)
		// 标记测试失败并立即终止当前测试
		t.FailNow()
	}
}

// _isNil 是 isNil 和 notNil 函数的辅助函数，不应直接使用。
func _isNil(obtained interface{}) bool {
	// 首先检查值是否直接为 nil
	if obtained == nil {
		return true
	}

	// 根据反射获取值的类型并检查是否为 nil
	switch v := reflect.ValueOf(obtained); v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}

	return false
}
