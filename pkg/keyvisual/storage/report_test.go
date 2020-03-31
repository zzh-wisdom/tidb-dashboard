package storage

import (
	"path"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	. "github.com/pingcap/check"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
)

func TestReportManage(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&testReportSuite{})

type testReportSuite struct {
	dir          string
	reportManage *ReportManage
}

func (t *testReportSuite) SetUpTest(c *C) {
	t.dir = c.MkDir()
	gormDB, err := gorm.Open("sqlite3", path.Join(t.dir, "test.sqlite.db"))
	if err != nil {
		c.Errorf("Open %s error: %v", path.Join(t.dir, "test.sqlite.db"), err)
	}
	db := &dbstore.DB{DB: gormDB}
	t.reportManage = NewReportManage(db, time.Now())
}

func (t *testReportSuite) TearDownTest(c *C) {
	_ = t.reportManage.Db.Close()
}

func (t *testReportSuite) TestRestoreReport(c *C) {
	time := t.reportManage.ReportTime
	err := t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime, Equals, time)
	c.Assert(t.reportManage.ReportEndTimes, HasLen, 0)

	err = t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime, Equals, time)
	c.Assert(t.reportManage.ReportEndTimes, HasLen, 0)

	endTime1 := time
	err = t.reportManage.InsertReport(matrix.Matrix{})
	if err != nil {
		c.Fatalf("InsertReport error: %v", err)
	}
	endTime2 := time.Add(reportInterval)
	err = t.reportManage.InsertReport(matrix.Matrix{})
	if err != nil {
		c.Fatalf("InsertReport error: %v", err)
	}
	err = t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime.Unix(), Equals, endTime2.Add(reportInterval).Unix())
	c.Assert(t.reportManage.ReportEndTimes, HasLen, 2)
	c.Assert(t.reportManage.ReportEndTimes[0].Unix(), Equals, endTime1.Unix())
	c.Assert(t.reportManage.ReportEndTimes[1].Unix(), Equals, endTime2.Unix())
}

func (t *testReportSuite) TestIsNeedReport(c *C) {
	nowTime := t.reportManage.ReportTime.Add(time.Second)
	c.Assert(t.reportManage.IsNeedReport(nowTime), Equals, true)
	nowTime = t.reportManage.ReportTime.Add(-time.Second)
	c.Assert(t.reportManage.IsNeedReport(nowTime), Equals, false)
}

func (t *testReportSuite) TestUpdateReportTime(c *C) {
	nowTime := t.reportManage.ReportTime

	nowTime = nowTime.Add(reportInterval)
	t.reportManage.UpdateReportTime(nowTime)
	c.Assert(t.reportManage.ReportTime, Equals, nowTime)

	nowTime = nowTime.Add(reportInterval+time.Second)
	t.reportManage.UpdateReportTime(nowTime)
	c.Assert(t.reportManage.ReportTime, Equals, nowTime)

	nowTime = nowTime.Add(reportInterval-time.Second)
	expectedTime := t.reportManage.ReportTime.Add(reportInterval)
	t.reportManage.UpdateReportTime(nowTime)
	c.Assert(t.reportManage.ReportTime, Equals, expectedTime)
}

func (t *testReportSuite) TestInsertReportAndFindReport(c *C) {
	nowTime := t.reportManage.ReportTime
	err := t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime, Equals, nowTime)
	c.Assert(t.reportManage.ReportEndTimes, HasLen, 0)

	obtainedMatrix, isFind, err := t.reportManage.FindReport(nowTime)
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, false)
	c.Assert(obtainedMatrix, DeepEquals, matrix.Matrix{})

	endTime1 := nowTime
	matrix1 := matrix.Matrix{
		Keys: []string{"a", "b"},
		DataMap: map[string][][]uint64{
			"integration": {
				{100},
			},
		},
		KeyAxis:  nil,
		TimeAxis: []int64{1, 2},
	}
	err = t.reportManage.InsertReport(matrix1)
	c.Assert(err, IsNil)
	endTime2 := nowTime.Add(reportInterval)
	matrix2 := matrix.Matrix{}
	err = t.reportManage.InsertReport(matrix2)
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime, Equals, nowTime.Add(reportInterval*2))
	c.Assert(t.reportManage.ReportEndTimes, HasLen, 2)
	c.Assert(t.reportManage.ReportEndTimes[0], Equals, endTime1)
	c.Assert(t.reportManage.ReportEndTimes[1], Equals, endTime2)

	obtainedMatrix1, isFind, err := t.reportManage.FindReport(endTime1)
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix1, DeepEquals, matrix1)
	obtainedMatrix2, isFind, err := t.reportManage.FindReport(endTime2)
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix2, DeepEquals, matrix2)

	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime1.Add(-time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix, DeepEquals, matrix1)
	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime1.Add(time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix, DeepEquals, matrix2)
	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime2.Add(time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, false)
	c.Assert(obtainedMatrix, DeepEquals, matrix.Matrix{})
}