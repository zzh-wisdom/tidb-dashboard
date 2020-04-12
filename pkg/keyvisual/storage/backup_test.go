package storage

//
//import (
//	"path"
//	"testing"
//	"time"
//
//	"github.com/jinzhu/gorm"
//	. "github.com/pingcap/check"
//
//	"github.com/pingcap-incubator/tidb-dashboard/pkg/dbstore"
//	"github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/matrix"
//)
//
//func TestDbstore(t *testing.T) {
//	TestingT(t)
//}
//
//var _ = Suite(&testDbstoreSuite{})
//
//type testDbstoreSuite struct {
//	dir string
//	db  *dbstore.DB
//}
//
//func (t *testDbstoreSuite) SetUpTest(c *C) {
//	t.dir = c.MkDir()
//	gormDB, err := gorm.Open("sqlite3", path.Join(t.dir, "test.sqlite.db"))
//	if err != nil {
//		c.Errorf("Open %s error: %v", path.Join(t.dir, "test.sqlite.db"), err)
//	}
//	t.db = &dbstore.DB{DB: gormDB}
//}
//
//func (t *testDbstoreSuite) TearDownTest(c *C) {
//	t.db.Close()
//}
//
//func (t *testDbstoreSuite) TestCreateTableIfNotExists(c *C) {
//	isExist, err := CreateTableIfNotExists(t.db, &Plane{})
//	c.Assert(isExist, Equals, false)
//	c.Assert(err, IsNil)
//	isExist, err = CreateTableIfNotExists(t.db, &Plane{})
//	c.Assert(isExist, Equals, true)
//	c.Assert(err, IsNil)
//}
//
//func (t *testDbstoreSuite) TestClearTable(c *C) {
//	_, err := CreateTableIfNotExists(t.db, &Plane{})
//	if err != nil {
//		c.Fatalf("Create table Plane error: %v", err)
//	}
//	err = InsertPlane(t.db, 0, time.Now(), matrix.Axis{})
//	if err != nil {
//		c.Fatalf("InsertPlane error: %v", err)
//	}
//	var count int
//	err = t.db.Table(tablePlaneName).Count(&count).Error
//	if err != nil {
//		c.Fatalf("Count table Plane error: %v", err)
//	}
//	c.Assert(count, Equals, 1)
//
//	err = ClearTable(t.db, &Plane{})
//	c.Assert(err, IsNil)
//
//	err = t.db.Table(tablePlaneName).Count(&count).Error
//	if err != nil {
//		c.Fatalf("Count table Plane error: %v", err)
//	}
//	c.Assert(count, Equals, 0)
//}
//
//func (t *testDbstoreSuite) TestInsertPlane(c *C) {
//	_, err := CreateTableIfNotExists(t.db, &Plane{})
//	if err != nil {
//		c.Fatalf("Create table Plane error: %v", err)
//	}
//	var num uint8 = 0
//	time := time.Now()
//	axis := matrix.Axis{}
//	err = InsertPlane(t.db, 0, time, axis)
//	if err != nil {
//		c.Fatalf("InsertPlane error: %v", err)
//	}
//	planes, err := FindPlaneOrderByTime(t.db, num)
//	if err != nil {
//		c.Fatalf("FindPlaneOrderByTime error: %v", err)
//	}
//	c.Assert(len(planes), Equals, 1)
//
//	c.Assert(planes[0].LayerNum, Equals, num)
//	c.Assert(planes[0].Time.Unix(), Equals, time.Unix())
//	obtainedAxis, err := planes[0].UnmarshalAxis()
//	if err != nil {
//		c.Fatalf("UnmarshalAxis error: %v", err)
//	}
//	c.Assert(obtainedAxis, DeepEquals, axis)
//}
//
//func (t *testDbstoreSuite) TestDeletePlane(c *C) {
//	_, err := CreateTableIfNotExists(t.db, &Plane{})
//	if err != nil {
//		c.Fatalf("Create table Plane error: %v", err)
//	}
//	var num uint8 = 0
//	time := time.Now()
//	axis := matrix.Axis{}
//	err = InsertPlane(t.db, 0, time, axis)
//	if err != nil {
//		c.Fatalf("InsertPlane error: %v", err)
//	}
//
//	var count int
//	err = t.db.Table(tablePlaneName).Count(&count).Error
//	if err != nil {
//		c.Fatalf("Count table Plane error: %v", err)
//	}
//	c.Assert(count, Equals, 1)
//
//	err = DeletePlane(t.db, num, time)
//	c.Assert(err, IsNil)
//
//	err = t.db.Table(tablePlaneName).Count(&count).Error
//	if err != nil {
//		c.Fatalf("Count table Plane error: %v", err)
//	}
//	c.Assert(count, Equals, 0)
//}
//
//func (t *testDbstoreSuite) TestFindPlaneOrderByTime(c *C) {
//	_, err := CreateTableIfNotExists(t.db, &Plane{})
//	if err != nil {
//		c.Fatalf("Create table Plane error: %v", err)
//	}
//
//	var num1 uint8 = 0
//	time1 := time.Now()
//	axis1 := matrix.Axis{}
//	err = InsertPlane(t.db, num1, time1, axis1)
//	if err != nil {
//		c.Fatalf("InsertPlane error: %v", err)
//	}
//	var num2 uint8 = 1
//	time2 := time.Now()
//	axis2 := matrix.Axis{
//		Keys:       []string{"a", "b"},
//		ValuesList: [][]uint64{{4}, {3}, {2}, {1}},
//	}
//	err = InsertPlane(t.db, num2, time2, axis2)
//	if err != nil {
//		c.Fatalf("InsertPlane error: %v", err)
//	}
//
//	planes, err := FindPlaneOrderByTime(t.db, num1)
//	c.Assert(err, IsNil)
//	c.Assert(len(planes), Equals, 1)
//	obtainedAxis1, err := planes[0].UnmarshalAxis()
//	if err != nil {
//		c.Fatalf("UnmarshalAxis error: %v", err)
//	}
//	c.Assert(planes[0].LayerNum, Equals, num1)
//	c.Assert(planes[0].Time.Unix(), Equals, time1.Unix())
//	c.Assert(obtainedAxis1, DeepEquals, axis1)
//
//	planes, err = FindPlaneOrderByTime(t.db, num2)
//	c.Assert(err, IsNil)
//	c.Assert(len(planes), Equals, 1)
//	obtainedAxis2, err := planes[0].UnmarshalAxis()
//	if err != nil {
//		c.Fatalf("UnmarshalAxis error: %v", err)
//	}
//	c.Assert(planes[0].LayerNum, Equals, num2)
//	c.Assert(planes[0].Time.Unix(), Equals, time2.Unix())
//	c.Assert(obtainedAxis2, DeepEquals, axis2)
//
//	err = InsertPlane(t.db, num2, time2.Add(time.Second), axis2)
//	if err != nil {
//		c.Fatalf("InsertPlane error: %v", err)
//	}
//	planes, err = FindPlaneOrderByTime(t.db, num2)
//	c.Assert(err, IsNil)
//	c.Assert(len(planes), Equals, 2)
//	c.Assert(planes[1].Time.Unix(), Equals, planes[0].Time.Add(time.Second).Unix())
//}