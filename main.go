package c3po

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

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

func parseTags(tag string) map[string]string {
	kvTags := map[string]string{}

	pairs := strings.Split(tag, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			kvTags[kv[0]] = ""
		} else {
			kvTags[kv[0]] = strings.ToLower(kv[1])
		}
	}
	return kvTags
}

func parseSchema(schema any, tagKey string, tags map[string]string) *Fielder {

	rv := reflect.ValueOf(schema)
	rt := reflect.TypeOf(schema)
	f := &Fielder{}

	if rt.Kind() == reflect.Ptr {
		f.IsPointer = true
		rt = rt.Elem()
		rv = rv.Elem()
	}

	f.Type = rt.Kind()
	f.Tags = tags
	f.Schema = schema
	f.Children = map[string]*Fielder{}

	if _, ok := tags["-"]; ok {
		return nil
	}

	v, ok := tags["escape"]
	f.Escape = (ok && (v == "" || v == "true"))

	f.RealName = tags["realName"]

	v, ok = tags["required"]
	f.Required = !(ok && (v == "false"))

	if v, ok := tags["name"]; ok && v != "" {
		f.Name = v
	} else {
		f.Name = strings.ToLower(f.RealName)
	}
	if rv.Kind() == reflect.Pointer {
		f.IsPointer = true
		rTmp := rv.Elem()
		if rTmp.IsValid() {
			rv = rTmp
			rt = rt.Elem()
		}
	}
	if !rv.IsValid() {
		rv = reflect.New(rt).Elem()
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
	}

	switch rt.Kind() {
	case reflect.Struct:
		f.FieldsByIndex = map[int]string{}
		for i := 0; i < rt.NumField(); i++ {
			fv := rv.Field(i)
			if fv.CanInterface() {
				ft := rt.Field(i)
				childTags := parseTags(ft.Tag.Get(tagKey))
				if _, ok := childTags["-"]; ok {
					continue
				}
				childTags["realName"] = strings.ToLower(ft.Name)
				if v, ok := childTags["name"]; ok && v != "" {
					childTags["name"] = v
				} else {
					childTags["name"] = strings.ToLower(ft.Name)
				}
				child := parseSchema(fv.Interface(), tagKey, childTags)
				f.FieldsByIndex[i] = ft.Name
				if child != nil {
					f.Children[ft.Name] = child
				}
			}
		}
	case reflect.Slice, reflect.Array:
		f.Type = reflect.Slice
		f.IsSlice = true

		sliceObjet := reflect.New(rv.Type().Elem()).Elem()
		f.SliceType = parseSchema(sliceObjet.Interface(), tagKey, map[string]string{"realName": ""})
	}

	if f.IsSlice {
		v, ok = tags["strict"]
		f.SliceStrict = !(ok && (v == "false"))
	}

	return f
}

func SetReflectValue(r reflect.Value, v *reflect.Value, escape bool) bool {
	if v.IsValid() {
		c := convert(v, r.Type(), escape)
		if c {
			r.Set(*v)
			return true
		}
	}
	return false
}

func ParseSchemaWithTag(tagKey string, schema any) *Fielder {
	tags := map[string]string{
		"realName": reflect.TypeOf(schema).Name(),
	}
	return parseSchema(schema, tagKey, tags)
}

func ParseSchema(schema any) *Fielder {
	return ParseSchemaWithTag("c3po", schema)
}

type Fielder struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	RealName string `json:"-"`

	IsPointer   bool     `json:"-"`
	IsSlice     bool     `json:"-"`
	SliceStrict bool     `json:"-"`
	SliceType   *Fielder `json:"-"`

	Type   reflect.Kind      `json:"type"`
	Tags   map[string]string `json:"-"`
	Schema any               `json:"-"`

	Children      map[string]*Fielder `json:"-"`
	FieldsByIndex map[int]string      `json:"-"`
	Escape        bool                `json:"-"`
}

func (f *Fielder) GetFieldsName() []string {
	if len(f.Children) == 0 {
		return []string{}
	}
	fields := []string{}
	for _, field := range f.FieldsByIndex {
		fields = append(fields, field)
	}
	return fields
}

func (f *Fielder) ToMap() map[string]any {
	data := map[string]any{}
	if f.Name != "" {
		data["name"] = f.Name
	} else {
		data["name"] = f.RealName
	}

	if len(f.Children) > 0 {
		dfs := map[string]any{}
		for _, ff := range f.Children {
			dfs[ff.RealName] = ff.ToMap()
		}
		data["fields"] = dfs
	}
	return data
}

