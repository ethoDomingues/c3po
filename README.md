# c3po

## A GoLang data validator

## Basic Usage

```go
package main

import (
 "encoding/json"
 "fmt"

 "github.com/ethodomingues/c3po"
)

type User struct {
 Name,
 Email string
}

type Studant struct {
 User
 Curse string
}

func main() {
 var userData = map[string]any{
  "name":  "tião",
  "email": "tião@localhost",
 }

 var studantData = map[string]any{
  "user":  userData,
  "curse": "mechanic",
 }

 u := &User{}
 userSchema := c3po.ParseSchema(u)
 u2, _err_ := userSchema.Mount(userData)
 Print(u)
 Print(u2)

 s := &Studant{}
 studantSchema := c3po.ParseSchema(s)
 s2, _ := studantSchema.Mount(studantData)
 Print(s)
 Print(s2)

}

func Print(v any) {
 if b, err := json.MarshalIndent(v, "", " "); err != nil {
  fmt.Println("{}")
 } else {
  fmt.Println(string(b))
 }
}

```

Output:

```sh
$ go run .
{
 "Name": "",
 "Email": ""
}

{
 "Name": "tião",
 "Email": "tião@localhost"
}

{
 "Name": "",
 "Email": "",
 "Curse": ""
}

{
 "Name": "tião",
 "Email": "tião@localhost",
 "Curse": "mechanic"
}

```

### Struct Tags Allowed

- ### strict

  - for: Slices
  - default: true

- ### required

  - for: All
  - default: true

- ### name

  - for: All
  - default: Struct field name

- ### escape

  - for: String
  - default: false
