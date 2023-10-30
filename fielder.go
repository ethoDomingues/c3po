package c3po

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
)

func parseSchema(schema any, tagKey string, tags map[string]string) *Fielder {
	if schema == nil {
		return nil
	}
	rv := reflect.ValueOf(schema)
	f := &Fielder{}

	if rv.Kind() == reflect.Ptr {
		f.IsPointer = true
	}

	rt := rv.Type()
	rv = GetReflectElem(rv)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	f.Tags = tags
	f.Type = rt.Kind()
	f.Children = map[string]*Fielder{}
	f.Schema = schema

	if _, ok := tags["-"]; ok {
		return nil
	}

	v, ok := tags["escape"]
	f.Escape = (ok && (v == "" || strings.ToLower(v) == "true"))

	f.RealName = tags["realName"]

	if f.RealName == "" {
		f.RealName = rt.Name()
	}

	v, ok = tags["required"]
	f.Required = ok && (strings.ToLower(v) == "true")

	v, ok = tags["skiponerr"]
	f.SkipOnErr = ok && (strings.ToLower(v) == "true") && !f.Required

	if v, ok := tags["name"]; ok && v != "" {
		f.Name = v
	} else {
		f.Name = f.RealName
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
				cname := ""
				childTags["realName"] = ft.Name
				if v, ok := childTags["name"]; ok && v != "" {
					cname = v
					childTags["name"] = v
				} else {
					cname = strings.ToLower(ft.Name)
					childTags["name"] = cname
				}
				child := parseSchema(fv.Interface(), tagKey, childTags)
				f.FieldsByIndex[i] = cname
				if child != nil {
					child.Index = i
					f.Children[cname] = child
					if v, ok := childTags["join"]; ok && strings.ToLower(v) == "true" {
						if f.JoinsFielders == nil {
							f.JoinsFielders = map[string]*Fielder{}
						}
						f.JoinsFielders[cname] = child
						for k, v := range child.Children {
							v.SuperIndex = i
							v.SuperName = ft.Name
							f.Children[k] = v
						}
					}
				}
			}
		}

	case reflect.Slice, reflect.Array:
		f.Type = reflect.Slice
		f.IsSlice = true

		sliceObjet := reflect.New(rv.Type().Elem()).Elem()
		f.SliceType = parseSchema(sliceObjet.Interface(), tagKey, map[string]string{"realName": ""})
	case reflect.Map:
		mapKey := reflect.New(rt.Key()).Elem()
		mapValue := reflect.New(rt.Elem()).Elem()
		f.Type = rt.Kind()
		f.MapKeyType = parseSchema(mapKey.Interface(), tagKey, map[string]string{"realName": ""})
		f.MapValueType = parseSchema(mapValue.Interface(), tagKey, map[string]string{"realName": ""})
	}

	if f.IsSlice {
		v, ok = tags["strict"]
		f.SliceStrict = !(ok && (v == "false"))
	}
	if f.IsMAP {
		v, ok = tags["strict"]
		f.SliceStrict = !(ok && (v == "false"))
	}
	if rv.CanInterface() && !rv.IsZero() {
		def, err := encode(rv.Interface())
		if err != nil {
			n := f.Name
			if n == "" {
				n = f.RealName
			}
			log.Println(err)
			log.Printf("warn: Default Value is invalid into fielder: '%s'\n", n)
		} else {
			f.Default = def
		}
	}
	return f
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
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	RealName  string `json:"-"`
	Escape    bool   `json:"-"`
	SkipOnErr bool

	IsPointer bool `json:"-"`
	Default   any

	IsSlice     bool     `json:"-"`
	SliceStrict bool     `json:"-"`
	SliceType   *Fielder `json:"-"`

	Join          bool                `json:"-"`
	JoinsFielders map[string]*Fielder `json:"-"`
	SuperIndex    int
	SuperName     string

	IsMAP        bool
	MapKeyType   *Fielder
	MapValueType *Fielder

	Type   reflect.Kind      `json:"type"`
	Tags   map[string]string `json:"-"`
	Index  int
	Schema any `json:"-"`

	Children      map[string]*Fielder `json:"-"`
	FieldsByIndex map[int]string      `json:"-"`
}

