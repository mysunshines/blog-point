package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mysunshines/blog-point/internal/client"
	"github.com/mysunshines/blog-point/internal/model"
	"github.com/mysunshines/blog-point/internal/repository"
	"github.com/mysunshines/gocommon/constants"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 领域错误（handler 负责映射为 proto 错误码）
var (
	ErrBadRequest         = errors.New("参数非法")
	ErrInsufficientPoints = errors.New("积分不足")
	ErrAlreadyCheckedIn   = errors.New("今日已签到")
	ErrRuleNotFound       = errors.New("规则不存在")
)

// CheckInResult 签到结果
type CheckInResult struct {
	Streak int      // 连续签到天数
	Points int64    // 本次获得积分
	Rules  []string // 命中的规则 code
}

// PointService 积分服务接口
type PointService interface {
	// 用户侧
	CheckIn(ctx context.Context, userID uint) (*CheckInResult, error)
	GetMyPoints(ctx context.Context, userID uint) (*model.UserPoint, error)
	GetPointLogs(ctx context.Context, userID uint, page, size int) ([]*model.PointLog, int64, error)
	GetCheckinStatus(ctx context.Context, userID uint) (*CheckinStatus, error)
	HasPurchased(ctx context.Context, userID uint, itemType string, itemID uint) (bool, error)
	// RebuildUserPointsBoard 全量重建用户积分榜（启动时调用：Redis 重启/数据丢失后自愈）
	RebuildUserPointsBoard(ctx context.Context) (int, error)

	// 事件加分（规则引擎入口）与消费
	EarnPoints(ctx context.Context, req *model.EarnPointsRequest) (*model.EarnResult, error)
	SpendPoints(ctx context.Context, req *model.SpendPointsRequest) (int64, error)

	// 后台管理
	AdminListRules(ctx context.Context) ([]*model.PointRule, error)
	AdminCreateRule(ctx context.Context, rule *model.PointRule) error
	AdminUpdateRule(ctx context.Context, rule *model.PointRule) error
	AdminDeleteRule(ctx context.Context, id uint) error
	AdminAdjustPoints(ctx context.Context, userID uint, amount int64, remark string) (int64, error)

	// 积分过期与存量迁移（启动 / 每日定时）
	ExpireGrants(ctx context.Context) (int64, error)        // 清零过期批次并扣减可用余额
	EnsureLegacyGrants(ctx context.Context) (int64, error)  // 为存量积分生成 legacy 批次
}

type pointService struct {
	repo *repository.PointRepository
	db   *gorm.DB
}

// NewPointService 构造积分服务
func NewPointService(repo *repository.PointRepository, db *gorm.DB) PointService {
	return &pointService{repo: repo, db: db}
}

// ============================================================================
// 签到
// ============================================================================

func (s *pointService) CheckIn(ctx context.Context, userID uint) (*CheckInResult, error) {
	if userID == 0 {
		return nil, ErrBadRequest
	}
	now := time.Now()
	today := now.Format(constants.DateFormat)

	// 一天只能签到一次（唯一索引兜底，此处提前返回友好错误）
	if _, err := s.repo.GetCheckinByDate(ctx, userID, today); err == nil {
		return nil, ErrAlreadyCheckedIn
	}

	// 连续天数：昨日有签到则 +1，否则重新从 1 开始
	streak := 1
	if last, err := s.repo.LatestCheckin(ctx, userID); err == nil && last != nil {
		yesterday := now.AddDate(0, 0, -1).Format(constants.DateFormat)
		if last.CheckinDate == yesterday {
			streak = last.Streak + 1
		}
	}

	if err := s.repo.CreateCheckin(ctx, &model.UserCheckin{
		UserID:      userID,
		CheckinDate: today,
		Streak:      streak,
	}); err != nil {
		return nil, err
	}

	// 走规则引擎：可同时命中「每日签到」与「连续签到 N 天」等多条规则
	res, err := s.EarnPoints(ctx, &model.EarnPointsRequest{
		UserID:    userID,
		EventType: model.EventCheckin,
		Context:   map[string]interface{}{"streak": streak},
	})
	if err != nil {
		return nil, err
	}

	return &CheckInResult{Streak: streak, Points: res.Points, Rules: res.RuleCodes}, nil
}

// CheckinStatus 签到状态（只读）
type CheckinStatus struct {
	CheckedInToday  bool
	Streak          int
	LastCheckinDate string
}

