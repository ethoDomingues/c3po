# c3po

## A GoLang data validator

```go
	ParseSchema(struct{}) => struct{}
	ParseSchema(&struct{}) => *struct{}
	ParseSchema(&struct{Field:Value}) => *struct{Field: value} // with default value

	type Schema struct{
		Field `c3po:"-"` // omit this field
		Field `c3po:"realName"` // string: real name field
		Field `c3po:"name"` 	// string: name of validation	(default realName)
		Field `c3po:"escape"`	// bool: escape html value		(default false)
		Field `c3po:"required"` // bool:		...			 	(default false)
		Field `c3po:"nullable"` // bool:if true,allow nil value (default false)
		Field `c3po:"recursive"`// bool: deep validation	  	(default true)
		Field `c3po:"skiponerr"`// bool: omit on valid. error 	(default false)
	}
```

## Basic Usage

```go
package main

import (
 "encoding/json"
 "fmt"

 "github.com/ethoDomingues/c3po"
)

type Model struct {
 UUID string
}
type User struct {
 Model `c3po:"heritage"`
 Name  string `c3po:"name=username"`
 Email string `c3po:"required"`
}

type Studant struct {
 User
 Curse *Curse `c3po:"recursive=false"`
}

type Curse struct {
 Name     string
 Duration string
}

func main() {
 var userData = map[string]any{
  "uuid":  "aaaa-aaaa-aaaa", // here 'uuid' is inheritance from 'model'
  "email": "tião@localhost",
 }

 var studantData = map[string]any{
  "user": userData,
  "curse": &Curse{
   Name:     "mechanic",
   Duration: "infinity",
  },
 }

 u := &User{Name: "tião"} // here the 'name' field has a default value
 userSchema := c3po.ParseSchema(u)
 u2, _ := userSchema.Decode(userData)
 fmt.Println(c3po.EncodeToString(u))
 fmt.Println(c3po.EncodeToString(u2))

 s := &Studant{}
 studantSchema := c3po.ParseSchema(s)
 s2, _ := studantSchema.Decode(studantData)
 fmt.Println(c3po.EncodeToString(s))
 fmt.Println(c3po.EncodeToString(s2))
}

```

Output:

```sh
$ go run .
{
 "UUID": "",
 "Name": "tião",
 "Email": ""
}
{
 "UUID": "aaaa-aaaa-aaaa",
 "Name": "tião",
 "Email": "tião@localhost"
}
{
 "UUID": "",
 "Name": "",
 "Email": "",
 "Curse": null
}
{
 "UUID": "aaaa-aaaa-aaaa",
 "Name": "",
 "Email": "tião@localhost",
 "Curse": {
  "Name": "mechanic",
  "Duration": "infinit"
}
```
## Others Functions

```go
	c3po.Encode(struct{Name:"Jão", Age:99}) => map[string]any{"Name":"jão", "Age":99}
	c3po.Encode([]struct{Name:"Jão", Age:99}) => []map[string]any{"Name":"jão", "Age":99}

  
	c3po.EncodeToJSON(struct{Name:"Jão", Age:99}) => []byte{"{'Name':'jão', 'Age':99}"}
	c3po.EncodeToString(struct{Name:"Jão", Age:99}) => "{'Name':'jão', 'Age':99}"
```