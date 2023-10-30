package c3po

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

var htmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&#34;",
	"'", "&#39;",
)

func HtmlEscape(s string) string { return htmlReplacer.Replace(s) }

func try() bool {
	err := recover()
	return err != nil
}

func convert(v *reflect.Value, t reflect.Type, stringEscape bool) bool {
	defer try()
	if v.Kind() == t.Kind() {
		return true
	}

	switch t.Kind() {
	case reflect.Float32, reflect.Float64:
		switch v.Kind() {
		case reflect.String:
			i, err := strconv.ParseFloat(v.Interface().(string), 64)
			if err != nil {
				return false
			}
			*v = reflect.ValueOf(i).Convert(t)
		case reflect.Int, reflect.Int64:
			*v = v.Convert(t)
		case reflect.Float32, reflect.Float64:
			*v = v.Convert(t)
		}
	case reflect.Int, reflect.Int64:
		switch v.Kind() {
		case reflect.String:
			val, err := strconv.ParseFloat(v.Interface().(string), 64)
			if err != nil {
				return false
			}
			*v = reflect.ValueOf(val).Convert(t)
		case reflect.Float32, reflect.Float64:
			*v = v.Convert(t)
		default:
			return false
		}
	case reflect.Bool:
		if v.Kind() != reflect.String {
			return false
		}

		b := strings.ToLower(v.Interface().(string))
		if b == "true" {
			*v = reflect.ValueOf(true)
		} else if b == "false" {
			*v = reflect.ValueOf(false)
		} else {
			return false
		}

	case reflect.String:
		nv := fmt.Sprint(v.Interface())
		if stringEscape {
			nv = htmlReplacer.Replace(nv)
		}
		*v = reflect.ValueOf(nv)

	}
	return true
}

func SetReflectValue(r reflect.Value, v reflect.Value, escape bool) bool {
	if v.IsValid() {
		c := convert(&v, r.Type(), escape)
		if c {
			if r.Kind() == reflect.Pointer && v.Kind() != reflect.Pointer {
				v = v.Addr()
			} else if r.Kind() != reflect.Pointer && v.Kind() == reflect.Pointer {
				v = v.Elem()
			}
			r.Set(v)
			return true
		}
	}
	return false
}

func parseTags(tag string) map[string]string {
	kvTags := map[string]string{}

	pairs := strings.Split(tag, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		kv := strings.Split(pair, "=")
		key := strings.ToLower(kv[0])
		if len(kv) == 1 {
			kvTags[key] = ""
		} else {
			kvTags[key] = kv[1]
		}
	}
	return kvTags
}

func RetInvalidType(f *Fielder) error {
	s := fmt.Sprintf(`{"field":"%s", "type": "%s","message": "invalid type","required": %v}`, f.Name, f.Type.String(), f.Required)
	return errors.New(s)
}

func RetMissing(f *Fielder) error {
	s := fmt.Sprintf(`{"field":"%s", "type": "%s","message": "missing","required": %v}`, f.Name, f.Type.String(), f.Required)
	return errors.New(s)
}

func GetFunctionName(i interface{}) string {
	return runtime.FuncForPC(
		reflect.ValueOf(i).Pointer(),
	).Name()
}
