MongoDB Adapter [![Build Status](https://travis-ci.org/litixsoft/mongodb-adapter.svg?branch=master)](https://travis-ci.org/litixsoft/mongodb-adapter) [![Coverage Status](https://coveralls.io/repos/github/litixsoft/mongodb-adapter/badge.svg?branch=master)](https://coveralls.io/github/litixsoft/mongodb-adapter?branch=master) [![Godoc](https://godoc.org/github.com/litixsoft/mongodb-adapter?status.svg)](https://godoc.org/github.com/litixsoft/mongodb-adapter)
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

## Filtered Policies

```go
import "github.com/globalsign/mgo/bson"

// This adapter also implements the FilteredAdapter interface. This allows for
// efficent, scalable enforcement of very large policies:
filter := &bson.M{"v0": "alice"}
e.LoadFilteredPolicy(filter)

// The loaded policy is now a subset of the policy in storage, containing only
// the policy lines that match the provided filter. This filter should be a
// valid MongoDB selector using BSON. A filtered policy cannot be saved.
```

## Getting Help

- [Casbin](https://github.com/casbin/casbin)

## License

This project is under Apache 2.0 License. See the [LICENSE](LICENSE) file for the full license text.
