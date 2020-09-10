// Code generated by SQLBoiler 3.7.1 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/randomize"
	"github.com/volatiletech/sqlboiler/strmangle"
)

var (
	// Relationships sometimes use the reflection helper queries.Equal/queries.Assign
	// so force a package dependency in case they don't.
	_ = queries.Equal
)

func testNodeLocations(t *testing.T) {
	t.Parallel()

	query := NodeLocations()

	if query.Query == nil {
		t.Error("expected a query, got nothing")
	}
}

func testNodeLocationsDelete(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if rowsAff, err := o.Delete(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testNodeLocationsQueryDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if rowsAff, err := NodeLocations().DeleteAll(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testNodeLocationsSliceDeleteAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice := NodeLocationSlice{o}

	if rowsAff, err := slice.DeleteAll(ctx, tx); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only have deleted one row, but affected:", rowsAff)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 0 {
		t.Error("want zero records, got:", count)
	}
}

func testNodeLocationsExists(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	e, err := NodeLocationExists(ctx, tx, o.Timestamp, o.Bin, o.Country)
	if err != nil {
		t.Errorf("Unable to check if NodeLocation exists: %s", err)
	}
	if !e {
		t.Errorf("Expected NodeLocationExists to return true, but got false.")
	}
}

func testNodeLocationsFind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	nodeLocationFound, err := FindNodeLocation(ctx, tx, o.Timestamp, o.Bin, o.Country)
	if err != nil {
		t.Error(err)
	}

	if nodeLocationFound == nil {
		t.Error("want a record, got nil")
	}
}

func testNodeLocationsBind(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if err = NodeLocations().Bind(ctx, tx, o); err != nil {
		t.Error(err)
	}
}

func testNodeLocationsOne(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if x, err := NodeLocations().One(ctx, tx); err != nil {
		t.Error(err)
	} else if x == nil {
		t.Error("expected to get a non nil record")
	}
}

func testNodeLocationsAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	nodeLocationOne := &NodeLocation{}
	nodeLocationTwo := &NodeLocation{}
	if err = randomize.Struct(seed, nodeLocationOne, nodeLocationDBTypes, false, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}
	if err = randomize.Struct(seed, nodeLocationTwo, nodeLocationDBTypes, false, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = nodeLocationOne.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}
	if err = nodeLocationTwo.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice, err := NodeLocations().All(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 2 {
		t.Error("want 2 records, got:", len(slice))
	}
}

func testNodeLocationsCount(t *testing.T) {
	t.Parallel()

	var err error
	seed := randomize.NewSeed()
	nodeLocationOne := &NodeLocation{}
	nodeLocationTwo := &NodeLocation{}
	if err = randomize.Struct(seed, nodeLocationOne, nodeLocationDBTypes, false, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}
	if err = randomize.Struct(seed, nodeLocationTwo, nodeLocationDBTypes, false, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = nodeLocationOne.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}
	if err = nodeLocationTwo.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 2 {
		t.Error("want 2 records, got:", count)
	}
}

func testNodeLocationsInsert(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testNodeLocationsInsertWhitelist(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Whitelist(nodeLocationColumnsWithoutDefault...)); err != nil {
		t.Error(err)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}
}

func testNodeLocationsReload(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	if err = o.Reload(ctx, tx); err != nil {
		t.Error(err)
	}
}

func testNodeLocationsReloadAll(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice := NodeLocationSlice{o}

	if err = slice.ReloadAll(ctx, tx); err != nil {
		t.Error(err)
	}
}

func testNodeLocationsSelect(t *testing.T) {
	t.Parallel()

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	slice, err := NodeLocations().All(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if len(slice) != 1 {
		t.Error("want one record, got:", len(slice))
	}
}

var (
	nodeLocationDBTypes = map[string]string{`Timestamp`: `bigint`, `Height`: `bigint`, `NodeCount`: `integer`, `Country`: `character varying`, `Bin`: `character varying`}
	_                   = bytes.MinRead
)

func testNodeLocationsUpdate(t *testing.T) {
	t.Parallel()

	if 0 == len(nodeLocationPrimaryKeyColumns) {
		t.Skip("Skipping table with no primary key columns")
	}
	if len(nodeLocationAllColumns) == len(nodeLocationPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	if rowsAff, err := o.Update(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("should only affect one row but affected", rowsAff)
	}
}

func testNodeLocationsSliceUpdateAll(t *testing.T) {
	t.Parallel()

	if len(nodeLocationAllColumns) == len(nodeLocationPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	o := &NodeLocation{}
	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationColumnsWithDefault...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Insert(ctx, tx, boil.Infer()); err != nil {
		t.Error(err)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}

	if count != 1 {
		t.Error("want one record, got:", count)
	}

	if err = randomize.Struct(seed, o, nodeLocationDBTypes, true, nodeLocationPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	// Remove Primary keys and unique columns from what we plan to update
	var fields []string
	if strmangle.StringSliceMatch(nodeLocationAllColumns, nodeLocationPrimaryKeyColumns) {
		fields = nodeLocationAllColumns
	} else {
		fields = strmangle.SetComplement(
			nodeLocationAllColumns,
			nodeLocationPrimaryKeyColumns,
		)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	typ := reflect.TypeOf(o).Elem()
	n := typ.NumField()

	updateMap := M{}
	for _, col := range fields {
		for i := 0; i < n; i++ {
			f := typ.Field(i)
			if f.Tag.Get("boil") == col {
				updateMap[col] = value.Field(i).Interface()
			}
		}
	}

	slice := NodeLocationSlice{o}
	if rowsAff, err := slice.UpdateAll(ctx, tx, updateMap); err != nil {
		t.Error(err)
	} else if rowsAff != 1 {
		t.Error("wanted one record updated but got", rowsAff)
	}
}

func testNodeLocationsUpsert(t *testing.T) {
	t.Parallel()

	if len(nodeLocationAllColumns) == len(nodeLocationPrimaryKeyColumns) {
		t.Skip("Skipping table with only primary key columns")
	}

	seed := randomize.NewSeed()
	var err error
	// Attempt the INSERT side of an UPSERT
	o := NodeLocation{}
	if err = randomize.Struct(seed, &o, nodeLocationDBTypes, true); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	ctx := context.Background()
	tx := MustTx(boil.BeginTx(ctx, nil))
	defer func() { _ = tx.Rollback() }()
	if err = o.Upsert(ctx, tx, false, nil, boil.Infer(), boil.Infer()); err != nil {
		t.Errorf("Unable to upsert NodeLocation: %s", err)
	}

	count, err := NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}

	// Attempt the UPDATE side of an UPSERT
	if err = randomize.Struct(seed, &o, nodeLocationDBTypes, false, nodeLocationPrimaryKeyColumns...); err != nil {
		t.Errorf("Unable to randomize NodeLocation struct: %s", err)
	}

	if err = o.Upsert(ctx, tx, true, nil, boil.Infer(), boil.Infer()); err != nil {
		t.Errorf("Unable to upsert NodeLocation: %s", err)
	}

	count, err = NodeLocations().Count(ctx, tx)
	if err != nil {
		t.Error(err)
	}
	if count != 1 {
		t.Error("want one record, got:", count)
	}
}
