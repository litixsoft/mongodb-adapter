// Copyright 2018 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbadapter

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"runtime"
	"time"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"go.mongodb.org/mongo-driver/mongo"
)

// CasbinRule represents a rule in Casbin.
type CasbinRule struct {
	PType string
	V0    string
	V1    string
	V2    string
	V3    string
	V4    string
	V5    string
}

// adapter represents the MongoDB adapter for policy storage.
type adapter struct {
	client       *mongo.Client
	collection   *mongo.Collection
	databasename string
	ownclient    bool
	filtered     bool
	context      context.Context
}

// finalizer is the destructor for adapter.
func finalizer(a *adapter) {
	if a.ownclient {
		a.close()
	}
}

func newAdapterWithParams(mC *mongo.Client, database string, autodisconnect bool) persist.Adapter {
	a := &adapter{client: mC}
	a.filtered = false
	a.ownclient = autodisconnect
	a.databasename = database
	a.context, _ = context.WithTimeout(context.Background(), 10*time.Second)

	// Open the DB, create it if not existed.
	a.open()

	// Call the destructor when the object is released.
	runtime.SetFinalizer(a, finalizer)

	return a
}

// NewAdapter is the constructor for Adapter. If database name is not provided
// in the Mongo URL, 'casbin' will be used as database name.
func NewAdapter(uri string) persist.Adapter {
	cs, err := connstring.Parse(uri)

	if err != nil {
		panic(err)
	}

	mC, err := mongo.NewClient(options.Client().ApplyURI(uri))

	if err != nil {
		panic(err)
	}

	return newAdapterWithParams(mC, cs.Database, true)
}

// NewAdapterWithClient is an alternative constructor for Adapter
// that does the same as NewAdapter, but uses mgo.DialInfo instead of a Mongo URL
func NewAdapterWithClient(mC *mongo.Client, database string) persist.Adapter {
	return newAdapterWithParams(mC, database, false)
}

// NewFilteredAdapter is the constructor for FilteredAdapter.
// Casbin will not automatically call LoadPolicy() for a filtered adapter.
func NewFilteredAdapter(uri string) persist.FilteredAdapter {
	a := NewAdapter(uri).(*adapter)
	a.filtered = true

	return a
}

func (a *adapter) open() {
	// FailFast will cause connection and query attempts to fail faster when
	// the server is unavailable, instead of retrying until the configured
	// timeout period. Note that an unavailable server may silently drop
	// packets instead of rejecting them, in which case it's impossible to
	// distinguish it from a slow server, so the timeout stays relevant.
	if err := a.client.Connect(a.context); err != nil {
		panic(err)
	}

	if a.databasename == "" {
		a.databasename = "casbin"
	}

	a.collection = a.client.Database(a.databasename).Collection("casbin_rule")

	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{{"ptype", 1}},
		},
		{
			Keys: bson.D{{"v0", 1}},
		},
		{
			Keys: bson.D{{"v1", 1}},
		},
		{
			Keys: bson.D{{"v2", 1}},
		},
		{
			Keys: bson.D{{"v3", 1}},
		},
		{
			Keys: bson.D{{"v4", 1}},
		},
		{
			Keys: bson.D{{"v5", 1}},
		},
	}

	if _, err := a.collection.Indexes().CreateMany(a.context, indexModels); err != nil {
		panic(err)
	}
}

func (a *adapter) close() {
	_ = a.client.Disconnect(a.context)
}

func (a *adapter) dropTable() error {
	err := a.collection.Drop(a.context)
	if err != nil {
		if err.Error() != "ns not found" {
			return err
		}
	}
	return nil
}

func loadPolicyLine(line CasbinRule, model model.Model) {
	key := line.PType
	sec := key[:1]

	var tokens []string

	// Helper func; breakable
	func() {
		if line.V0 != "" {
			tokens = append(tokens, line.V0)
		} else {
			return
		}

		if line.V1 != "" {
			tokens = append(tokens, line.V1)
		} else {
			return
		}

		if line.V2 != "" {
			tokens = append(tokens, line.V2)
		} else {
			return
		}

		if line.V3 != "" {
			tokens = append(tokens, line.V3)
		} else {
			return
		}

		if line.V4 != "" {
			tokens = append(tokens, line.V4)
		} else {
			return
		}

		if line.V5 != "" {
			tokens = append(tokens, line.V5)
		} else {
			return
		}
	}()

	model[sec][key].Policy = append(model[sec][key].Policy, tokens)
}

