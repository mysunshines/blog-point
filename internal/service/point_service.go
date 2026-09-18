package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/mysunshines/blog-point/internal/client"
	"github.com/mysunshines/blog-point/internal/model"
	"github.com/mysunshines/blog-point/internal/repository"
	"gorm.io/gorm"
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
	HasPurchased(ctx context.Context, userID uint, itemType string, itemID uint) (bool, error)

	// 事件加分（规则引擎入口）与消费
	EarnPoints(ctx context.Context, req *model.EarnPointsRequest) (*model.EarnResult, error)
	SpendPoints(ctx context.Context, req *model.SpendPointsRequest) (int64, error)

	// 后台管理
	AdminListRules(ctx context.Context) ([]*model.PointRule, error)
	AdminCreateRule(ctx context.Context, rule *model.PointRule) error
	AdminUpdateRule(ctx context.Context, rule *model.PointRule) error
	AdminDeleteRule(ctx context.Context, id uint) error
	AdminAdjustPoints(ctx context.Context, userID uint, amount int64, remark string) (int64, error)
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
	today := now.Format("2006-01-02")

	// 一天只能签到一次（唯一索引兜底，此处提前返回友好错误）
	if _, err := s.repo.GetCheckinByDate(ctx, userID, today); err == nil {
		return nil, ErrAlreadyCheckedIn
	}

	// 连续天数：昨日有签到则 +1，否则重新从 1 开始
	streak := 1
	if last, err := s.repo.LatestCheckin(ctx, userID); err == nil && last != nil {
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
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

// ============================================================================
// 规则引擎
// ============================================================================

// EarnPoints 事件加分：按事件取出启用规则，逐条匹配条件与限次，命中即计分。
// 分值完全由 point_rules 数据决定（管理员可配），代码不感知具体业务。
func (s *pointService) EarnPoints(ctx context.Context, req *model.EarnPointsRequest) (*model.EarnResult, error) {
	if req == nil || req.UserID == 0 || req.EventType == "" {
		return nil, ErrBadRequest
	}

	rules, err := s.repo.ListEnabledRulesByEvent(ctx, req.EventType)
	if err != nil {
		return nil, err
	}

	result := &model.EarnResult{}
	for _, rule := range rules {
		if rule.Points <= 0 {
			continue
		}
		// 条件匹配（JSON condition vs 事件上下文）
		if !matchCondition(rule.Condition, req.Context) {
			continue
		}
		// 限次防刷
		ok, err := s.checkLimit(ctx, req.UserID, rule)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, err := s.applyDelta(ctx, req.UserID, rule.Points,
			model.PointLogTypeEarn, rule.Code, req.RelatedType, req.RelatedID, rule.Name); err != nil {
			return nil, err
		}
		result.Points += rule.Points
		result.RuleCodes = append(result.RuleCodes, rule.Code)
	}
	// 产生加分后同步积分榜（best-effort）
	if result.Points > 0 {
		if p, err := s.repo.GetPoint(ctx, req.UserID); err == nil && p != nil {
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

// applyDelta 原子变更余额并写流水（正=获得，负=消费）
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
		updates := map[string]interface{}{"balance": gorm.Expr("balance + ?", amount)}
		if amount > 0 {
			updates["total_earned"] = gorm.Expr("total_earned + ?", amount)
		} else {
			updates["total_spent"] = gorm.Expr("total_spent + ?", -amount)
		}
		if err := tx.Model(&model.UserPoint{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
			return err
		}
		var after model.UserPoint
		if err := tx.Where("user_id = ?", userID).First(&after).Error; err != nil {
			return err
		}
		balance = after.Balance
		return s.repo.CreateLog(ctx, tx, &model.PointLog{
			UserID:       userID,
			Amount:       amount,
			BalanceAfter: balance,
			Type:         logType,
			RuleCode:     ruleCode,
			RelatedType:  relatedType,
			RelatedID:    relatedID,
			Remark:       remark,
		})
	})
	return balance, err
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

func (s *pointService) SpendPoints(ctx context.Context, req *model.SpendPointsRequest) (int64, error) {
	if req == nil || req.UserID == 0 || req.Amount <= 0 || req.ItemType == "" || req.ItemID == 0 {
		return 0, ErrBadRequest
	}

	var balance int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 事务内校验余额，防并发超卖
		var cur model.UserPoint
		if err := tx.Where("user_id = ?", req.UserID).First(&cur).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInsufficientPoints
			}
			return err
		}
		if cur.Balance < req.Amount {
			return ErrInsufficientPoints
		}
		if err := tx.Model(&model.UserPoint{}).Where("user_id = ?", req.UserID).Updates(map[string]interface{}{
			"balance":      gorm.Expr("balance - ?", req.Amount),
			"total_spent":  gorm.Expr("total_spent + ?", req.Amount),
		}).Error; err != nil {
			return err
		}
		var after model.UserPoint
		if err := tx.Where("user_id = ?", req.UserID).First(&after).Error; err != nil {
			return err
		}
		balance = after.Balance

		// 流水（负向）
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
		// 已购记录（唯一索引 + DoNothing，重复购买幂等）
		return s.repo.CreatePurchase(ctx, tx, &model.UserPurchase{
			UserID:   req.UserID,
			ItemType: req.ItemType,
			ItemID:   req.ItemID,
			Price:    req.Amount,
		})
	})
	if err == nil {
		s.syncBoard(ctx, req.UserID, balance)
	}
	return balance, err
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

// AdminAdjustPoints 管理员手动调整（纠错 / 运营发放）
func (s *pointService) AdminAdjustPoints(ctx context.Context, userID uint, amount int64, remark string) (int64, error) {
	if userID == 0 || amount == 0 {
		return 0, ErrBadRequest
	}
	balance, err := s.applyDelta(ctx, userID, amount, model.PointLogTypeAdminAdjust, "", "", 0, remark)
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
