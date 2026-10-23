package tsarchive

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"netops/internal/core"
	"netops/internal/model"
	"netops/internal/tsdb"
	"netops/internal/tsdb/builtin"
)

func newTestApp(t *testing.T) (*core.App, *gorm.DB) {
	dir := t.TempDir()
	eng, err := builtin.NewBuiltin(filepath.Join(dir, "ts"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "store.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TsArchive{}, &model.TsArchiveRetry{}, &model.SystemSetting{}); err != nil {
		t.Fatal(err)
	}
	return &core.App{TSDB: eng, DB: db}, db
}

func TestArchiveMetric(t *testing.T) {
	app, db := newTestApp(t)
	day := time.Now().AddDate(0, 0, -1)
	base := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	eng := app.TSDB.(*builtin.Engine)
	rows := []tsdb.Row{
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 10, TS: base.Add(time.Hour)},
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 20, TS: base.Add(2 * time.Hour)},
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 30, TS: base.Add(3 * time.Hour)},
		{Metric: "link_rt", Field: "value", Tags: map[string]string{"task_id": "9"}, Value: 5.5, TS: base.Add(time.Hour)},
		// 不同标签组合独立归档
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "2"}, Value: 99, TS: base.Add(time.Hour)},
	}
	if err := eng.Write(rows); err != nil {
		t.Fatal(err)
	}
	start := base
	end := base.Add(24*time.Hour - time.Millisecond)
	dayStr := day.Format("2006-01-02")

	n1, err := archiveMetric(app, "dev_cpu", start, end, dayStr)
	if err != nil {
		t.Fatal(err)
	}
	n2, _ := archiveMetric(app, "link_rt", start, end, dayStr)
	if n1 != 2 || n2 != 1 {
		t.Fatalf("期望 dev_cpu=2 link_rt=1，实际 %d %d", n1, n2)
	}

	var rec model.TsArchive
	if err := db.Where("metric = ? AND tags LIKE ?", "dev_cpu", `%device_id":"1"%`).First(&rec).Error; err != nil {
		t.Fatal(err)
	}
	if rec.AvgVal != 20 || rec.MaxVal != 30 || rec.MinVal != 10 || rec.LastVal != 30 || rec.Count != 3 {
		t.Fatalf("聚合值不对: avg=%v max=%v min=%v last=%v count=%d", rec.AvgVal, rec.MaxVal, rec.MinVal, rec.LastVal, rec.Count)
	}
	if rec.Day != dayStr || rec.Field != "value" {
		t.Fatalf("日期/字段不对: %s %s", rec.Day, rec.Field)
	}

	// 幂等：再跑一次不新增
	n3, _ := archiveMetric(app, "dev_cpu", start, end, dayStr)
	if n3 != 0 {
		t.Fatalf("幂等失败：重复归档新增 %d 条", n3)
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}
	eng.Close()
}

func TestArchiveValidateAndCleanup(t *testing.T) {
	app, db := newTestApp(t)
	day := time.Now().AddDate(0, 0, -1)
	base := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	eng := app.TSDB.(*builtin.Engine)
	// 正常序列 + 乱码标签序列（tags 含替换符）+ NaN 值序列
	rows := []tsdb.Row{
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 10, TS: base.Add(time.Hour)},
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 20, TS: base.Add(2 * time.Hour)},
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "\ufffd\ufffd"}, Value: 99, TS: base.Add(time.Hour)},
		{Metric: "dev_mem", Field: "value", Tags: map[string]string{"device_id": "3"}, Value: math.NaN(), TS: base.Add(time.Hour)},
	}
	if err := eng.Write(rows); err != nil {
		t.Fatal(err)
	}
	start := base
	end := base.Add(24*time.Hour - time.Millisecond)
	dayStr := day.Format("2006-01-02")

	n, err := archiveMetric(app, "dev_cpu", start, end, dayStr)
	if err != nil {
		t.Fatal(err)
	}
	// 乱码序列被跳过：只归档 1 条
	if n != 1 {
		t.Fatalf("期望归档 1 条（乱码跳过），实际 %d", n)
	}
	// dev_cpu 正常序列已转存 → 被清除；乱码序列保留原始数据
	q, qerr := eng.Query(tsdb.Query{Metric: "dev_cpu", Start: start, End: end})
	if qerr != nil {
		t.Fatal(qerr)
	}
	normalLeft := false
	badLeft := false
	for _, s := range q {
		if s.Tags["device_id"] == "1" {
			normalLeft = true
		}
		if !isCleanText(s.Tags["device_id"]) {
			badLeft = true
		}
	}
	if normalLeft {
		t.Fatalf("清除失败：已转存的正常序列仍残留")
	}
	if !badLeft {
		t.Fatalf("乱码序列不应被删除（保留原始数据待核查）")
	}
	// 校验函数单测：NaN/Inf/越界时间/空序列均应拒绝
	badNaN := &tsdb.Series{Metric: "dev_mem", Field: "value", Tags: map[string]string{"device_id": "x"},
		Points: [][2]float64{{float64(start.Add(time.Hour).UnixMilli()), math.NaN()}}}
	if err := validateSeries(badNaN, start, end); err == nil {
		t.Fatal("NaN 值应校验失败")
	}
	badInf := &tsdb.Series{Metric: "dev_mem", Field: "value", Tags: map[string]string{"device_id": "x"},
		Points: [][2]float64{{float64(start.Add(time.Hour).UnixMilli()), math.Inf(1)}}}
	if err := validateSeries(badInf, start, end); err == nil {
		t.Fatal("Inf 值应校验失败")
	}
	badTime := &tsdb.Series{Metric: "dev_mem", Field: "value", Tags: map[string]string{"device_id": "x"},
		Points: [][2]float64{{float64(start.Add(48 * time.Hour).UnixMilli()), 1}}}
	if err := validateSeries(badTime, start, end); err == nil {
		t.Fatal("越界时间应校验失败")
	}
	if err := validateSeries(&tsdb.Series{Metric: "dev_mem", Field: "value", Tags: map[string]string{}, Points: nil}, start, end); err == nil {
		t.Fatal("空序列应校验失败")
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}
	eng.Close()
}

