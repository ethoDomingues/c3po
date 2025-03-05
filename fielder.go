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

	f.RealName = tags["realName"]
	if f.RealName == "" {
		f.RealName = rt.Name()
	}

	if v, ok := tags["name"]; ok && v != "" {
		f.Name = v
	} else {
		f.Name = f.RealName
	}

	v, ok := tags["escape"] // default true
	f.Escape = (ok && (strings.ToLower(v) == "true"))

	v, ok = tags["required"] // default false
	f.Required = ok && (strings.ToLower(v) == "true")

	v, ok = tags["recursive"] // default false
	f.Recursive = !ok || strings.ToLower(v) != "false"

	v, ok = tags["nullable"] // default false
	f.Nullable = ok && strings.ToLower(v) == "true" && !f.Required

	v, ok = tags["nonzero"] // default false
	f.NonZero = ok && (strings.ToLower(v) == "true")

	v, ok = tags["skiponerr"] // skip field on err - default false
	f.SkipOnErr = ok && (strings.ToLower(v) == "true") && !f.Required

	if rv.Kind() == reflect.Pointer {
		f.IsPointer = true
		rTmp := rv.Elem()
		if rTmp.Kind() == reflect.Pointer {
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
		f.IsStruct = true
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
					child.SuperIndex = &i
					f.Children[cname] = child
					if v, ok := childTags["heritage"]; ok && strings.ToLower(v) == "true" {
						child.Heritage = true
					}
				}
			}
		}
	case reflect.Slice, reflect.Array:
		f.Recursive = true
		f.Type = rt.Kind()
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

	if rv.CanInterface() && !rv.IsZero() {
		def, err := encode(rv.Interface())
		if err != nil {
			n := f.Name
			if n == "" {
				n = f.RealName
			}
			log.Printf("warn: Default Value is invalid into fielder: '%s'\n", n)
			log.Println(err)
		} else {
			f.Default = def
		}
	}
	return f
}

func ParseSchemaWithTag(tagKey string, schema any) *Fielder {
	tags := map[string]string{}
	if rn := reflect.TypeOf(schema).Name(); rn != "" {
		tags["realName"] = rn
	}
	return parseSchema(schema, tagKey, tags)
}

/*
usage:

	c3po.ParseSchema(struct{}) => struct{}
	c3po.ParseSchema(&struct{}) => *struct{}
	c3po.ParseSchema(&struct{Field:Value}) => *struct{Field: value} // with default value

	type Schema struct{
		Field `c3po:"-"` // omit this field
		Field `c3po:"realName"` // string: real name field
		Field `c3po:"name"` 	// string: name of validation	(default realName)
		Field `c3po:"escape"`	// bool: escape html value		(default false)
		Field `c3po:"required"` // bool:		...			 	(default false)
		Field `c3po:"nullable"` // bool: if true, allow nil value (default false)
		Field `c3po:"recursive"`// bool: deep validation	  	(default true)
		Field `c3po:"skiponerr"`// bool: omit on valid. error 	(default false)
	}
*/
func ParseSchema(schema any) *Fielder {
	return ParseSchemaWithTag("c3po", schema)
}

func reflectOf(v any) reflect.Value {
	var rv = reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv2 := rv.Elem(); rv2.Kind() == reflect.Pointer {
			rv = rv2
		}
	}
	return rv
}

type Fielder struct {
	Name     string
	RealName string

	Default any
	Schema  any

	Escape    bool // default: false
	Required  bool // default: false
	Nullable  bool // default: false
	NonZero   bool // default: false -> only Integers
	Heritage  bool // default: false
	Recursive bool // default: true
	SkipOnErr bool // default: false
	OmitEmpty bool // default: false

	IsMAP,
	IsSlice,
	IsStruct,
	IsPointer bool

	SliceType,
	MapKeyType,
	MapValueType *Fielder

	Type reflect.Kind
	Tags map[string]string

	SuperIndex *int // if a field to a struct

	Children      map[string]*Fielder
	FieldsByIndex map[int]string
}

func (f *Fielder) decodePrimitive(rv reflect.Value) (sch reflect.Value, err any) {
	sch = GetReflectElem(f.New())
	if !SetReflectValue(sch, rv, f.Escape) {
		if !f.SkipOnErr {
			return reflect.Value{}, RetInvalidType(f)
		}
	}
	return
}

func (f *Fielder) decodeSlice(rv reflect.Value) (sch reflect.Value, err any) {
	sliceOf := reflect.TypeOf(f.Schema)
	lenSlice := rv.Len()
	capSlice := rv.Cap()

	sch = reflect.MakeSlice(sliceOf, lenSlice, capSlice)

	errs := []any{}
	for i := 0; i < lenSlice; i++ {
		var (
			s       = GetReflectElem(rv.Index(i))
			sf      = f.SliceType
			err     any
			slicSch reflect.Value
		)

		if f.Recursive {
			slicSch, err = sf.decodeSchema(s.Interface())
		} else {
			if sliceOf == s.Type() {
				slicSch = s
			} else {
				err = RetInvalidType(f.SliceType)
			}
		}

		if err != nil {
			errs = append(errs, err)
			continue
		}
		sIndex := sch.Index(i)
		if f.SliceType.IsPointer {
			if slicSch.Kind() != reflect.Ptr && slicSch.CanAddr() {
				slicSch = slicSch.Addr()
			}
		} else {
			if slicSch.Kind() == reflect.Ptr {
				slicSch = slicSch.Elem()
			}
		}
		sIndex.Set(slicSch)
	}
	if sch.Len() == 0 {
		if f.Required {
			errs = append(errs, RetMissing(f))
		}
	}
	if len(errs) > 0 {
		if len(errs) == 1 {
			err = errs[0]
		} else {
			err = errs
		}
	}
	return
}

