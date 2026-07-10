package server

import (
	"reflect"
	"strings"
)

func getConfigField(cfg *ReaderConfig, key string) (any, bool) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		name := strings.Split(jsonTag, ",")[0]
		if name == key {
			return v.Field(i).Interface(), true
		}
	}
	return nil, false
}

func setConfigField(cfg *ReaderConfig, key string, value any) bool {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		name := strings.Split(jsonTag, ",")[0]
		if name != key {
			continue
		}
		fv := v.Field(i)
		if !fv.CanSet() {
			return false
		}
		val := reflect.ValueOf(value)
		if val.Type().ConvertibleTo(fv.Type()) {
			fv.Set(val.Convert(fv.Type()))
			return true
		}
		return false
	}
	return false
}
