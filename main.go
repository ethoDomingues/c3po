package c3po

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Fielder struct {
	Name     string `json:"name,omitempty"`
	realName string
	Type     reflect.Kind        `json:"type,omitempty"`
	Children map[string]*Fielder `json:"children,omitempty"`

	Required  bool `json:"required,"`
	recursive bool

	SliceOf reflect.Type `json:"-"`
	Schema  any          `json:"-"`
}

func (f *Fielder) String() string {
	s, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return ""
	}
	return string(s)
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
			kvTags[kv[0]] = kv[1]
		}
	}
	return kvTags
}

func ParseSchema(name, tagKey string, schema any) *Fielder {
	schemaFields := &Fielder{}

	var rt reflect.Type
	var rv reflect.Value

	rv = reflect.ValueOf(schema)
	rt = reflect.TypeOf(schema)

	if rv.Kind() == reflect.Pointer {
		rTmp := rv.Elem()
		if rTmp.IsValid() {
			rv = rTmp
			rt = rt.Elem()
		}
	}

	schemaFields.Name = name
	schemaFields.Type = rt.Kind()
	schemaFields.Schema = schema
	schemaFields.Children = map[string]*Fielder{}

	switch rt.Kind() {
	case reflect.Struct:
		for i := 0; i < rt.NumField(); i++ {
			var (
				name = ""
				req  = false
				rec  = true
			)

			fv := rv.Field(i)
			ft := rt.Field(i)

			tags := parseTags(ft.Tag.Get(tagKey))
			if v, ok := tags["required"]; ok && (v == "" || v == "true") {
				req = true
			}

			if v, ok := tags["recursive"]; ok && (v == "false") {
				rec = false
			}

			if v, ok := tags["name"]; ok && v != "" {
				name = v
			} else {
				name = strings.ToLower(string(ft.Name[0])) + ft.Name[1:]
			}
			if rec {
				schemaFields.Children[name] = ParseSchema(name, tagKey, fv.Interface())
			} else {
				schemaFields.Children[name] = &Fielder{
					Type:   fv.Kind(),
					Schema: fv.Interface(),
				}
			}
			schemaFields.Children[name].Name = name
			schemaFields.Children[name].realName = ft.Name
			schemaFields.Children[name].Required = req
			schemaFields.Children[name].recursive = rec

		}
	case reflect.Slice:
		sliceObjet := reflect.New(rv.Type().Elem()).Elem()
		schemaFields.Type = reflect.Slice
		schemaFields.SliceOf = sliceObjet.Type()
		schemaFields.Children["[]"] = ParseSchema("[]", tagKey, sliceObjet.Interface())
	}

	return schemaFields
}

func setReflectValue(r reflect.Value, v reflect.Value) bool {
	rKind := r.Kind()
	ok := false

	if rKind != v.Kind() {
		v = convert(v, r.Type(), &ok)
	} else {
		ok = true
	}

	if !ok {
		return false
	}

	r.Set(v)
	return true
}

func convert(v reflect.Value, t reflect.Type, convt *bool) reflect.Value {
	*convt = true // se acontecer algum erro de converção, try() return false
	defer try(convt)
	switch t.Kind() {
	case reflect.Float32, reflect.Float64:
		switch v.Kind() {
		case reflect.String:
			if i, err := strconv.ParseFloat(v.Interface().(string), 64); err == nil {
				return reflect.ValueOf(i).Convert(t)
			}
		case reflect.Int, reflect.Int64:
			return v.Convert(t)
		}
	case reflect.Int, reflect.Int64:
		switch v.Kind() {
		case reflect.String:
			if val, err := strconv.ParseFloat(v.Interface().(string), 64); err == nil {
				return reflect.ValueOf(val).Convert(t)
			}
		case reflect.Float32, reflect.Float64:
			return v.Convert(t)
		}
	case reflect.Bool:
		if v.Kind() == reflect.String {
			// "true" == true | "false" == false. other thing == error
			if v.Interface() == "true" {
				return reflect.ValueOf(true)
			} else if v.Interface() == "false" {
				return reflect.ValueOf(false)
			}
		}
	}
	*convt = false // não converteu, então retorna false
	return reflect.Value{}
}

func try(convt *bool) {
	if err := recover(); err != nil {
		*convt = false
		fmt.Println("batata -> ", err)
	}
}

func (f *Fielder) MountSchema(data map[string]any) (reflect.Value, error) {
	schT := reflect.TypeOf(f.Schema)
	if schT.Kind() == reflect.Pointer {
		schT = schT.Elem()
	}
	sch := reflect.New(schT).Elem()
	var err error
	for fieldName, fielder := range f.Children {
		if v, ok := data[fieldName]; ok {
			var schVal reflect.Value
			if dataFielder, ok := v.(map[string]any); ok {
				schVal, err = fielder.MountSchema(dataFielder)
				if err != nil {
					fmt.Println("missing -> ", fieldName)
				}
			} else {
				schVal = reflect.ValueOf(v)
				if fielder.Type == reflect.Slice {
					if schVal.Kind() == reflect.Slice {
						lenSlice := schVal.Len()
						slice := reflect.MakeSlice(reflect.SliceOf(fielder.SliceOf), lenSlice, lenSlice)
						for i := 0; i < lenSlice; i++ {
							s := schVal.Index(i)
							sf := fielder.Children["[]"]
							slicSch, err := sf.MountSchema(s.Interface().(map[string]any))
							if err != nil {
								fmt.Println("missing -> ", fieldName)
							}
							sItem := slice.Index(i)
							sItem.Set(slicSch)
						}
						schVal = slice
					}
				}
			}
			rf := sch.FieldByName(fielder.realName)
			if !setReflectValue(rf, schVal) {
				fmt.Println("invalid type -> ", fielder.realName)
			}
		} else if fielder.Required {
			fmt.Println("missing -> ", fieldName)
		}
	}
	return sch, nil
}