func (f *Fielder) MountSchema(v any) (reflect.Value, any) {
	if v == nil {
		if f.Required {
			errs := map[string]any{}
			if len(f.Children) > 0 {
				for _, c := range f.Children {
					if c.Required {
						errs[c.Name] = RetMissing(c)
					}
				}
				return reflect.Value{}, errs
			} else {
				return reflect.Value{}, map[string]any{
					f.Name: RetMissing(f),
				}
			}
		}
		return reflect.Value{}, nil
	}

	var errs any
	var sch reflect.Value

	switch f.Type {
	default:
		_sch := reflect.TypeOf(f.Schema)
		if _sch.Kind() == reflect.Ptr {
			_sch = _sch.Elem()
		}
		sch = reflect.New(_sch).Elem()
		schV := reflect.ValueOf(v)

		if f.IsPointer {
			if sch.Kind() != reflect.Ptr && sch.CanAddr() {
				sch = sch.Addr()
			}
		} else if sch.Kind() == reflect.Ptr {
			sch = sch.Elem()
		}

		if !SetReflectValue(sch, &schV, f.Escape) {
			return reflect.Value{}, RetInvalidType(f)
		}
	case reflect.Array, reflect.Slice:
		schVal := reflect.ValueOf(v)

		if k := schVal.Kind(); k != reflect.Slice {
			errs = RetInvalidType(f)
			break
		}

		sliceOf := reflect.TypeOf(f.SliceType.Schema)
		lenSlice := schVal.Len()
		sch = reflect.MakeSlice(reflect.SliceOf(sliceOf), lenSlice, lenSlice)
		_errs := []any{}
		for i := 0; i < lenSlice; i++ {
			s := schVal.Index(i)
			sf := f.SliceType

			slicSch, err := sf.MountSchema(s.Interface().(map[string]any))
			if err != nil {
				_errs = append(_errs, err)
				if f.SliceStrict {
					break
				}
			}
			sItem := sch.Index(i)
			if !sItem.IsValid() {
				continue
			}

			if f.SliceType.IsPointer {
				if slicSch.Kind() != reflect.Ptr && slicSch.CanAddr() {
					slicSch = slicSch.Addr()
				}
			} else {
				if slicSch.Kind() == reflect.Ptr {
					slicSch = slicSch.Elem()
				}
			}
			sItem.Set(slicSch)
		}
		if sch.Len() == 0 {
			if f.Required {
				_errs = append(_errs, RetMissing(f))
			}
		}
		if len(_errs) > 0 {
			if len(_errs) == 1 {
				errs = _errs[0]
			} else {
				errs = _errs
			}
		}
	case reflect.Struct:
		_errs := []any{}
		data, ok := v.(map[string]any)
		if !ok {
			for _, fielder := range f.Children {
				if fielder.Required {
					_errs = append(_errs, RetMissing(fielder))
				}
			}
			return reflect.Value{}, _errs
		}
		_sch := reflect.TypeOf(f.Schema)
		if _sch.Kind() == reflect.Ptr {
			_sch = _sch.Elem()
		}
		sch = reflect.New(_sch).Elem()

		for i := 0; i < sch.NumField(); i++ {
			fieldName, ok := f.FieldsByIndex[i]
			if ok {
				fielder := f.Children[fieldName]

				schF := sch.FieldByName(fieldName)
				value, ok := data[fielder.Name]
				if !ok {
					value, ok = data[fielder.RealName]
				}
				if !ok || value == nil {
					if fielder.Required {
						_errs = append(_errs, map[string]any{fielder.Name: RetMissing(fielder)})
					}
					continue
				}
				rv, __errs := fielder.MountSchema(value)
				if __errs != nil {
					_errs = append(_errs, __errs)
					continue
				}

				if !SetReflectValue(schF, &rv, false) {
					_errs = append(_errs, map[string]any{fielder.Name: RetInvalidType(fielder)})
					continue
				}
			}
		}
		if len(_errs) > 0 {
			if len(_errs) == 1 {
				errs = _errs[0]
			} else {
				errs = _errs
			}
		}
	}

	if f.IsPointer {
		if sch.Kind() != reflect.Ptr && sch.CanAddr() {
			sch = sch.Addr()
		}
	} else {
		if sch.Kind() == reflect.Ptr {
			sch = sch.Elem()
		}
	}
	if errs != nil {
		if f.Name != "" {
			return sch, map[string]any{f.Name: errs}
		}
		if slcErr, ok := errs.([]any); ok && len(slcErr) == 1 {
			return sch, slcErr[0]
		} else {
			if mapErrs, ok := errs.(map[string]any); ok && len(mapErrs) == 1 {
				for _, err := range mapErrs {
					return sch, err
				}
			}
			return sch, errs
		}

	}
	return sch, nil
}

func (f *Fielder) Mount(data any) (any, error) {
	sch, err := f.MountSchema(data)
	if err != nil {
		e, _ := json.MarshalIndent(err, "", "    ")
		return nil, errors.New(string(e))
	}
	if f.IsPointer {
		if sch.Kind() != reflect.Pointer {
			if sch.CanAddr() {
				return sch.Addr().Interface(), nil
			}
		}
	} else {
		if sch.Kind() == reflect.Pointer {
			return sch.Elem().Interface(), nil
		}
	}
	return sch.Interface(), nil
}

func (f *Fielder) String() string {
	s, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return ""
	}
	return string(s)
}