// GetCheckinStatus 查询用户签到状态：今日是否已签到 + 当前连续天数。
// 连续天数语义：今天已签到取当日记录；今天未签到但昨天签到了则保留（还没断），
// 否则记为 0（断签）。前端据此渲染「已签到 / 可签到」。
func (s *pointService) GetCheckinStatus(ctx context.Context, userID uint) (*CheckinStatus, error) {
	if userID == 0 {
		return nil, ErrBadRequest
	}
	now := time.Now()
	today := now.Format(constants.DateFormat)

	todayRec, err := s.repo.GetCheckinByDate(ctx, userID, today)
	checked := err == nil && todayRec != nil

	streak := 0
	lastDate := ""
	if last, err := s.repo.LatestCheckin(ctx, userID); err == nil && last != nil {
		lastDate = last.CheckinDate
		if checked {
			streak = todayRec.Streak
		} else {
			yesterday := now.AddDate(0, 0, -1).Format(constants.DateFormat)
			if last.CheckinDate == yesterday {
				streak = last.Streak
			}
		}
	}

	return &CheckinStatus{
		CheckedInToday:  checked,
		Streak:          streak,
		LastCheckinDate: lastDate,
	}, nil
}

// ============================================================================
// 规则引擎
// ============================================================================

// EarnPoints 事件加分：按事件取出启用规则，逐条匹配条件与限次，命中即计分。
// 分值完全由 point_rules 数据决定（管理员可配），代码不感知具体业务。
// 幂等：调用方事务内对幂等行加行锁（FOR UPDATE）串行化并发重放，已处理（done）的请求
// 直接返回首次结果；加分在单事务内完成（余额 + 流水 + 积分批次），保证不重复发放。
func (s *pointService) EarnPoints(ctx context.Context, req *model.EarnPointsRequest) (*model.EarnResult, error) {
	if req == nil || req.UserID == 0 || req.EventType == "" {
		return nil, ErrBadRequest
	}

	rules, err := s.repo.ListEnabledRulesByEvent(ctx, req.EventType)
	if err != nil {
		return nil, err
	}

	// 先过滤命中规则（只读，不改库）
	var hits []*model.PointRule
	for _, rule := range rules {
		if rule.Points <= 0 {
			continue
		}
		if !matchCondition(rule.Condition, req.Context) {
			continue
		}
		ok, err := s.checkLimit(ctx, req.UserID, rule)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		hits = append(hits, rule)
	}

	key := s.earnIdemKey(req)
	// 确保账户存在（惰性开户；重放时无害）
	if _, err := s.repo.EnsurePoint(ctx, req.UserID); err != nil {
		return nil, err
	}

	result := &model.EarnResult{}
	replayed := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 幂等门：插入或取已有幂等行并加行锁，串行化并发重放，杜绝重复加分
		if err := s.repo.UpsertIdempotencyTx(tx, key, req.UserID, model.IdemKindEarn); err != nil {
			return err
		}
		row, err := s.repo.LockIdempotencyTx(tx, key)
		if err != nil {
			return err
		}
		if row.Status == model.IdemStatusDone {
			// 重放：直接解码首次结果，不做任何加分
			if r, ok := decodeIdemEarn(row.Result); ok {
				*result = *r
			}
			replayed = true
			return nil
		}
		// 首次处理：单事务内完成所有加分（余额 + 流水 + 批次）
		now := time.Now()
		for _, rule := range hits {
			if _, err := s.applyEarnTx(ctx, tx, req.UserID, rule.Points,
				model.PointLogTypeEarn, rule.Code, req.RelatedType, req.RelatedID, rule.Name, now); err != nil {
				return err
			}
			result.Points += rule.Points
			result.RuleCodes = append(result.RuleCodes, rule.Code)
		}
		return s.repo.MarkIdempotencyDoneTx(tx, key, s.encodeIdemEarn(result))
	})
	if err != nil {
		return nil, err
	}

	// 产生加分后同步积分榜（best-effort，重放不重复同步）
	if !replayed && result.Points > 0 {
		if p, e := s.repo.GetPoint(ctx, req.UserID); e == nil && p != nil {
			s.syncBoard(ctx, req.UserID, p.Balance)
		}
	}
	return result, nil
}

