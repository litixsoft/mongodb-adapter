MongoDB Adapter [![Build Status](https://travis-ci.org/litixsoft/mongodb-adapter.svg?branch=master)](https://travis-ci.org/litixsoft/mongodb-adapter)
====

MongoDB Adapter is the [Mongo DB](https://www.mongodb.com) adapter for [Casbin](https://github.com/casbin/casbin). With this library, Casbin can load policy from MongoDB or save policy to it.

## Installation

    go get github.com/litixsoft/mongodb-adapter

## Simple Example

```go
package main

import (
	"github.com/casbin/casbin"
	"github.com/globalsign/mgo"
	"github.com/litixsoft/mongodb-adapter"
	"log"
	"time"
)

func main() {
    // Connect to mongodb 
    dialInfo := &mgo.DialInfo{
        Addrs:   []string{"localhost:27017"},
        Timeout: 30 * time.Second,
    }
    session, err := mgo.DialWithInfo(dialInfo)
    if err != nil {
        log.Fatal(err)
    }
    session.SetMode(mgo.Monotonic, true)
	
    // Initialize a MongoDB adapter and use it in a Casbin enforcer:
    // The adapter will use the given database and collection name. 
    // If it doesn't exist, the adapter will create it automatically. 
    a := mongodbadapter.NewAdapter(session, "my-db", "casbin-rules")
    e := casbin.NewEnforcer("examples/rbac_model.conf", a)
	
    // Load the policy from DB. 
    e.LoadPolicy()
	
    // Check the permission.
    e.Enforce("alice", "data1", "read")
	
    // Modify the policy.
    // e.AddPolicy(...) 
    // e.RemovePolicy(...)
	
    // Save the policy back to DB.
    e.SavePolicy()
}
```
## Getting Help

- [Casbin](https://github.com/casbin/casbin)

## License

This project is under Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.
