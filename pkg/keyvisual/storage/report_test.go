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

var testReportConfig = ReportConfig{
	ReportInterval:    time.Minute * 10,
	ReportTimeRange:   time.Minute * 10,
	ReportMaxDisplayY: 1536,
	MaxReportNum:      2,
}

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
	t.reportManage = NewReportManage(db, time.Now(), testReportConfig)
}

func (t *testReportSuite) TearDownTest(c *C) {
	_ = t.reportManage.Db.Close()
}

func (t *testReportSuite) TestReportConfigCheck(c *C) {
	cfg1 := testReportConfig
	cfg1.ReportMaxDisplayY = 0
	msg := cfg1.Check()
	c.Assert(msg, Not(HasLen), 0)
	cfg1.ReportMaxDisplayY = -1
	msg = cfg1.Check()
	c.Assert(msg, Not(HasLen), 0)
	cfg1.ReportMaxDisplayY = 1
	msg = cfg1.Check()
	c.Assert(msg, HasLen, 0)

	cfg2 := testReportConfig
	cfg2.MaxReportNum = 0
	msg = cfg2.Check()
	c.Assert(msg, Not(HasLen), 0)
	cfg2.MaxReportNum = -1
	msg = cfg2.Check()
	c.Assert(msg, Not(HasLen), 0)
	cfg2.MaxReportNum = 1
	msg = cfg2.Check()
	c.Assert(msg, HasLen, 0)
}

func (t *testReportSuite) TestRestoreReport(c *C) {
	reportManage := *t.reportManage
	err := t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(*t.reportManage, DeepEquals, reportManage)

	err = t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(*t.reportManage, DeepEquals, reportManage)

	endTime1 := t.reportManage.ReportTime
	err = t.reportManage.InsertReport(matrix.Matrix{})
	if err != nil {
		c.Fatalf("InsertReport error: %v", err)
	}
	endTime2 := endTime1.Add(t.reportManage.ReportInterval)
	err = t.reportManage.InsertReport(matrix.Matrix{})
	if err != nil {
		c.Fatalf("InsertReport error: %v", err)
	}
	err = t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime.Unix(), Equals, endTime2.Add(t.reportManage.ReportInterval).Unix())
	c.Assert(t.reportManage.Head, Equals, 0)
	c.Assert(t.reportManage.Tail, Equals, 0)
	c.Assert(t.reportManage.Empty, Equals, false)
	c.Assert(t.reportManage.ReportEndTimes[0].Unix(), Equals, endTime1.Unix())
	c.Assert(t.reportManage.ReportEndTimes[1].Unix(), Equals, endTime2.Unix())

	startTime3 := endTime2
	endTime3 := endTime2.Add(t.reportManage.ReportInterval)
	report, err := NewReport(startTime3, endTime3, matrix.Matrix{})
	c.Assert(err, IsNil)
	err = t.reportManage.Db.Table(tableReportName).Create(report).Error
	c.Assert(err, IsNil)

	err = t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime.Unix(), Equals, endTime3.Add(t.reportManage.ReportInterval).Unix())
	c.Assert(t.reportManage.Head, Equals, 0)
	c.Assert(t.reportManage.Tail, Equals, 0)
	c.Assert(t.reportManage.Empty, Equals, false)
	c.Assert(t.reportManage.ReportEndTimes[0].Unix(), Equals, endTime2.Unix())
	c.Assert(t.reportManage.ReportEndTimes[1].Unix(), Equals, endTime3.Unix())
}

func (t *testReportSuite) TestIsNeedReport(c *C) {
	nowTime := t.reportManage.ReportTime.Add(time.Second)
	c.Assert(t.reportManage.IsNeedReport(nowTime), Equals, true)
	nowTime = t.reportManage.ReportTime.Add(-time.Second)
	c.Assert(t.reportManage.IsNeedReport(nowTime), Equals, false)
}

func (t *testReportSuite) TestInsertReportAndFindReport(c *C) {
	reportManage := *t.reportManage
	err := t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(*t.reportManage, DeepEquals, reportManage)

	nowTime := t.reportManage.ReportTime
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
	c.Assert(t.reportManage.ReportTime, Equals, nowTime.Add(t.reportManage.ReportInterval))
	c.Assert(t.reportManage.Head, Equals, 0)
	c.Assert(t.reportManage.Tail, Equals, 1)
	c.Assert(t.reportManage.Empty, Equals, false)
	c.Assert(t.reportManage.ReportEndTimes[0], Equals, endTime1)

	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime1.Add(-time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix, DeepEquals, matrix1)
	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime1.Add(time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, false)
	c.Assert(obtainedMatrix, DeepEquals, matrix.Matrix{})

	endTime2 := endTime1.Add(t.reportManage.ReportInterval)
	matrix2 := matrix.Matrix{}
	err = t.reportManage.InsertReport(matrix2)
	c.Assert(err, IsNil)
	endTime3 := endTime2.Add(t.reportManage.ReportInterval)
	matrix3 := matrix1
	err = t.reportManage.InsertReport(matrix3)
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.ReportTime, Equals, nowTime.Add(t.reportManage.ReportInterval*3))
	c.Assert(t.reportManage.Head, Equals, 1)
	c.Assert(t.reportManage.Tail, Equals, 1)
	c.Assert(t.reportManage.Empty, Equals, false)
	c.Assert(t.reportManage.ReportEndTimes[1], Equals, endTime2)
	c.Assert(t.reportManage.ReportEndTimes[0], Equals, endTime3)

	obtainedMatrix2, isFind, err := t.reportManage.FindReport(endTime2)
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix2, DeepEquals, matrix2)
	obtainedMatrix3, isFind, err := t.reportManage.FindReport(endTime3)
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix3, DeepEquals, matrix3)

	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime2.Add(-time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix, DeepEquals, matrix2)
	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime2.Add(time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, true)
	c.Assert(obtainedMatrix, DeepEquals, matrix3)
	obtainedMatrix, isFind, err = t.reportManage.FindReport(endTime3.Add(time.Second))
	c.Assert(err, IsNil)
	c.Assert(isFind, Equals, false)
	c.Assert(obtainedMatrix, DeepEquals, matrix.Matrix{})
}

func (t *testReportSuite) TestDeleteReport(c *C) {
	reportManage := *t.reportManage
	err := t.reportManage.RestoreReport()
	c.Assert(err, IsNil)
	c.Assert(*t.reportManage, DeepEquals, reportManage)

	err = t.reportManage.DeleteReport()
	c.Assert(err, IsNil)
	c.Assert(*t.reportManage, DeepEquals, reportManage)

	err = t.reportManage.InsertReport(matrix.Matrix{})
	c.Assert(err, IsNil)
	err = t.reportManage.InsertReport(matrix.Matrix{})
	c.Assert(err, IsNil)

	err = t.reportManage.DeleteReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.Head, Equals, 1)
	c.Assert(t.reportManage.Tail, Equals, 0)
	c.Assert(t.reportManage.Empty, Equals, false)

	err = t.reportManage.DeleteReport()
	c.Assert(err, IsNil)
	c.Assert(t.reportManage.Head, Equals, 0)
	c.Assert(t.reportManage.Tail, Equals, 0)
	c.Assert(t.reportManage.Empty, Equals, true)
}