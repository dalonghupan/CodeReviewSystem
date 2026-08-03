package bizadapter

import (
	"fmt"

	"github.com/xuri/excelize/v2"

	"cr-system/app/quality-stat/internal/data"
)

// ExcelRenderer Excel 报表渲染器
type ExcelRenderer struct{}

// NewExcelRenderer 创建 Excel 渲染器
func NewExcelRenderer() *ExcelRenderer {
	return &ExcelRenderer{}
}

// RenderMonthlyReport 渲染月报 Excel
// 包含：综合指标、模块缺陷分布、用户统计、趋势数据
func (r *ExcelRenderer) RenderMonthlyReport(metrics *data.QualityMetrics, moduleStats []data.ModuleStatItem,
	userStats []data.UserStatItem, reviewTrend, defectTrend []data.TrendPoint) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// ===== Sheet1: 综合指标 =====
	idx, err := f.NewSheet("综合指标")
	if err != nil {
	    return nil, fmt.Errorf("创建Sheet失败: %w", err)
	}
	f.SetActiveSheet(idx)

	// 标题
	f.SetCellValue("综合指标", "A1", "质量月报 - 综合指标")
	f.MergeCell("综合指标", "A1", "E1")

	headers := []string{"指标", "数值", "单位"}
	for i, h := range headers {
		cell := fmt.Sprintf("%c2", 'A'+i)
		f.SetCellValue("综合指标", cell, h)
	}

	metricsData := [][]interface{}{
		{"评审单总数", metrics.TotalReviews, "个"},
		{"已完成评审", metrics.CompletedReviews, "个"},
		{"评审完成率", fmt.Sprintf("%.1f", metrics.ReviewCompletionRate), "%"},
		{"缺陷总数", metrics.TotalDefects, "个"},
		{"已修复缺陷", metrics.FixedDefects, "个"},
		{"缺陷修复及时率", fmt.Sprintf("%.1f", metrics.DefectFixTimelyRate), "%"},
		{"平均修复时长", fmt.Sprintf("%.1f", metrics.AvgFixHours), "小时"},
		{"致命缺陷", metrics.FatalDefects, "个"},
		{"严重缺陷", metrics.CriticalDefects, "个"},
		{"一般缺陷", metrics.MajorDefects, "个"},
		{"优化建议", metrics.MinorDefects, "个"},
	}
	for i, row := range metricsData {
		rowNum := i + 3
		for j, val := range row {
			cell := fmt.Sprintf("%c%d", 'A'+j, rowNum)
			f.SetCellValue("综合指标", cell, val)
		}
	}

	// ===== Sheet2: 模块缺陷分布 =====
	if len(moduleStats) > 0 {
		_, err := f.NewSheet("模块缺陷分布")
		if err != nil {
		    return nil, fmt.Errorf("创建Sheet失败: %w", err)
		}
		modHeaders := []string{"模块名称", "缺陷数", "占比(%)", "最高缺陷等级"}
		for i, h := range modHeaders {
			cell := fmt.Sprintf("%c1", 'A'+i)
			f.SetCellValue("模块缺陷分布", cell, h)
		}
		for i, item := range moduleStats {
			rowNum := i + 2
			f.SetCellValue("模块缺陷分布", fmt.Sprintf("A%d", rowNum), item.ModuleName)
			f.SetCellValue("模块缺陷分布", fmt.Sprintf("B%d", rowNum), item.DefectCount)
			f.SetCellValue("模块缺陷分布", fmt.Sprintf("C%d", rowNum), fmt.Sprintf("%.1f", item.Percentage))
			severity := "一般"
			switch item.MaxSeverity {
			case data.DefectFatal:
				severity = "致命"
			case data.DefectCritical:
				severity = "严重"
			case data.DefectMajor:
				severity = "一般"
			case data.DefectMinor:
				severity = "建议"
			}
			f.SetCellValue("模块缺陷分布", fmt.Sprintf("D%d", rowNum), severity)
		}
	}

	// ===== Sheet3: 评审趋势 =====
	if len(reviewTrend) > 0 || len(defectTrend) > 0 {
		_, err := f.NewSheet("趋势数据")
		if err != nil {
		    return nil, fmt.Errorf("创建Sheet失败: %w", err)
		}
		f.SetCellValue("趋势数据", "A1", "日期")
		f.SetCellValue("趋势数据", "B1", "评审单数")
		f.SetCellValue("趋势数据", "C1", "缺陷数")

		maxLen := len(reviewTrend)
		if len(defectTrend) > maxLen {
			maxLen = len(defectTrend)
		}
		for i := 0; i < maxLen; i++ {
			rowNum := i + 2
			if i < len(reviewTrend) {
				f.SetCellValue("趋势数据", fmt.Sprintf("A%d", rowNum), reviewTrend[i].Date)
				f.SetCellValue("趋势数据", fmt.Sprintf("B%d", rowNum), reviewTrend[i].Count)
			}
			if i < len(defectTrend) {
				f.SetCellValue("趋势数据", fmt.Sprintf("C%d", rowNum), defectTrend[i].Count)
			}
		}
	}

	// ===== Sheet4: 用户统计 =====
	if len(userStats) > 0 {
		_, err := f.NewSheet("用户统计")
		if err != nil {
		    return nil, fmt.Errorf("创建Sheet失败: %w", err)
		}
		userHeaders := []string{"用户ID", "评审数", "完成率(%)", "缺陷数", "修复数"}
		for i, h := range userHeaders {
			cell := fmt.Sprintf("%c1", 'A'+i)
			f.SetCellValue("用户统计", cell, h)
		}
		for i, item := range userStats {
			rowNum := i + 2
			f.SetCellValue("用户统计", fmt.Sprintf("A%d", rowNum), item.UserID)
			f.SetCellValue("用户统计", fmt.Sprintf("B%d", rowNum), item.ReviewCount)
			f.SetCellValue("用户统计", fmt.Sprintf("C%d", rowNum), fmt.Sprintf("%.1f", item.CompletionRate))
			f.SetCellValue("用户统计", fmt.Sprintf("D%d", rowNum), item.DefectCount)
			f.SetCellValue("用户统计", fmt.Sprintf("E%d", rowNum), item.FixedCount)
		}
	}

	// 写入缓冲区
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("生成Excel失败: %w", err)
	}
	return buf.Bytes(), nil
}