func (f *Fielder) decodeMap(rv reflect.Value) (sch reflect.Value, err any) {
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Map || !rv.IsValid() {
		err = map[string]any{f.Name: RetInvalidType(f)}
		return
	}
	mt := reflect.TypeOf(f.Schema)
	m := reflect.MakeMap(mt)
	for _, key := range rv.MapKeys() {
		mindex := rv.MapIndex(key)

		mkey, _err := f.MapKeyType.decodeSchema(key.Interface())
		if _err != nil {
			err = _err
			return
		}
		mval, _err := f.MapValueType.decodeSchema(mindex.Interface())
		if _err != nil {
			err = _err
			return
		}
		m.SetMapIndex(mkey, mval)
	}
	sch = m
	return
}

func (f *Fielder) decodeStruct(v any) (sch reflect.Value, err any) {
	errs := []any{}
	data, ok := v.(map[string]any)
	if !ok {
		_data, _err := Encode(v)
		if _err != nil {
			err = _err
			return
		}
		data, ok = _data.(map[string]any)
		if !ok {
			if !f.SkipOnErr {
				err = RetInvalidType(f)
				return
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
		fName := f.FieldsByIndex[i]
		fielder, ok := f.Children[fName]
		if !ok {
			continue
		}
		if fielder.Heritage {
			value = data
		} else {
			if value, ok = data[fielder.Name]; !ok {
				value, ok = data[fielder.RealName]
				if !ok {
					if fielder.Default == nil {
						if fielder.Required {
							errs = append(errs, map[string]any{fielder.Name: RetMissing(fielder)})
						}
						continue
					}
					value = fielder.Default
				}
			}
			if value == nil && !fielder.Nullable {
				if fielder.Default == nil {
					if fielder.Required {
						errs = append(errs, map[string]any{fielder.Name: RetMissing(fielder)})
					}
					continue
				}
				value = fielder.Default
			}
		}

		var rvF reflect.Value

		if fielder.Recursive {
			_rvF, e := fielder.decodeSchema(value)
			if e != nil {
				errs = append(errs, e)
				continue
			}
			rvF = _rvF
		} else {
			rvF = reflect.ValueOf(value)
		}

		if !SetReflectValue(schF, rvF, false) {
			if !fielder.SkipOnErr {
				errs = append(errs, map[string]any{fielder.Name: RetInvalidType(fielder)})
			}
			continue
		}
	}
	if len(errs) > 0 {
		if len(errs) == 1 {
			err = errs[0]
		} else {
			err = errs
		}
	}
	return
}

func (f *Fielder) decodeSchema(v any) (reflect.Value, any) {
	if v == "" && f.Type != reflect.String {
		v = nil
	}
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
			return f.New(), nil
		}
	}

	var rfVal = reflectOf(v)
	if rfVal.CanInt() || rfVal.CanFloat() && rfVal.Interface() == 0 {
		if f.NonZero {
			if f.Default.(int) == 0 {
				return reflect.Value{}, RetInvalidValue(f)
			}
			v = f.Default
			rfVal = reflectOf(v)
		}
	}
	switch f.Type {
	default:
		return f.decodePrimitive(rfVal)
	case reflect.Map:
		return f.decodeMap(rfVal)
	case reflect.Array, reflect.Slice:
		return f.decodeSlice(rfVal)
	case reflect.Struct:
		return f.decodeStruct(v)
	}
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

	return f.CheckSchPtr(sch), nil
}

func (f *Fielder) CheckSchPtr(r reflect.Value) any {

	if f.IsPointer && (r.CanAddr() && r.Kind() != reflect.Pointer) {
		return r.Addr().Interface()
	} else if !f.IsPointer && r.Kind() == reflect.Pointer {
		return r.Elem().Interface()
	}
	return r.Interface()
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

func (f *Fielder) ToMap() map[string]any {
	fildMap := map[string]any{}
	if n, ok := f.Tags["name"]; ok {
		fildMap["name"] = n
	} else {
		fildMap["name"] = f.Name
	}
	if in, ok := f.Tags["in"]; ok {
		fildMap["in"] = in
	}
	if st, ok := f.Tags["strType"]; ok {
		fildMap["type"] = st
	} else {
		fildMap["type"] = f.Type.String()
	}

	if len(f.Children) > 0 {
		childsMap := map[string]any{}
		for cn, cv := range f.Children {
			childsMap[cn] = cv.ToMap()
		}
		fildMap["schema"] = childsMap
	}
	return fildMap
}

func (f *Fielder) String() string {
	return EncodeToStringIndent("  ", f.ToMap())
}
