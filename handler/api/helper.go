package api

import (
	"reflect"
)

// 结构体复制
func structAssign(dst interface{}, src interface{}) interface{} {
	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src).Elem()
	sTypeOfT := srcVal.Type()
	for i := 0; i < srcVal.NumField(); i++ {
		name := sTypeOfT.Field(i).Name
		if ok := dstVal.FieldByName(name).IsValid(); ok {
			dstVal.FieldByName(name).Set(srcVal.FieldByName(name))
		}
	}
	return dst
}
