// Code generated by SQLBoiler 3.5.0 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/volatiletech/sqlboiler/queries/qmhelper"
	"github.com/volatiletech/sqlboiler/strmangle"
)

// Reddit is an object representing the database table.
type Reddit struct {
	Date           time.Time `boil:"date" json:"date" toml:"date" yaml:"date"`
	Subreddit      string    `boil:"subreddit" json:"subreddit" toml:"subreddit" yaml:"subreddit"`
	Subscribers    int       `boil:"subscribers" json:"subscribers" toml:"subscribers" yaml:"subscribers"`
	ActiveAccounts int       `boil:"active_accounts" json:"active_accounts" toml:"active_accounts" yaml:"active_accounts"`

	R *redditR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L redditL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var RedditColumns = struct {
	Date           string
	Subreddit      string
	Subscribers    string
	ActiveAccounts string
}{
	Date:           "date",
	Subreddit:      "subreddit",
	Subscribers:    "subscribers",
	ActiveAccounts: "active_accounts",
}

// Generated where

var RedditWhere = struct {
	Date           whereHelpertime_Time
	Subreddit      whereHelperstring
	Subscribers    whereHelperint
	ActiveAccounts whereHelperint
}{
	Date:           whereHelpertime_Time{field: "\"reddit\".\"date\""},
	Subreddit:      whereHelperstring{field: "\"reddit\".\"subreddit\""},
	Subscribers:    whereHelperint{field: "\"reddit\".\"subscribers\""},
	ActiveAccounts: whereHelperint{field: "\"reddit\".\"active_accounts\""},
}

// RedditRels is where relationship names are stored.
var RedditRels = struct {
}{}

// redditR is where relationships are stored.
type redditR struct {
}

// NewStruct creates a new relationship struct
func (*redditR) NewStruct() *redditR {
	return &redditR{}
}

// redditL is where Load methods for each relationship are stored.
type redditL struct{}

var (
	redditAllColumns            = []string{"date", "subreddit", "subscribers", "active_accounts"}
	redditColumnsWithoutDefault = []string{"date", "subreddit", "subscribers", "active_accounts"}
	redditColumnsWithDefault    = []string{}
	redditPrimaryKeyColumns     = []string{"date"}
)

type (
	// RedditSlice is an alias for a slice of pointers to Reddit.
	// This should generally be used opposed to []Reddit.
	RedditSlice []*Reddit

	redditQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	redditType                 = reflect.TypeOf(&Reddit{})
	redditMapping              = queries.MakeStructMapping(redditType)
	redditPrimaryKeyMapping, _ = queries.BindMapping(redditType, redditMapping, redditPrimaryKeyColumns)
	redditInsertCacheMut       sync.RWMutex
	redditInsertCache          = make(map[string]insertCache)
	redditUpdateCacheMut       sync.RWMutex
	redditUpdateCache          = make(map[string]updateCache)
	redditUpsertCacheMut       sync.RWMutex
	redditUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

// One returns a single reddit record from the query.
func (q redditQuery) One(ctx context.Context, exec boil.ContextExecutor) (*Reddit, error) {
	o := &Reddit{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(ctx, exec, o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for reddit")
	}

	return o, nil
}

// All returns all Reddit records from the query.
func (q redditQuery) All(ctx context.Context, exec boil.ContextExecutor) (RedditSlice, error) {
	var o []*Reddit

	err := q.Bind(ctx, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to Reddit slice")
	}

	return o, nil
}

// Count returns the count of all Reddit records in the query.
func (q redditQuery) Count(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count reddit rows")
	}

	return count, nil
}

// Exists checks if the row exists in the table.
func (q redditQuery) Exists(ctx context.Context, exec boil.ContextExecutor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRowContext(ctx, exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if reddit exists")
	}

	return count > 0, nil
}

// Reddits retrieves all the records using an executor.
func Reddits(mods ...qm.QueryMod) redditQuery {
	mods = append(mods, qm.From("\"reddit\""))
	return redditQuery{NewQuery(mods...)}
}

// FindReddit retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindReddit(ctx context.Context, exec boil.ContextExecutor, date time.Time, selectCols ...string) (*Reddit, error) {
	redditObj := &Reddit{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"reddit\" where \"date\"=$1", sel,
	)

	q := queries.Raw(query, date)

	err := q.Bind(ctx, exec, redditObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from reddit")
	}

	return redditObj, nil
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *Reddit) Insert(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no reddit provided for insertion")
	}

	var err error

	nzDefaults := queries.NonZeroDefaultSet(redditColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	redditInsertCacheMut.RLock()
	cache, cached := redditInsertCache[key]
	redditInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			redditAllColumns,
			redditColumnsWithDefault,
			redditColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(redditType, redditMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(redditType, redditMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"reddit\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"reddit\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into reddit")
	}

	if !cached {
		redditInsertCacheMut.Lock()
		redditInsertCache[key] = cache
		redditInsertCacheMut.Unlock()
	}

	return nil
}

// Update uses an executor to update the Reddit.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *Reddit) Update(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) (int64, error) {
	var err error
	key := makeCacheKey(columns, nil)
	redditUpdateCacheMut.RLock()
	cache, cached := redditUpdateCache[key]
	redditUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			redditAllColumns,
			redditPrimaryKeyColumns,
		)

		if len(wl) == 0 {
			return 0, errors.New("models: unable to update reddit, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"reddit\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, redditPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(redditType, redditMapping, append(wl, redditPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	var result sql.Result
	result, err = exec.ExecContext(ctx, cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update reddit row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for reddit")
	}

	if !cached {
		redditUpdateCacheMut.Lock()
		redditUpdateCache[key] = cache
		redditUpdateCacheMut.Unlock()
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values.
func (q redditQuery) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for reddit")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for reddit")
	}

	return rowsAff, nil
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o RedditSlice) UpdateAll(ctx context.Context, exec boil.ContextExecutor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("models: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), redditPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"reddit\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, redditPrimaryKeyColumns, len(o)))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in reddit slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all reddit")
	}
	return rowsAff, nil
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *Reddit) Upsert(ctx context.Context, exec boil.ContextExecutor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no reddit provided for upsert")
	}

	nzDefaults := queries.NonZeroDefaultSet(redditColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	redditUpsertCacheMut.RLock()
	cache, cached := redditUpsertCache[key]
	redditUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := insertColumns.InsertColumnSet(
			redditAllColumns,
			redditColumnsWithDefault,
			redditColumnsWithoutDefault,
			nzDefaults,
		)
		update := updateColumns.UpdateColumnSet(
			redditAllColumns,
			redditPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert reddit, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(redditPrimaryKeyColumns))
			copy(conflict, redditPrimaryKeyColumns)
		}
		cache.query = buildUpsertQueryPostgres(dialect, "\"reddit\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(redditType, redditMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(redditType, redditMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRowContext(ctx, cache.query, vals...).Scan(returns...)
		if err == sql.ErrNoRows {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.ExecContext(ctx, cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert reddit")
	}

	if !cached {
		redditUpsertCacheMut.Lock()
		redditUpsertCache[key] = cache
		redditUpsertCacheMut.Unlock()
	}

	return nil
}

// Delete deletes a single Reddit record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *Reddit) Delete(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no Reddit provided for delete")
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), redditPrimaryKeyMapping)
	sql := "DELETE FROM \"reddit\" WHERE \"date\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from reddit")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for reddit")
	}

	return rowsAff, nil
}

// DeleteAll deletes all matching rows.
func (q redditQuery) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no redditQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.ExecContext(ctx, exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from reddit")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for reddit")
	}

	return rowsAff, nil
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o RedditSlice) DeleteAll(ctx context.Context, exec boil.ContextExecutor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), redditPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"reddit\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, redditPrimaryKeyColumns, len(o))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	result, err := exec.ExecContext(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from reddit slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for reddit")
	}

	return rowsAff, nil
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *Reddit) Reload(ctx context.Context, exec boil.ContextExecutor) error {
	ret, err := FindReddit(ctx, exec, o.Date)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *RedditSlice) ReloadAll(ctx context.Context, exec boil.ContextExecutor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := RedditSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), redditPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"reddit\".* FROM \"reddit\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, redditPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(ctx, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in RedditSlice")
	}

	*o = slice

	return nil
}

// RedditExists checks if the Reddit row exists.
func RedditExists(ctx context.Context, exec boil.ContextExecutor, date time.Time) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"reddit\" where \"date\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, date)
	}

	row := exec.QueryRowContext(ctx, sql, date)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if reddit exists")
	}

	return exists, nil
}
