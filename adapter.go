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
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"runtime"
	"strings"
	"time"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"go.mongodb.org/mongo-driver/mongo"
)

const CasbinMongodbDatabasename = "casbin"
const CasbinMongodbCollectionname = "casbin_rule"
const ContextTimeout = 30 * time.Second

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
	timeout      time.Duration
}

// finalizer is the destructor for adapter.
func finalizer(a *adapter) {
	if a.ownclient {
		a.close()
	}
}

// NewAdapter is the constructor for Adapter.
// param can be a mongodb uri string, *mongo.Database or *mongo.Collection
// If database name is not provided in the Mongo URI, 'casbin' will be used as database name.
func NewAdapter(param interface{}) (persist.Adapter, error) {
	a := &adapter{
		client:       nil,
		collection:   nil,
		databasename: "",
		ownclient:    false,
		timeout:      ContextTimeout,
	}
	a.filtered = false

	// database interface{} as string or *mongo.Database or *mongo.Collection
	switch param.(type) {
	case nil:
		panic(errors.New("nil not allowed"))

	case string: // Given string; handle as MongoDB Uri
		cs, err := connstring.Parse(param.(string))

		if err != nil {
			return nil, err
		}

		a.client, err = mongo.NewClient(options.Client().ApplyURI(param.(string)))

		if err != nil {
			return nil, err
		}

		a.databasename = cs.Database
		a.ownclient = true

	case *mongo.Database: // Given *mongo.Database; handle as it
		a.client = param.(*mongo.Database).Client()
		a.databasename = param.(*mongo.Database).Name()
		a.collection = param.(*mongo.Database).Collection(CasbinMongodbCollectionname)

	case *mongo.Collection: // Given *mongo.Collection; handle as it
		a.client = param.(*mongo.Collection).Database().Client()
		a.databasename = param.(*mongo.Collection).Database().Name()
		a.collection = param.(*mongo.Collection)

	case time.Duration:
		a.timeout = param.(time.Duration)

	default:
		panic(errors.New("string/*mongo.Database/*mongo.Collection required"))
	}

	// Open the DB, create it if not existed.
	if err := a.open(); err != nil {
		return nil, err
	}

	// Call the destructor when the object is released.
	runtime.SetFinalizer(a, finalizer)

	return a, nil
}

// NewAdapterWithDatabase is an alternative constructor for Adapter
// that does the same as NewAdapter, but uses *mongo.Database instead of a Mongo URI
func NewAdapterWithDatabase(mDatabase *mongo.Database) (persist.Adapter, error) {
	return NewAdapter(mDatabase)
}

// NewAdapterWithDatabase is an alternative constructor for Adapter
// that does the same as NewAdapter, but uses *mongo.Collection instead of a Mongo URI
func NewAdapterWithCollection(mCollection *mongo.Collection) (persist.Adapter, error) {
	return NewAdapter(mCollection)
}

// NewFilteredAdapter is the constructor for FilteredAdapter.
// Casbin will not automatically call LoadPolicy() for a filtered adapter.
func NewFilteredAdapter(uri string) (persist.FilteredAdapter, error) {
	a, err := NewAdapter(uri)

	if err != nil {
		return nil, err
	}

	a.(*adapter).filtered = true

	return a.(*adapter), nil
}

func (a *adapter) getContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), ContextTimeout)
	return ctx
}

func (a *adapter) forceConnect() error {
	err := a.client.Ping(a.getContext(), readpref.Primary())

	if err != nil {
		if err == mongo.ErrClientDisconnected {
			// try to reconnect
			if err := a.client.Connect(a.getContext()); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (a *adapter) open() error {
	// Force a ping to database host
	// if fails with ErrClientDisconnected then reconnect
	if err := a.forceConnect(); err != nil {
		panic(err)
	}

	if a.databasename == "" {
		a.databasename = CasbinMongodbDatabasename
	}

	// Create or use a collection
	if a.collection == nil {
		a.collection = a.client.Database(a.databasename).Collection(CasbinMongodbCollectionname)
	}
	indexes := []string{"ptype", "v0", "v1", "v2", "v3", "v4", "v5"}
	keysDoc := bsonx.Doc{}

	for _, k := range indexes {
		keysDoc = keysDoc.Append(k, bsonx.Int32(1))
	}

	if _, err := a.collection.Indexes().CreateOne(
		context.Background(),
		mongo.IndexModel{
			Keys:    keysDoc,
			Options: options.Index().SetUnique(true),
		},
	); err != nil {
		return err
	}

	return nil
}

func (a *adapter) close() {
	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	_ = a.client.Disconnect(ctx)
}

func (a *adapter) dropTable() error {
	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	err := a.collection.Drop(ctx)

	if err != nil {
		return err
	}

	return nil
}

func loadPolicyLine(line CasbinRule, model model.Model) {
	var p = []string{line.PType,
		line.V0, line.V1, line.V2, line.V3, line.V4, line.V5}
	var lineText string
	if line.V5 != "" {
		lineText = strings.Join(p, ", ")
	} else if line.V4 != "" {
		lineText = strings.Join(p[:6], ", ")
	} else if line.V3 != "" {
		lineText = strings.Join(p[:5], ", ")
	} else if line.V2 != "" {
		lineText = strings.Join(p[:4], ", ")
	} else if line.V1 != "" {
		lineText = strings.Join(p[:3], ", ")
	} else if line.V0 != "" {
		lineText = strings.Join(p[:2], ", ")
	}

	persist.LoadPolicyLine(lineText, model)
}

// LoadPolicy loads policy from database.
func (a *adapter) LoadPolicy(model model.Model) error {
	return a.LoadFilteredPolicy(model, nil)
}

// LoadFilteredPolicy loads matching policy lines from database. If not nil,
// the filter must be a valid MongoDB selector.
func (a *adapter) LoadFilteredPolicy(model model.Model, filter interface{}) error {
	if filter == nil {
		a.filtered = false
		filter = bson.D{{}}
	} else {
		a.filtered = true
	}
	line := CasbinRule{}

	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	cursor, err := a.collection.Find(ctx, filter)
	if err != nil {
		return err
	}

	for cursor.Next(ctx) {
		err := cursor.Decode(&line)
		if err != nil {
			return err
		}
		loadPolicyLine(line, model)
	}

	return cursor.Close(ctx)
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
	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	if _, err := a.collection.InsertMany(ctx, lines); err != nil {
		return err
	}

	return nil
}

// AddPolicy adds a policy rule to the storage.
func (a *adapter) AddPolicy(sec string, ptype string, rule []string) error {
	line := savePolicyLine(ptype, rule)

	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	if _, err := a.collection.InsertOne(ctx, line); err != nil {
		return err
	}

	return nil
}

// RemovePolicy removes a policy rule from the storage.
func (a *adapter) RemovePolicy(sec string, ptype string, rule []string) error {
	line := savePolicyLine(ptype, rule)

	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	if _, err := a.collection.DeleteOne(ctx, line); err != nil {
		return err
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

	ctx, cancel := context.WithTimeout(context.TODO(), a.timeout)
	defer cancel()

	if _, err := a.collection.DeleteMany(ctx, selector); err != nil {
		return err
	}

	return nil
}