// checkLimit 判断规则是否还有发放额度
func (s *pointService) checkLimit(ctx context.Context, userID uint, rule *model.PointRule) (bool, error) {
	if rule.LimitType == model.LimitTypeNone || rule.LimitCount <= 0 {
		return true, nil
	}
	cnt, err := s.repo.CountLogsByRuleSince(ctx, userID, rule.Code, limitWindowStart(rule.LimitType))
	if err != nil {
		return false, err
	}
	return cnt < int64(rule.LimitCount), nil
}

// limitWindowStart 限次窗口起点
func limitWindowStart(limitType string) time.Time {
	now := time.Now()
	switch limitType {
	case model.LimitTypeDaily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case model.LimitTypeWeekly:
		// 周一为一周起点
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		d := now.AddDate(0, 0, -(wd - 1))
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
	default:
		// once / none：自纪元起统计
		return time.Time{}
	}
}

// matchCondition 用事件上下文匹配规则条件。
// 支持：{"streak":7}（相等）、{"rank_lte":10}（数值比较：_lte/_gte/_lt/_gt）。
func matchCondition(condition string, ctx map[string]interface{}) bool {
	if strings.TrimSpace(condition) == "" {
		return true
	}
	var cond map[string]interface{}
	if err := json.Unmarshal([]byte(condition), &cond); err != nil {
		return false
	}
	if len(cond) == 0 {
		return true
	}
	if ctx == nil {
		return false
	}
	for k, want := range cond {
		base, op := splitOp(k)
		actual, ok := ctx[base]
		if !ok {
			return false
		}
		if op != "" {
			if !compareNumber(actual, toFloat64(want), op) {
				return false
			}
			continue
		}
		if !equalValue(actual, want) {
			return false
		}
	}
	return true
}

func splitOp(key string) (string, string) {
	for _, op := range []string{"_lte", "_gte", "_lt", "_gt"} {
		if strings.HasSuffix(key, op) {
			return strings.TrimSuffix(key, op), op
		}
	}
	return key, ""
}

func compareNumber(actual interface{}, want float64, op string) bool {
	a := toFloat64(actual)
	switch op {
	case "_lte":
		return a <= want
	case "_gte":
		return a >= want
	case "_lt":
		return a < want
	case "_gt":
		return a > want
	}
	return false
}

func equalValue(actual, want interface{}) bool {
	switch w := want.(type) {
	case float64:
		return toFloat64(actual) == w
	case string:
		v, ok := actual.(string)
		return ok && v == w
	case bool:
		v, ok := actual.(bool)
		return ok && v == w
	}
	return false
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case uint:
		return float64(n)
	}
	return 0
}

// ============================================================================
// 账户与流水
// ============================================================================

// applyDelta 原子变更余额并写流水（正=获得，负=消费）。正值时同时建立积分批次
// （1 年过期），供 FIFO 消费与过期清理使用。仅用于管理员单笔调整等单操作场景。
func (s *pointService) applyDelta(ctx context.Context, userID uint, amount int64,
	logType, ruleCode, relatedType string, relatedID uint, remark string) (int64, error) {

	if amount == 0 {
		p, err := s.repo.EnsurePoint(ctx, userID)
		if err != nil {
			return 0, err
		}
		return p.Balance, nil
	}

	var balance int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 惰性开户（账户不存在时先建余额为 0 的账户）
		if _, err := s.repo.EnsurePoint(ctx, userID); err != nil {
			return err
		}
		now := time.Now()
		if _, err := s.applyEarnTx(ctx, tx, userID, amount, logType, ruleCode, relatedType, relatedID, remark, now); err != nil {
			return err
		}
		var after model.UserPoint
		if err := tx.Where("user_id = ?", userID).First(&after).Error; err != nil {
			return err
		}
		balance = after.Balance
		return nil
	})
	return balance, err
}

