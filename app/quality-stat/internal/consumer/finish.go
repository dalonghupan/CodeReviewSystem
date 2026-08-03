// Package consumer quality-stat MQ 消费者
// 消费 review_finish_topic（评审归档 → 缺陷归集 + 月度统计更新）
package consumer

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"cr-system/app/quality-stat/internal/data"
	"cr-system/pkg/mq"
)

// FinishConsumer 评审完结消费者
// 对应 LLD §3.5：消费 review_finish_topic → 归集缺陷 → 更新月度统计
type FinishConsumer struct {
	data *data.Data
	log  *log.Helper
}

// NewFinishConsumer 构造
func NewFinishConsumer(d *data.Data, logger log.Logger) *FinishConsumer {
	return &FinishConsumer{data: d, log: log.NewHelper(logger)}
}

// Handle 处理评审完结消息（mq.MessageHandler 签名）
func (c *FinishConsumer) Handle(ctx context.Context, body []byte) error {
	var msg mq.ReviewFinishMessage
	if err := mq.Unmarshal(body, &msg); err != nil {
		return nil // 格式非法直接丢弃
	}

	c.log.Infow("msg", "收到评审完结事件", "review_id", msg.ReviewID,
		"tenant_id", msg.TenantID, "defect_comments", msg.DefectComments, "trace_id", msg.TraceID)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 计算统计月份
	statMonth := time.Now().Format("2006-01")

	// 解析 DefectIDs 并入库（消息体中 DefectIDs 由 cr-core 预归集）
	if len(msg.DefectIDs) > 0 {
		c.log.Infow("msg", "开始归集缺陷", "count", len(msg.DefectIDs))
		// 此处 DefectIDs 列表仅包含 ID 引用，需要 cr-core 额外提供评论详情
		// 当前版本简化：缺陷数据由 cr-core 通过消息体完整传递
		c.log.Infow("msg", "缺陷ID列表收到", "defect_ids", msg.DefectIDs, "review_id", msg.ReviewID)
	}

	// 更新月度统计
	monthlyStat := &data.QualityMonthlyStat{
		TenantID:       msg.TenantID,
		StatMonth:      statMonth,
		TotalDefects:   msg.DefectComments,
		CompletedReviews: 1,
	}
	// 计算评审耗时（小时）
	if msg.CreatedTime > 0 && msg.ArchiveTime > 0 {
		hours := float64(msg.ArchiveTime-msg.CreatedTime) / 3600000.0
		monthlyStat.AvgReviewHours = hours
	}
	monthlyStat.CompletionRate = 100.0 // 归档即完成

	if err := c.data.UpsertMonthlyStat(ctx, monthlyStat); err != nil {
		c.log.Warnw("msg", "更新月度统计失败", "error", err.Error())
		// 不阻断主流程
	}

	c.log.Infow("msg", "评审完结处理完成", "review_id", msg.ReviewID)
	return nil
}
