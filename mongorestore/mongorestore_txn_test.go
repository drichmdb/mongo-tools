// Copyright (C) MongoDB, Inc. 2019-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongorestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mongodb/mongo-tools/common/bsonutil"
	"github.com/mongodb/mongo-tools/common/db"
	"github.com/mongodb/mongo-tools/common/testtype"
	"github.com/mongodb/mongo-tools/common/testutil"
	. "github.com/smartystreets/goconvey/convey"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Test files with applyOps transaction entries for
// MongoDB server versions <6.1 and >=6.1, respectively.
const txnTestDataFilePre61 = "testdata/transactions.json"
const txnTestDataFile61Plus = "testdata/transactions-6.1.json"

type txnTestDataMap map[string]*txnTestDataCase

type txnTestDataCase struct {
	Ops       []db.Oplog `bson:"ops"`
	NS        string     `bson:"ns"`
	PostImage []bson.D   `bson:"postimage"`
}

func TestMongorestoreTxns(t *testing.T) {
	testtype.SkipUnlessTestType(t, testtype.IntegrationTestType)
	client, err := testutil.GetBareSession()
	if err != nil {
		t.Fatalf("No server available")
	}

	restore, err := getRestoreWithArgs()
	if err != nil {
		t.Fatal(err)
	}

	file := txnTestDataFilePre61
	if restore.serverVersion.GTE(db.Version{6, 1, 0}) {
		file = txnTestDataFile61Plus
	}
	data, err := readTxnTestData(file)
	if err != nil {
		t.Fatal(err)
	}

	// Create test collections (if they don't exist) and clear documents.
	for _, v := range data {
		parts := strings.SplitN(v.NS, ".", 2)
		db := client.Database(parts[0])
		coll := db.Collection(parts[1])
		err := coll.Drop(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		res := db.RunCommand(context.Background(), bson.D{{"create", parts[1]}})
		if res.Err() != nil {
			t.Fatal(res.Err())
		}
	}

	// Create a dump directory from transactions.json
	dumpPath := createTxnTestDataDir(t, data)

	Convey("With a test MongoRestore", t, func() {
		args := []string{
			OplogReplayOption,
			DropOption,
			dumpPath,
		}
		restore, err := getRestoreWithArgs(args...)
		So(err, ShouldBeNil)
		defer restore.Close()

		result := restore.Restore()
		So(result.Err, ShouldBeNil)

		for k, v := range data {
			_, err = Println("postImageCheck for", k)
			So(err, ShouldBeNil)
			So(postImageCheck(client, v), ShouldBeNil)
		}
	})
}

// createTxnTestDataDir constructs a dump directory with an oplog.bson
// file that randomly interleaves different cases from the
// testdata/transactions.json file.  This tests that different transactions
// can be cached while continuing processing waiting for a committing entry.
func createTxnTestDataDir(t *testing.T, data txnTestDataMap) string {
	var opStreams [][]db.Oplog
	for _, v := range data {
		if len(v.Ops) != 0 {
			opStreams = append(opStreams, v.Ops)
		}
	}

	dumpDir := testDumpDir{
		dirName: "txntest",
		oplog:   testutil.MergeOplogStreams(opStreams),
	}

	err := dumpDir.Create()
	if err != nil {
		t.Fatal(err)
	}

	return dumpDir.Path()
}

func readTxnTestData(filename string) (txnTestDataMap, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("couldn't load %s: %v", filename, err)
	}
	var data bson.Raw
	err = bson.UnmarshalExtJSON(b, false, &data)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode JSON: %v", err)
	}
	txnTestData := make(txnTestDataMap)
	err = bson.Unmarshal(data, &txnTestData)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode test data: %v", err)
	}

	return txnTestData, nil
}

func postImageCheck(client *mongo.Client, c *txnTestDataCase) error {
	expected := make(map[int]bson.D)
	for _, v := range c.PostImage {
		id, err := bsonutil.FindIntByKey("_id", &v)
		if err != nil {
			return err
		}
		expected[id] = v
	}

	parts := strings.SplitN(c.NS, ".", 2)
	db := client.Database(parts[0])
	coll := db.Collection(parts[1])

	cursor, err := coll.Find(context.Background(), bson.D{})
	if err != nil {
		return err
	}
	defer cursor.Close(context.Background())
	var docs []bson.D
	err = cursor.All(context.Background(), &docs)
	if err != nil {
		return err
	}

	for _, got := range docs {
		id, err := bsonutil.FindIntByKey("_id", &got)
		if err != nil {
			return err
		}
		want, ok := expected[id]
		if !ok {
			return fmt.Errorf("got unexpected document with _id '%d'", id)
		}
		if diff := cmp.Diff(got, want); diff != "" {
			return errors.New(diff)
		}
		delete(expected, id)
	}

	// Check if all documents were found
	if len(expected) != 0 {
		var missing []int
		for i := range expected {
			missing = append(missing, i)
		}
		return fmt.Errorf("missing documents: %v", missing)
	}

	return nil
}
