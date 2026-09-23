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

// CreatePurchase 记录购买（重复时忽略，保证幂等）。返回受影响行数：
// RowsAffected==1 表示新购买（需扣费）；==0 表示已购买（重复，不扣费）。
func (r *PointRepository) CreatePurchase(ctx context.Context, tx *gorm.DB, p *model.UserPurchase) (int64, error) {
	res := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "item_type"}, {Name: "item_id"}},
		DoNothing: true,
	}).Create(p)
	return res.RowsAffected, res.Error
}

// ============================================================================
// 积分批次（point_grants）：1 年过期 + FIFO 消费
// ============================================================================

// CreateGrant 插入一条积分批次（一个事务内调用，随加分/消费一起提交或回滚）
func (r *PointRepository) CreateGrant(ctx context.Context, tx *gorm.DB, g *model.PointGrant) error {
	return tx.WithContext(ctx).Create(g).Error
}

// ListUnexpiredGrants 取用户未过期且有余额的批次（按 earned_at ASC → FIFO 先扣早期）。
// forUpdate=true 时在事务内加行锁，防止并发消费超卖。
func (r *PointRepository) ListUnexpiredGrants(ctx context.Context, tx *gorm.DB, userID uint, now time.Time, forUpdate bool) ([]*model.PointGrant, error) {
	q := tx.WithContext(ctx).
		Where("user_id = ? AND remaining > 0 AND expires_at > ?", userID, now).
		Order("earned_at ASC")
	if forUpdate {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var grants []*model.PointGrant
	err := q.Find(&grants).Error
	return grants, err
}

// UpdateGrantRemaining 更新批次剩余量（FIFO 扣减）
func (r *PointRepository) UpdateGrantRemaining(ctx context.Context, tx *gorm.DB, id uint, remaining int64) error {
	return tx.WithContext(ctx).Model(&model.PointGrant{}).Where("id = ?", id).Update("remaining", remaining).Error
}

// ExpireGrants 每日任务：把已过期的批次清零，并从账户余额扣减对应量。
// 在事务内对过期批次加行锁，与消费（同样锁批次行）串行，避免重复扣减余额。
// 返回被清零的批次数。
func (r *PointRepository) ExpireGrants(ctx context.Context, now time.Time) (int64, error) {
	var expired int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var grants []*model.PointGrant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("expires_at <= ? AND remaining > 0 AND consumed = ?", now, false).
			Find(&grants).Error; err != nil {
			return err
		}
		if len(grants) == 0 {
			return nil
		}
		byUser := make(map[uint]int64)
		ids := make([]uint, 0, len(grants))
		for _, g := range grants {
			byUser[g.UserID] += g.Remaining
			ids = append(ids, g.ID)
		}
		for uid, sum := range byUser {
			if err := tx.Model(&model.UserPoint{}).Where("user_id = ?", uid).
				Update("balance", gorm.Expr("balance - ?", sum)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.PointGrant{}).Where("id IN ?", ids).
			Updates(map[string]interface{}{"remaining": 0, "consumed": true}).Error; err != nil {
			return err
		}
		expired = int64(len(grants))
		return nil
	})
	return expired, err
}

// BackfillLegacyGrants 存量迁移：为每个「有余额但无批次」的用户生成一条 legacy 批次
// （remaining=balance，过期时间=现在+1 年），使 FIFO/过期逻辑对存量积分生效。
// 返回生成的批次数。
func (r *PointRepository) BackfillLegacyGrants(ctx context.Context) (int64, error) {
	res := r.db.WithContext(ctx).Exec(`
		INSERT INTO point_grants (user_id, amount, remaining, earned_at, expires_at, consumed)
		SELECT up.user_id, up.balance, up.balance, NOW(), DATE_ADD(NOW(), INTERVAL 1 YEAR), 0
		FROM user_points up
		WHERE up.balance > 0
		  AND NOT EXISTS (SELECT 1 FROM point_grants g WHERE g.user_id = up.user_id)
	`)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// ============================================================================
// 幂等（point_idempotency）
// ----------------------------------------------------------------------------
// 幂等门在「调用方事务内」使用，保证并发重放串行化：
//   UpsertIdempotencyTx  插入或取已有行（OnConflict DoNothing）；
//   LockIdempotencyTx    对幂等行加行锁（FOR UPDATE），尚未处理的并发请求在此阻塞，
//                        直到首次处理提交后再读 status=done 并直接返回结果；
//   MarkIdempotencyDoneTx 写入处理结果（与工作同事务提交，杜绝并发重放窗口/重复加扣）。
// ============================================================================

// UpsertIdempotencyTx 在事务内插入幂等行（已存在则忽略）
func (r *PointRepository) UpsertIdempotencyTx(tx *gorm.DB, key string, userID uint, kind string) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoNothing: true,
	}).Create(&model.PointIdempotency{Key: key, UserID: userID, Kind: kind, Status: model.IdemStatusPending}).Error
}

// LockIdempotencyTx 对幂等行加行锁并读取（FOR UPDATE），供幂等门串行化并发重放
func (r *PointRepository) LockIdempotencyTx(tx *gorm.DB, key string) (*model.PointIdempotency, error) {
	var row model.PointIdempotency
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key = ?", key).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkIdempotencyDoneTx 在事务内写入处理结果（与工作同提交）
func (r *PointRepository) MarkIdempotencyDoneTx(tx *gorm.DB, key, result string) error {
	return tx.Model(&model.PointIdempotency{}).Where("key = ?", key).
		Updates(map[string]interface{}{"status": model.IdemStatusDone, "result": result}).Error
}
