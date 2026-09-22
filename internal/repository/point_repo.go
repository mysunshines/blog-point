package repository

import (
	"context"
	"time"

	"github.com/mysunshines/blog-point/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PointRepository 积分域数据访问
type PointRepository struct {
	db *gorm.DB
}

// NewPointRepository 构造积分仓储
func NewPointRepository(db *gorm.DB) *PointRepository {
	return &PointRepository{db: db}
}

// ============================================================================
// 账户
// ============================================================================

// GetPoint 获取账户；不存在时返回 nil（由调用方决定是否初始化）
func (r *PointRepository) GetPoint(ctx context.Context, userID uint) (*model.UserPoint, error) {
	var p model.UserPoint
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// EnsurePoint 获取账户；不存在则创建（首次获得积分时惰性开户）
func (r *PointRepository) EnsurePoint(ctx context.Context, userID uint) (*model.UserPoint, error) {
	p, err := r.GetPoint(ctx, userID)
	if err == nil {
		return p, nil
	}
	// 并发下可能重复插入，用 Upsert 保证幂等
	newP := &model.UserPoint{UserID: userID, Balance: 0}
	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: true,
	}).Create(newP).Error
	if err != nil {
		return nil, err
	}
	return r.GetPoint(ctx, userID)
}

// ListAllPoints 列出全部积分账户（用于启动时全量重建积分榜：Redis 重启/数据丢失后自愈）
func (r *PointRepository) ListAllPoints(ctx context.Context) ([]*model.UserPoint, error) {
	var points []*model.UserPoint
	err := r.db.WithContext(ctx).Find(&points).Error
	return points, err
}

// ============================================================================
// 流水
// ============================================================================

// CreateLog 记录流水（追加写）
func (r *PointRepository) CreateLog(ctx context.Context, tx *gorm.DB, log *model.PointLog) error {
	return tx.WithContext(ctx).Create(log).Error
}

// ListLogs 分页查询用户流水
func (r *PointRepository) ListLogs(ctx context.Context, userID uint, page, size int) ([]*model.PointLog, int64, error) {
	var logs []*model.PointLog
	var total int64
	q := r.db.WithContext(ctx).Model(&model.PointLog{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	err := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs).Error
	return logs, total, err
}

// CountLogsByRuleSince 统计某规则在 since 之后的发放次数（用于限次防刷）
func (r *PointRepository) CountLogsByRuleSince(ctx context.Context, userID uint, ruleCode string, since time.Time) (int64, error) {
	var cnt int64
	err := r.db.WithContext(ctx).Model(&model.PointLog{}).
		Where("user_id = ? AND rule_code = ? AND created_at >= ?", userID, ruleCode, since).
		Count(&cnt).Error
	return cnt, err
}

// ============================================================================
// 规则
// ============================================================================

// ListEnabledRulesByEvent 取某事件下启用的规则（按 sort 升序）
func (r *PointRepository) ListEnabledRulesByEvent(ctx context.Context, eventType string) ([]*model.PointRule, error) {
	var rules []*model.PointRule
	err := r.db.WithContext(ctx).
		Where("event_type = ? AND status = ?", eventType, model.RuleStatusEnabled).
		Order("sort ASC, id ASC").Find(&rules).Error
	return rules, err
}

// ListRules 全量规则（后台管理）
func (r *PointRepository) ListRules(ctx context.Context) ([]*model.PointRule, error) {
	var rules []*model.PointRule
	err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&rules).Error
	return rules, err
}

// GetRuleByID 按 ID 取规则
func (r *PointRepository) GetRuleByID(ctx context.Context, id uint) (*model.PointRule, error) {
	var rule model.PointRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateRule 新增规则
func (r *PointRepository) CreateRule(ctx context.Context, rule *model.PointRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

// UpdateRule 更新规则（只更新非零值字段）
// 注意：status=0（停用）属零值，用本方法不会生效；需显式置零请用 UpdateRuleFields。
func (r *PointRepository) UpdateRule(ctx context.Context, rule *model.PointRule) error {
	return r.db.WithContext(ctx).Model(rule).Omit("created_at").Updates(rule).Error
}

// UpdateRuleFields 按字段名显式更新规则（可写入零值，如 status=0 停用）
func (r *PointRepository) UpdateRuleFields(ctx context.Context, id uint, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.PointRule{}).
		Where("id = ?", id).Updates(updates).Error
}

// DeleteRule 删除规则
func (r *PointRepository) DeleteRule(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.PointRule{}, id).Error
}

// ============================================================================
// 签到
// ============================================================================

// GetCheckinByDate 查询某用户某天的签到记录
func (r *PointRepository) GetCheckinByDate(ctx context.Context, userID uint, date string) (*model.UserCheckin, error) {
	var c model.UserCheckin
	err := r.db.WithContext(ctx).Where("user_id = ? AND checkin_date = ?", userID, date).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// LatestCheckin 取最近一次签到（用于计算连续天数）
func (r *PointRepository) LatestCheckin(ctx context.Context, userID uint) (*model.UserCheckin, error) {
	var c model.UserCheckin
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("checkin_date DESC").First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCheckin 写入签到记录（唯一索引保证一天一次）
func (r *PointRepository) CreateCheckin(ctx context.Context, c *model.UserCheckin) error {
	return r.db.WithContext(ctx).Create(c).Error
}

// ============================================================================
// 已购
// ============================================================================

// HasPurchase 判断是否已购买（唯一索引保证不会重复购买）
func (r *PointRepository) HasPurchase(ctx context.Context, userID uint, itemType string, itemID uint) (bool, error) {
	var cnt int64
	err := r.db.WithContext(ctx).Model(&model.UserPurchase{}).
		Where("user_id = ? AND item_type = ? AND item_id = ?", userID, itemType, itemID).
		Count(&cnt).Error
	return cnt > 0, err
}

// CreatePurchase 记录购买（重复时忽略，保证幂等）
func (r *PointRepository) CreatePurchase(ctx context.Context, tx *gorm.DB, p *model.UserPurchase) error {
	return tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "item_type"}, {Name: "item_id"}},
		DoNothing: true,
	}).Create(p).Error
}