// LoadPolicy loads policy from database.
func (a *adapter) LoadPolicy(model model.Model) error {
	return a.LoadFilteredPolicy(model, nil)
}

// LoadFilteredPolicy loads matching policy lines from database. If not nil,
// the filter must be a valid MongoDB selector.
func (a *adapter) LoadFilteredPolicy(model model.Model, filter interface{}) error {
	a.filtered = filter != nil

	if !a.filtered {
		filter = bson.M{}
	}

	cursor, err := a.collection.Find(a.context, filter)

	if err != nil {
		return err
	}

	for cursor.Next(a.context) {
		var line CasbinRule

		if err := cursor.Decode(&line); err != nil {
			return err
		}

		loadPolicyLine(line, model)
	}

	return nil
}

// IsFiltered returns true if the loaded policy has been filtered.
func (a *adapter) IsFiltered() bool {
	return a.filtered
}

func savePolicyLine(ptype string, rule []string) CasbinRule {
	line := CasbinRule{
		PType: ptype,
	}

	if len(rule) > 0 {
		line.V0 = rule[0]
	}
	if len(rule) > 1 {
		line.V1 = rule[1]
	}
	if len(rule) > 2 {
		line.V2 = rule[2]
	}
	if len(rule) > 3 {
		line.V3 = rule[3]
	}
	if len(rule) > 4 {
		line.V4 = rule[4]
	}
	if len(rule) > 5 {
		line.V5 = rule[5]
	}

	return line
}

// SavePolicy saves policy to database.
func (a *adapter) SavePolicy(model model.Model) error {
	if a.filtered {
		return errors.New("cannot save a filtered policy")
	}
	if err := a.dropTable(); err != nil {
		return err
	}

	var lines []interface{}

	for ptype, ast := range model["p"] {
		for _, rule := range ast.Policy {
			line := savePolicyLine(ptype, rule)
			lines = append(lines, &line)
		}
	}

	for ptype, ast := range model["g"] {
		for _, rule := range ast.Policy {
			line := savePolicyLine(ptype, rule)
			lines = append(lines, &line)
		}
	}

	_, err := a.collection.InsertMany(a.context, lines)
	return err
}

// AddPolicy adds a policy rule to the storage.
func (a *adapter) AddPolicy(sec string, ptype string, rule []string) error {
	line := savePolicyLine(ptype, rule)
	_, err := a.collection.InsertOne(a.context, line)

	return err
}

// RemovePolicy removes a policy rule from the storage.
func (a *adapter) RemovePolicy(sec string, ptype string, rule []string) error {
	line := savePolicyLine(ptype, rule)
	_, err := a.collection.DeleteOne(a.context, line)

	if err != nil {
		switch err {
		case mongo.ErrNoDocuments:
			return nil
		default:
			return err
		}
	}
	return nil
}

// RemoveFilteredPolicy removes policy rules that match the filter from the storage.
func (a *adapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	selector := make(map[string]interface{})
	selector["ptype"] = ptype

	if fieldIndex <= 0 && 0 < fieldIndex+len(fieldValues) {
		if fieldValues[0-fieldIndex] != "" {
			selector["v0"] = fieldValues[0-fieldIndex]
		}
	}
	if fieldIndex <= 1 && 1 < fieldIndex+len(fieldValues) {
		if fieldValues[1-fieldIndex] != "" {
			selector["v1"] = fieldValues[1-fieldIndex]
		}
	}
	if fieldIndex <= 2 && 2 < fieldIndex+len(fieldValues) {
		if fieldValues[2-fieldIndex] != "" {
			selector["v2"] = fieldValues[2-fieldIndex]
		}
	}
	if fieldIndex <= 3 && 3 < fieldIndex+len(fieldValues) {
		if fieldValues[3-fieldIndex] != "" {
			selector["v3"] = fieldValues[3-fieldIndex]
		}
	}
	if fieldIndex <= 4 && 4 < fieldIndex+len(fieldValues) {
		if fieldValues[4-fieldIndex] != "" {
			selector["v4"] = fieldValues[4-fieldIndex]
		}
	}
	if fieldIndex <= 5 && 5 < fieldIndex+len(fieldValues) {
		if fieldValues[5-fieldIndex] != "" {
			selector["v5"] = fieldValues[5-fieldIndex]
		}
	}

	_, err := a.collection.DeleteMany(a.context, selector)
	return err
}
