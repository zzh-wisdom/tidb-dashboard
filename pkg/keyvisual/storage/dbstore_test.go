package storage

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	. "github.com/pingcap/check"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

func TestDbstore(t *testing.T) {
	//len := 3
	//ringTimes := make([]time.Time, len)
	//ringAxes := make([]matrix.Axis, len)
	//
	//startTime := time.Now()
	//for i := 0; i < len; i++ {
	//	ringTimes[i] = startTime.Add( time.Minute)
	//	ringAxes[i] = matrix.Axis{
	//		Keys:[]string{"a", "b", "c"},
	//		ValuesList:[][]uint64 {
	//			{1, 1},
	//			{2, 2},
	//			{1, 1},
	//			{1, 1},
	//		},
	//	}
	//}
	//ringData := RingData{
	//	RingAxes:  ringAxes,
	//	RingTimes: ringTimes,
	//}
	axis := matrix.Axis{
		Keys: []string{"\ucdcd\u1243\u2364", "b", "c"},
		ValuesList: [][]uint64{
			{1, 1},
			{2, 2},
			{1, 1},
			{1, 1},
		},
	}
	bytes, err := json.Marshal(axis)
	fmt.Println(string(bytes))
	fmt.Println(err)

	var axis2 matrix.Axis
	err = json.Unmarshal(bytes, &axis2)
	fmt.Println(err)
	fmt.Println(axis2)

	fmt.Println(reflect.DeepEqual(axis, axis2))

	emptyAxis := matrix.Axis{}
	bytes, err = json.Marshal(emptyAxis)
	fmt.Println(string(bytes))
	fmt.Println(err)

	TestingT(t)
}
