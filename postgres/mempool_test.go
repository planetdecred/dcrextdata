package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/planetdecred/dcrextdata/cache"
)

var tempDir string

func printJson(thing interface{}) {
	s, _ := json.MarshalIndent(thing, "", "    ")
	fmt.Println(string(s))
}

// TestMain setups the tempDir and cleans it up after tests.
func TestMain(m *testing.M) {
	var err error
	tempDir, err = ioutil.TempDir(os.TempDir(), "cache")
	if err != nil {
		fmt.Printf("ioutil.TempDir: %v", err)
		return
	}

	code := m.Run()

	// clean up
	os.RemoveAll(tempDir)

	os.Exit(code)
}

func TestMempool(t *testing.T) {
	comp := func(k string, a interface{}, b interface{}, expectation bool) {
		v := reflect.DeepEqual(a, b)
		if v != expectation {
			spew.Dump(a, b)
			t.Fatalf("DeepEqual: expected %t, found %t for %s", expectation, v, k)
		}
	}

	errShouldBeNil := func(err error) {
		if err != nil {
			t.Fatalf("expected no err but found: %s", err.Error())
		}
	}
	syncSources := []string{"a", "b"}
	charts := cache.NewChartData(context.Background(), true, syncSources, nil, nil, nil, nil, nil, nil, tempDir)

	t.Run("append_propagation_data", func(t *testing.T) {
		currentSet, err := charts.PropagationSet(cache.DefaultBin)
		if err != nil && err != cache.UnknownChartErr {
			t.Fatalf("expected no error but found: %s", err.Error())

		}

		newRecord := propagationSet{
			height:                    []uint64{1, 2},
			time:                      []uint64{1, 2},
			voteReceiveTimeDeviations: []float64{1, 2},
			blockDelay:                []float64{1, 2},
			blockPropagation:          make(map[string]cache.ChartFloats),
		}
		currentSet.Heights = append(currentSet.Heights, newRecord.height...)
		currentSet.Time = append(currentSet.Time, newRecord.time...)
		currentSet.VoteReceiveTimeDeviations = append(currentSet.VoteReceiveTimeDeviations, newRecord.voteReceiveTimeDeviations...)
		currentSet.BlockDelay = append(currentSet.BlockDelay, newRecord.blockDelay...)
		for _, s := range syncSources {
			newRecord.blockPropagation[s] = append(newRecord.blockPropagation[s], 1, 2)
			currentSet.BlockPropagation[s] = append(currentSet.BlockPropagation[s], 1, 2)
		}

		err = appendBlockPropagationChart(charts, newRecord)
		errShouldBeNil(err)

		updatedSet, err := charts.PropagationSet(cache.DefaultBin)
		errShouldBeNil(err)

		comp("propagation Heights cache is updated", updatedSet.Heights, currentSet.Heights, true)
		comp("propagation Time cache is updated", updatedSet.Time, currentSet.Time, true)
		comp("propagation VoteReceiveTimeDeviations cache is updated", updatedSet.VoteReceiveTimeDeviations,
			currentSet.VoteReceiveTimeDeviations, true)
		comp("propagation BlockDelay cache is updated", updatedSet.BlockDelay, currentSet.BlockDelay, true)
		for _, s := range syncSources {
			comp("propagation BlockPropagation cache is updated", updatedSet.BlockPropagation[s],
				currentSet.BlockPropagation[s], true)
		}
	})

}