// applyEarnTx 在事务内「更新余额 + 写流水 + 建积分批次」（正值获得）。
func (s *pointService) applyEarnTx(ctx context.Context, tx *gorm.DB, userID uint, amount int64,
	logType, ruleCode, relatedType string, relatedID uint, remark string, now time.Time) (int64, error) {
	if amount == 0 {
		return 0, nil
	}
	updates := map[string]interface{}{"balance": gorm.Expr("balance + ?", amount)}
	if amount > 0 {
		updates["total_earned"] = gorm.Expr("total_earned + ?", amount)
	} else {
		updates["total_spent"] = gorm.Expr("total_spent + ?", -amount)
	}
	if err := tx.Model(&model.UserPoint{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		return 0, err
	}
	var after model.UserPoint
	if err := tx.Where("user_id = ?", userID).First(&after).Error; err != nil {
		return 0, err
	}
	if err := s.repo.CreateLog(ctx, tx, &model.PointLog{
		UserID:       userID,
		Amount:       amount,
		BalanceAfter: after.Balance,
		Type:         logType,
		RuleCode:     ruleCode,
		RelatedType:  relatedType,
		RelatedID:    relatedID,
		Remark:       remark,
	}); err != nil {
		return 0, err
	}
	// 仅正获得建立批次（负调整走 consumeGrants，不建批次）
	if amount > 0 {
		if err := s.repo.CreateGrant(ctx, tx, &model.PointGrant{
			UserID:    userID,
			Amount:    amount,
			Remaining: amount,
			EarnedAt:  now,
			ExpiresAt: model.GrantExpiry(now),
		}); err != nil {
			return 0, err
		}
	}
	return after.Balance, nil
}

// consumeGrants 在事务内按 FIFO（earned_at ASC）从最早批次扣减积分（仅未过期批次），
// 并同步扣减账户余额与累计消费。余额不足（含积分已过期）返回 ErrInsufficientPoints。
// 调用方需保证账户行 / 批次行加锁，防止并发超卖。
func (s *pointService) consumeGrants(ctx context.Context, tx *gorm.DB, userID uint, amount int64, now time.Time) (int64, error) {
	var up model.UserPoint
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).First(&up).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrInsufficientPoints
		}
		return 0, err
	}
	grants, err := s.repo.ListUnexpiredGrants(ctx, tx, userID, now, true)
	if err != nil {
		return 0, err
	}
	var available int64
	for _, g := range grants {
		available += g.Remaining
	}
	if available < amount {
		return 0, ErrInsufficientPoints
	}
	left := amount
	for _, g := range grants {
		if left == 0 {
			break
		}
		take := g.Remaining
		if take > left {
			take = left
		}
		g.Remaining -= take
		left -= take
		if err := s.repo.UpdateGrantRemaining(ctx, tx, g.ID, g.Remaining); err != nil {
			return 0, err
		}
	}
	balance := up.Balance - amount
	if err := tx.Model(&model.UserPoint{}).Where("user_id = ?", userID).Updates(map[string]interface{}{
		"balance":     balance,
		"total_spent": gorm.Expr("total_spent + ?", amount),
	}).Error; err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *pointService) GetMyPoints(ctx context.Context, userID uint) (*model.UserPoint, error) {
	if userID == 0 {
		return nil, ErrBadRequest
	}
	return s.repo.EnsurePoint(ctx, userID)
}

func (s *pointService) GetPointLogs(ctx context.Context, userID uint, page, size int) ([]*model.PointLog, int64, error) {
	if userID == 0 {
		return nil, 0, ErrBadRequest
	}
	return s.repo.ListLogs(ctx, userID, page, size)
}

// ============================================================================
// 消费（购买付费文章 / 背景）
// ============================================================================

// SpendPoints 消费积分（购买付费文章 / 背景等）。
// 幂等：事务内对幂等行加行锁串行化并发重放，已处理直接返回余额；事务内「先插购买记录
// （唯一索引）」，重复购买（RowsAffected==0）直接返回当前余额，绝不二次扣费（修复旧实现
// 「先扣后查」的重复扣款 bug）。新购买则按 FIFO 从未过期批次扣减，并写负向流水。
func (s *pointService) SpendPoints(ctx context.Context, req *model.SpendPointsRequest) (int64, error) {
	if req == nil || req.UserID == 0 || req.Amount <= 0 || req.ItemType == "" || req.ItemID == 0 {
		return 0, ErrBadRequest
	}

	key := s.spendIdemKey(req)
	var balance int64
	replayed := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 幂等门：行锁串行化并发重放
		if err := s.repo.UpsertIdempotencyTx(tx, key, req.UserID, model.IdemKindSpend); err != nil {
			return err
		}
		row, err := s.repo.LockIdempotencyTx(tx, key)
		if err != nil {
			return err
		}
		if row.Status == model.IdemStatusDone {
			if b, ok := decodeIdemSpend(row.Result); ok {
				balance = b
			}
			replayed = true
			return nil
		}
		now := time.Now()
		// 先插购买记录（唯一索引）：重复购买直接返回当前余额，不二次扣费
		aff, err := s.repo.CreatePurchase(ctx, tx, &model.UserPurchase{
			UserID:   req.UserID,
			ItemType: req.ItemType,
			ItemID:   req.ItemID,
			Price:    req.Amount,
		})
		if err != nil {
			return err
		}
		if aff == 0 {
			var cur model.UserPoint
			if err := tx.Where("user_id = ?", req.UserID).First(&cur).Error; err != nil {
				return err
			}
			balance = cur.Balance
			return s.repo.MarkIdempotencyDoneTx(tx, key, s.encodeIdemSpend(balance))
		}
		// 新购买：FIFO 扣减未过期批次
		bal, err := s.consumeGrants(ctx, tx, req.UserID, req.Amount, now)
		if err != nil {
			return err
		}
		balance = bal
		if err := s.repo.CreateLog(ctx, tx, &model.PointLog{
			UserID:       req.UserID,
			Amount:       -req.Amount,
			BalanceAfter: balance,
			Type:         model.PointLogTypeSpend,
			RelatedType:  req.ItemType,
			RelatedID:    req.ItemID,
			Remark:       req.Remark,
		}); err != nil {
			return err
		}
		return s.repo.MarkIdempotencyDoneTx(tx, key, s.encodeIdemSpend(balance))
	})
	if err != nil {
		return 0, err
	}
	if !replayed {
		s.syncBoard(ctx, req.UserID, balance)
	}
	return balance, nil
}

