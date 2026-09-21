package model

import "time"

// ============================================================================
// 积分域数据模型
// ----------------------------------------------------------------------------
// 设计要点：
//   1. 账户（user_points）与流水（point_logs）分离：账户余额是「当前状态」，
//      流水是「不可变事实」，任何积分变动必须同时写流水，便于对账与审计。
//   2. 规则（point_rules）把「事件 -> 积分」的映射外置为数据，管理员可配，
//      分值不写死在代码里（这是「灵活配置积分规则」的核心）。
//   3. 金额一律用 int64（正负号表示方向），避免浮点误差。
// ============================================================================

// UserPoint 积分账户（一个用户一行）
type UserPoint struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance     int64     `gorm:"not null;default:0" json:"balance"`      // 当前可用积分
	TotalEarned int64     `gorm:"not null;default:0" json:"total_earned"` // 累计获得
	TotalSpent  int64     `gorm:"not null;default:0" json:"total_spent"`  // 累计消费
	CreatedAt   time.Time `gorm:"<-:create" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UserPoint) TableName() string { return "user_points" }

// PointLog 积分流水（追加写，不更新、不删除）
type PointLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Amount       int64     `gorm:"not null" json:"amount"` // 正=获得，负=消费
	BalanceAfter int64     `gorm:"not null;default:0" json:"balance_after"`
	Type         string    `gorm:"size:32;not null" json:"type"` // earn / spend / admin_adjust
	RuleCode     string    `gorm:"size:64;not null;default:''" json:"rule_code"`
	RelatedType  string    `gorm:"size:32;not null;default:''" json:"related_type"` // article / background
	RelatedID    uint      `gorm:"not null;default:0" json:"related_id"`
	Remark       string    `gorm:"size:256;not null;default:''" json:"remark"`
	CreatedAt    time.Time `gorm:"<-:create;index" json:"created_at"`
}

func (PointLog) TableName() string { return "point_logs" }

// PointRule 积分规则（管理员可配）
type PointRule struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Code       string    `gorm:"uniqueIndex;size:64;not null" json:"code"` // 唯一标识，如 checkin_daily
	Name       string    `gorm:"size:128;not null" json:"name"`
	EventType  string    `gorm:"size:64;not null;index" json:"event_type"`          // checkin / publish_article / weekly_rank ...
	Condition  string    `gorm:"type:text" json:"condition"`                        // JSON 条件，如 {"streak":7}
	Points     int64     `gorm:"not null;default:0" json:"points"`                  // 奖励积分
	LimitType  string    `gorm:"size:16;not null;default:'none'" json:"limit_type"` // none/daily/weekly/once
	LimitCount int       `gorm:"not null;default:0" json:"limit_count"`             // 窗口内最多发放次数（0=不限）
	Status     uint      `gorm:"not null;default:1" json:"status"`                  // 1=启用 0=停用
	Sort       int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt  time.Time `gorm:"<-:create" json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PointRule) TableName() string { return "point_rules" }

// UserCheckin 签到记录（user_id + checkin_date 唯一，保证一天一次）
type UserCheckin struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;uniqueIndex:idx_user_date" json:"user_id"`
	CheckinDate string    `gorm:"size:10;not null;uniqueIndex:idx_user_date" json:"checkin_date"` // YYYY-MM-DD
	Streak      int       `gorm:"not null;default:1" json:"streak"`                               // 连续签到天数（签到当日）
	CreatedAt   time.Time `gorm:"<-:create" json:"created_at"`
}

func (UserCheckin) TableName() string { return "user_checkins" }

// UserPurchase 已购记录（付费文章 / 背景），唯一约束防重复付费
type UserPurchase struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_user_item" json:"user_id"`
	ItemType  string    `gorm:"size:32;not null;uniqueIndex:idx_user_item" json:"item_type"` // article / background
	ItemID    uint      `gorm:"not null;uniqueIndex:idx_user_item" json:"item_id"`
	Price     int64     `gorm:"not null;default:0" json:"price"` // 成交价（积分）
	CreatedAt time.Time `gorm:"<-:create" json:"created_at"`
}

func (UserPurchase) TableName() string { return "user_purchases" }

// 积分流水类型
const (
	PointLogTypeEarn        = "earn"         // 规则/事件获得
	PointLogTypeSpend       = "spend"        // 消费（购买）
	PointLogTypeAdminAdjust = "admin_adjust" // 管理员调整
)

// 规则限次类型
const (
	LimitTypeNone   = "none"   // 不限
	LimitTypeDaily  = "daily"  // 每日限次
	LimitTypeWeekly = "weekly" // 每周限次
	LimitTypeOnce   = "once"   // 仅一次（终身）
)

// 事件类型（与规则 event_type 对应；后续扩展只需加常量 + 管理员配规则）
const (
	EventCheckin        = "checkin"           // 签到
	EventPublishArticle = "publish_article"   // 发布文章
	EventWeeklyRank     = "weekly_rank"       // 周榜排名
	EventArticleSold    = "article_purchased" // 文章被购买（作者收益）
)

// 规则启用状态
const (
	RuleStatusDisabled uint = 0
	RuleStatusEnabled  uint = 1
)

// ============ 请求/响应 DTO ============

// EarnPointsRequest 事件加分入参（service 层）
type EarnPointsRequest struct {
	UserID      uint
	EventType   string
	Context     map[string]interface{} // 供 condition 匹配的上下文
	RelatedType string
	RelatedID   uint
}

// EarnResult 事件加分结果
type EarnResult struct {
	Points    int64
	RuleCodes []string
}

// SpendPointsRequest 消费入参（service 层）
type SpendPointsRequest struct {
	UserID   uint
	Amount   int64
	ItemType string
	ItemID   uint
	Remark   string
}