func TestArchiveRetryAndBackfill(t *testing.T) {
	app, db := newTestApp(t)
	day := time.Now().AddDate(0, 0, -1)
	base := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	eng := app.TSDB.(*builtin.Engine)
	start := base
	end := base.Add(24*time.Hour - time.Millisecond)
	dayStr := day.Format("2006-01-02")

	// 1) 先写入数据，归档成功 → 无补传记录
	eng.Write([]tsdb.Row{
		{Metric: "dev_up", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 1, TS: base.Add(time.Hour)},
	})
	if n, _ := archiveMetric(app, "dev_up", start, end, dayStr); n != 1 {
		t.Fatalf("归档应写入 1 条，实际 %d", n)
	}
	var rc int64
	db.Model(&model.TsArchiveRetry{}).Where("metric = ? AND day = ?", "dev_up", dayStr).Count(&rc)
	if rc != 0 {
		t.Fatalf("成功归档不应产生补传记录，实际 %d", rc)
	}

	// 2) 写入乱码数据 → 校验失败 → 生成补传记录（保留时序库原始数据）
	eng.Write([]tsdb.Row{
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "\ufffd"}, Value: 5, TS: base.Add(time.Hour)},
	})
	if n, _ := archiveMetric(app, "dev_cpu", start, end, dayStr); n != 0 {
		t.Fatalf("乱码数据不应归档，实际 %d", n)
	}
	db.Model(&model.TsArchiveRetry{}).Where("metric = ? AND day = ?", "dev_cpu", dayStr).Count(&rc)
	if rc != 1 {
		t.Fatalf("校验失败应生成 1 条补传记录，实际 %d", rc)
	}

	// 3) 人工清理时序库中的乱码数据后写入正常数据 → 补传成功 → 记录被清除
	eng.Delete(tsdb.Query{Metric: "dev_cpu", Tags: map[string]string{"device_id": "\ufffd"}, Start: start, End: end})
	eng.Write([]tsdb.Row{
		{Metric: "dev_cpu", Field: "value", Tags: map[string]string{"device_id": "1"}, Value: 5, TS: base.Add(time.Hour)},
	})
	retryFailed(app, time.Now())
	db.Model(&model.TsArchiveRetry{}).Where("metric = ? AND day = ?", "dev_cpu", dayStr).Count(&rc)
	if rc != 0 {
		t.Fatalf("补传成功后记录应清除，实际 %d", rc)
	}

	// 4) 先归档登记 pending，再重复补传 → 重试递增 → 达到上限标记 discard
	eng.Write([]tsdb.Row{
		{Metric: "dev_mem", Field: "value", Tags: map[string]string{"device_id": "\ufffd"}, Value: 1, TS: base.Add(time.Hour)},
	})
	archiveMetric(app, "dev_mem", start, end, dayStr)
	for i := 0; i < retryMax+1; i++ {
		retryFailed(app, time.Now())
		var probe model.TsArchiveRetry
		db.Where("metric = ? AND day = ?", "dev_mem", dayStr).First(&probe)
		t.Logf("iter=%d probe: status=%s retries=%d", i, probe.Status, probe.Retries)
	}
	var item model.TsArchiveRetry
	if err := db.Where("metric = ? AND day = ?", "dev_mem", dayStr).First(&item).Error; err != nil {
		t.Fatalf("应有 discard 记录，实际无: %v", err)
	}
	if item.Status != "discard" {
		t.Fatalf("超过上限应标记 discard，实际 %s(retries=%d)", item.Status, item.Retries)
	}
	q, _ := eng.Query(tsdb.Query{Metric: "dev_mem", Start: start, End: end})
	if len(q) == 0 {
		t.Fatalf("discard 后时序库原始数据应保留")
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}
	eng.Close()
}