func (s *pointService) HasPurchased(ctx context.Context, userID uint, itemType string, itemID uint) (bool, error) {
	if userID == 0 || itemType == "" || itemID == 0 {
		return false, ErrBadRequest
	}
	return s.repo.HasPurchase(ctx, userID, itemType, itemID)
}

// ============================================================================
// 后台管理
// ============================================================================

func (s *pointService) AdminListRules(ctx context.Context) ([]*model.PointRule, error) {
	return s.repo.ListRules(ctx)
}

func (s *pointService) AdminCreateRule(ctx context.Context, rule *model.PointRule) error {
	if rule == nil || rule.Code == "" || rule.EventType == "" {
		return ErrBadRequest
	}
	if rule.LimitType == "" {
		rule.LimitType = model.LimitTypeNone
	}
	if rule.Status == 0 {
		rule.Status = model.RuleStatusEnabled
	}
	return s.repo.CreateRule(ctx, rule)
}

func (s *pointService) AdminUpdateRule(ctx context.Context, rule *model.PointRule) error {
	if rule == nil || rule.ID == 0 {
		return ErrBadRequest
	}
	// 校验存在性
	if _, err := s.repo.GetRuleByID(ctx, rule.ID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRuleNotFound
		}
		return err
	}
	// 显式字段更新，避免「只改状态」时把未传字段清零（Select("*") 的坑）：
	//   - 空字符串 / 0 视为「本次不修改」
	//   - status 例外：0（停用）也必须能写入，故始终更新
	// sort 传负数（-1）表示「不修改」，便于「仅启停」时保持原排序
	updates := map[string]interface{}{"status": rule.Status}
	if rule.Sort >= 0 {
		updates["sort"] = rule.Sort
	}
	if rule.Name != "" {
		updates["name"] = rule.Name
	}
	if rule.Condition != "" {
		updates["condition"] = rule.Condition
	}
	if rule.Points != 0 {
		updates["points"] = rule.Points
	}
	if rule.LimitType != "" {
		updates["limit_type"] = rule.LimitType
	}
	if rule.LimitCount != 0 {
		updates["limit_count"] = rule.LimitCount
	}
	return s.repo.UpdateRuleFields(ctx, rule.ID, updates)
}

func (s *pointService) AdminDeleteRule(ctx context.Context, id uint) error {
	if id == 0 {
		return ErrBadRequest
	}
	return s.repo.DeleteRule(ctx, id)
}

// AdminAdjustPoints 管理员手动调整（纠错 / 运营发放）。
// 正调整：发放积分并建批次（1 年过期）；负调整：从 FIFO 批次扣减，保证余额 = 未过期批次之和。
func (s *pointService) AdminAdjustPoints(ctx context.Context, userID uint, amount int64, remark string) (int64, error) {
	if userID == 0 || amount == 0 {
		return 0, ErrBadRequest
	}
	if amount > 0 {
		balance, err := s.applyDelta(ctx, userID, amount, model.PointLogTypeAdminAdjust, "", "", 0, remark)
		if err == nil {
			s.syncBoard(ctx, userID, balance)
		}
		return balance, err
	}
	// 负调整：FIFO 扣减（与普通消费一致）
	var balance int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.repo.EnsurePoint(ctx, userID); err != nil {
			return err
		}
		bal, err := s.consumeGrants(ctx, tx, userID, -amount, time.Now())
		if err != nil {
			return err
		}
		balance = bal
		return s.repo.CreateLog(ctx, tx, &model.PointLog{
			UserID:       userID,
			Amount:       amount,
			BalanceAfter: balance,
			Type:         model.PointLogTypeAdminAdjust,
			Remark:       remark,
		})
	})
	if err == nil {
		s.syncBoard(ctx, userID, balance)
	}
	return balance, err
}