func (f *Fielder) decodeSchema(v any) (reflect.Value, any) {
	if v == nil {
		if f.Default != nil {
			return f.decodeSchema(f.Default)
		} else if f.Required {
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
		} else {
			return reflect.Value{}, nil
		}
	}

	var errs any
	var sch reflect.Value
	var rfVal = reflect.ValueOf(v)

	if rfVal.Kind() == reflect.Pointer {
		rfVal = rfVal.Elem()
	}
	switch f.Type {
	default:
		sch = GetReflectElem(f.New())
		if !SetReflectValue(sch, rfVal, f.Escape) {
			if !f.SkipOnErr {
				return reflect.Value{}, RetInvalidType(f)
			}
			return sch, nil
		}
	case reflect.Array, reflect.Slice:
		if rfVal.Kind() != reflect.Slice {
			errs = RetInvalidType(f)
			break
		}

		sliceOf := reflect.TypeOf(f.SliceType.Schema)
		lenSlice := rfVal.Len()
		sch = reflect.MakeSlice(reflect.SliceOf(sliceOf), lenSlice, lenSlice)
		_errs := []any{}
		for i := 0; i < lenSlice; i++ {
			s := rfVal.Index(i)
			sf := f.SliceType

			slicSch, err := sf.decodeSchema(s.Interface())
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
			_data, err := Encode(v)
			if err != nil {
				return reflect.Value{}, err
			}
			data, ok = _data.(map[string]any)
			if !ok {
				if !f.SkipOnErr {
					return reflect.Value{}, RetInvalidType(f)
				}
				return f.New(), nil
			}
		}

		sch = f.New().Elem()
		for i := 0; i < sch.NumField(); i++ {
			schF := sch.Field(i)
			if !schF.CanInterface() {
				continue
			}
			var value any
			fieldJoin := false
			fieldName := f.FieldsByIndex[i]
			fielder, ok := f.JoinsFielders[fieldName]
			if ok {
				fieldJoin = true
				_val, err := fielder.decodeSchema(data)
				if err != nil {
					_errs = append(_errs, err)
				}
				value = _val
			} else {
				fielder, ok = f.Children[fieldName]
				if !ok {
					continue
				}
			}
			if !fieldJoin {
				value, ok = data[fielder.Name]
				if !ok {
					value, ok = data[fielder.RealName]
					if !ok {
						if fielder.Default == nil {
							if fielder.Required {
								_errs = append(_errs, map[string]any{fielder.Name: RetMissing(fielder)})
							}
							continue
						}
						value = fielder.Default
					}
				}
				if value == nil {
					if fielder.Default == nil {
						if fielder.Required {
							_errs = append(_errs, map[string]any{fielder.Name: RetMissing(fielder)})
						}
						continue
					}
					value = fielder.Default
				}
			}
			rv, __errs := fielder.decodeSchema(value)
			if __errs != nil {
				_errs = append(_errs, __errs)
				continue
			}
			if !SetReflectValue(schF, rv, false) {
				if !fielder.SkipOnErr {
					_errs = append(_errs, map[string]any{fielder.Name: RetInvalidType(fielder)})
				}
				continue
			}

		}
		if len(_errs) > 0 {
			if len(_errs) == 1 {
				errs = _errs[0]
			} else {
				errs = _errs
			}
		}
	case reflect.Map:
		if rfVal.Kind() == reflect.Pointer {
			rfVal = rfVal.Elem()
		}
		if rfVal.Kind() != reflect.Map || !rfVal.IsValid() {
			errs = map[string]any{f.Name: RetInvalidType(f)}
			break
		}
		mt := reflect.TypeOf(f.Schema)
		m := reflect.MakeMap(mt)
		for _, key := range rfVal.MapKeys() {
			mindex := rfVal.MapIndex(key)

			mkey, err := f.MapKeyType.decodeSchema(key.Interface())
			if err != nil {
				errs = err
				break
			}
			mval, err := f.MapValueType.decodeSchema(mindex.Interface())
			if err != nil {
				errs = err
				break
			}
			m.SetMapIndex(mkey, mval)
		}
		sch = m
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

func (f *Fielder) CheckSchPtr(r reflect.Value) any {
	if f.IsPointer && r.Kind() != reflect.Pointer {
		return r.Addr().Interface()
	} else if !f.IsPointer && r.Kind() == reflect.Pointer {
		return r.Elem().Interface()
	}
	return r.Interface()
}

func (f *Fielder) Decode(data any) (any, error) {
	sch, err := f.decodeSchema(data)
	if err != nil {
		if e, ok := err.(error); ok {
			return nil, e
		}
		if s, ok := err.(string); ok {
			return nil, errors.New(s)
		}
		return nil, errors.New(fmt.Sprint(err))
	}
	if !sch.IsValid() {
		return nil, errors.New("invalid reflect")
	}
	return f.CheckSchPtr(sch), nil
}

func (f *Fielder) New() reflect.Value {
	rs := reflect.ValueOf(f.Schema)
	if f.IsSlice {
		t := reflect.TypeOf(f.SliceType.Schema)
		t = reflect.SliceOf(t)
		rs = reflect.MakeSlice(t, 0, 0)
	}
	t := GetReflectTypeElem(rs.Type())
	v := reflect.New(t)
	return v
}

func GetReflectElem(r reflect.Value) reflect.Value {
	for c := 0; c < 10; c++ {
		if r.Kind() != reflect.Ptr {
			break
		}
		r = r.Elem()
	}
	return r
}

func GetReflectTypeElem(t reflect.Type) reflect.Type {
	for c := 0; c < 10; c++ {
		if t.Kind() != reflect.Ptr {
			break
		}
		t = t.Elem()
	}
	return t
}
