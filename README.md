# c3po

## A GoLang data validator

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
 Print(u)
 Print(u2)

 s := &Studant{}
 studantSchema := c3po.ParseSchema(s)
 s2, _ := studantSchema.Decode(studantData)
 Print(s)
 Print(s2)

}

func Print(v any) {
 b, _ := json.MarshalIndent(v, "", " ")
 fmt.Println(string(b))
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

### Struct Tags Allowed

- ### escape

  - for: String
  - default: false

- ### nullable

  - for: All
  - default: false

- ### required

  - for: All
  - default: false

- ### name

  - for: All
  - default: Struct field name

- ### heritage

  - for: struct field
  - default: false

- ### recursive

  - for: structs & slices
  - default: true

- ### SkipOnErr

  - for: All
  - default: false