// RenderDefectList 渲染缺陷清单 Excel
func (r *ExcelRenderer) RenderDefectList(defects []*data.DefectRecord) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	headers := []string{"缺陷ID", "评审单ID", "文件路径", "行号", "缺陷等级", "描述", "模块", "标记人", "状态", "修复人", "修复时间", "统计月份"}
	for i, h := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		f.SetCellValue("Sheet1", cell, h)
	}

	for i, d := range defects {
		rowNum := i + 2
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", rowNum), d.DefectID)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", rowNum), d.ReviewID)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", rowNum), d.FilePath)
		f.SetCellValue("Sheet1", fmt.Sprintf("D%d", rowNum), d.LineNum)

		level := ""
		switch d.DefectLevel {
		case data.DefectFatal:
			level = "致命(P0)"
		case data.DefectCritical:
			level = "严重(P1)"
		case data.DefectMajor:
			level = "一般(P2)"
		case data.DefectMinor:
			level = "建议(P3)"
		}
		f.SetCellValue("Sheet1", fmt.Sprintf("E%d", rowNum), level)
		f.SetCellValue("Sheet1", fmt.Sprintf("F%d", rowNum), d.Content)
		f.SetCellValue("Sheet1", fmt.Sprintf("G%d", rowNum), d.ModuleName)
		f.SetCellValue("Sheet1", fmt.Sprintf("H%d", rowNum), d.CreatorUID)

		status := "未修复"
		if d.IsFixed {
			status = "已修复"
		}
		f.SetCellValue("Sheet1", fmt.Sprintf("I%d", rowNum), status)
		f.SetCellValue("Sheet1", fmt.Sprintf("J%d", rowNum), d.FixedByUID)
		if d.FixTime != nil {
			f.SetCellValue("Sheet1", fmt.Sprintf("K%d", rowNum), d.FixTime.Format("2006-01-02 15:04"))
		}
		f.SetCellValue("Sheet1", fmt.Sprintf("L%d", rowNum), d.StatMonth)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("生成缺陷清单Excel失败: %w", err)
	}
	return buf.Bytes(), nil
}
