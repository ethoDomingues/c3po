# c3po

GoLang data validator

```go
package main

import (
    "github.com/ethoDomingues/c3po"
)

type User struct {
    Name,
    Email string
}

type Studant struct {
    User
    Curse string
}

func main(){
    var userData = map[string]any{
        "name":"tião",
        "email":"tião@localhost",
    }
    var studantData = map[string]any{
        "user":userData,
        "curse":"mechanic"
    }

}
```
