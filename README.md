MongoDB Adapter [![Build Status](https://travis-ci.org/litixsoft/mongodb-adapter.svg?branch=master)](https://travis-ci.org/litixsoft/mongodb-adapter) [![Coverage Status](https://coveralls.io/repos/github/litixsoft/mongodb-adapter/badge.svg?branch=master)](https://coveralls.io/github/litixsoft/mongodb-adapter?branch=master) [![Godoc](https://godoc.org/github.com/litixsoft/mongodb-adapter?status.svg)](https://godoc.org/github.com/litixsoft/mongodb-adapter)
====

MongoDB Adapter is the [Mongo DB](https://www.mongodb.com) adapter for [Casbin](https://github.com/casbin/casbin). With this library, Casbin can load policy from MongoDB or save policy to it.

```
Note: This library use the official go-mongo-driver.
```

## Installation

    go get -u github.com/litixsoft/mongodb-adapter

## Simple Example

```go
package main

import (
	"github.com/casbin/casbin/v2"
	"github.com/litixsoft/mongodb-adapter"
)

func main() {
	// Initialize a MongoDB adapter and use it in a Casbin enforcer:
	// The adapter accept a mongodb uri string, a *mongo.Database or *mongo.Collection 
    // If no database given in uri string, the adapter will use the database named "casbin".
	a := mongodbadapter.NewAdapter("mongodb://127.0.0.1:27017") // Your MongoDB URI. 
	
	// Or you can use an existing DB "abc" like this:
	// The adapter will use the table named "casbin_rule".
	// If it doesn't exist, the adapter will create it automatically.
	// a := mongodbadapter.NewAdapter("mongodb://127.0.0.1:27017/abc")

	e, err := casbin.NewEnforcer("examples/rbac_model.conf", a)
	if err != nil {
		panic(err)
	}

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
import "go.mongodb.org/mongo-driver/bson"

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