// syncBoard 同步用户积分到积分榜（best-effort：榜单是派生数据，失败只告警）
func (s *pointService) syncBoard(ctx context.Context, userID uint, balance int64) {
	if err := client.SyncUserPoints(ctx, userID, balance); err != nil {
		log.Printf("[ranking] sync user points failed user=%d: %v", userID, err)
	}
}

// RebuildUserPointsBoard 全量重建用户积分榜：把 user_points 表中所有账户的余额
// 以绝对值覆盖写入 ranking-service（ZADD SET）。
// 背景：积分只在「发生变动」时才同步到榜单，若 Redis 重启/数据丢失，榜单会为空且
// 无法自愈（只有下次积分变动的那批用户才会回到榜上）。启动时按 DB 余额重建即可修复。
// best-effort：单条失败只告警并继续，返回成功写入的用户数。
func (s *pointService) RebuildUserPointsBoard(ctx context.Context) (int, error) {
	points, err := s.repo.ListAllPoints(ctx)
	if err != nil {
		return 0, err
	}
	ok := 0
	for _, p := range points {
		if err := client.SyncUserPoints(ctx, p.UserID, p.Balance); err != nil {
			log.Printf("[ranking] rebuild sync user points failed user=%d: %v", p.UserID, err)
			continue
		}
		ok++
	}
	return ok, nil
}

// ============================================================================
// 幂等辅助
// ============================================================================

// idemResult 幂等结果序列化结构（重放时直接返回）
type idemResult struct {
	Points  int64    `json:"points"`
	Rules   []string `json:"rules,omitempty"`
	Balance int64    `json:"balance"`
}

// earnIdemKey 派生加分幂等键；调用方传入则优先
func (s *pointService) earnIdemKey(req *model.EarnPointsRequest) string {
	if req.IdempotencyKey != "" {
		return req.IdempotencyKey
	}
	return fmt.Sprintf("earn:%d:%s:%s:%d", req.UserID, req.EventType, req.RelatedType, req.RelatedID)
}

// spendIdemKey 派生消费幂等键；调用方传入则优先
func (s *pointService) spendIdemKey(req *model.SpendPointsRequest) string {
	if req.IdempotencyKey != "" {
		return req.IdempotencyKey
	}
	return fmt.Sprintf("spend:%d:%s:%d", req.UserID, req.ItemType, req.ItemID)
}

func (s *pointService) encodeIdemEarn(r *model.EarnResult) string {
	b, _ := json.Marshal(idemResult{Points: r.Points, Rules: r.RuleCodes})
	return string(b)
}

func (s *pointService) encodeIdemSpend(balance int64) string {
	b, _ := json.Marshal(idemResult{Balance: balance})
	return string(b)
}

// decodeIdemEarn 解码已落库的加分结果（重放时使用）
func decodeIdemEarn(result string) (*model.EarnResult, bool) {
	var r idemResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		return nil, false
	}
	return &model.EarnResult{Points: r.Points, RuleCodes: r.Rules}, true
}

// decodeIdemSpend 解码已落库的消费结果（重放时使用）
func decodeIdemSpend(result string) (int64, bool) {
	var r idemResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		return 0, false
	}
	return r.Balance, true
}

// ============================================================================
// 积分过期与存量迁移
// ============================================================================

// ExpireGrants 每日定时：清零过期批次并扣减账户可用余额（best-effort，失败仅告警）。
func (s *pointService) ExpireGrants(ctx context.Context) (int64, error) {
	return s.repo.ExpireGrants(ctx, time.Now())
}

// EnsureLegacyGrants 存量迁移：为每个「有余额但无批次」的用户生成 legacy 批次，
// 使 FIFO/过期逻辑对存量积分生效。返回生成批次数。
func (s *pointService) EnsureLegacyGrants(ctx context.Context) (int64, error) {
	return s.repo.BackfillLegacyGrants(ctx)
}
