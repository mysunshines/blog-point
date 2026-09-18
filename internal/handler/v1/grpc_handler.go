package v1

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"

	"github.com/mysunshines/blog-point/internal/model"
	"github.com/mysunshines/blog-point/internal/service"
	pb "github.com/mysunshines/blog-point/proto/pb/v1"

	"github.com/mysunshines/gocommon/constants"
	commonmiddleware "github.com/mysunshines/gocommon/middleware"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GrpcPointHandler gRPC 积分处理器（支持熔断）
type GrpcPointHandler struct {
	pb.UnimplementedPointServiceServer
	Svc service.PointService
	Cb  *gobreaker.CircuitBreaker
}

// requireGRPCAdmin 校验后台接口访问权限：先校验 JWT 登录，再校验角色为管理员。
// 后台接口经 Gateway /api/v1 反射代理转发，仅靠 JWT 鉴权不够，必须显式校验角色，
// 否则任意登录用户都能调用 Admin* 方法。
func requireGRPCAdmin(ctx context.Context) error {
	if _, err := commonmiddleware.RequireGRPCAuth(ctx); err != nil {
		return err
	}
	raw, ok := commonmiddleware.GetGRPCRole(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "未认证")
	}
	var role uint8
	switch v := raw.(type) {
	case float64:
		role = uint8(v)
	case uint8:
		role = v
	case int:
		role = uint8(v)
	case int64:
		role = uint8(v)
	default:
		return status.Error(codes.PermissionDenied, "无效的角色信息")
	}
	if role != constants.RoleAdmin {
		return status.Error(codes.PermissionDenied, "需要管理员权限")
	}
	return nil
}

// isInternalCall 判断是否为可信内部服务调用。
// 内部服务（如 article-service）没有用户 JWT，改为携带共享令牌
// （metadata: x-point-internal，由环境变量 POINT_INTERNAL_TOKEN 下发）。
// 令牌未配置时内部通道关闭，仅用户 JWT 可用（与 ranking 摄入接口的令牌思路一致）。
func isInternalCall(ctx context.Context) bool {
	token := os.Getenv("POINT_INTERNAL_TOKEN")
	if token == "" {
		return false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	for _, v := range md.Get("x-point-internal") {
		if subtle.ConstantTimeCompare([]byte(v), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

// resolveUser 解析目标用户：
//   - 内部服务调用（带有效令牌）：信任请求中的 user_id（业务服务已确认过业务归属）
//   - 用户调用：用 JWT 登录用户；显式指定他人需管理员权限
func resolveUser(ctx context.Context, reqUserID uint32) (uint, error) {
	if isInternalCall(ctx) {
		if reqUserID == 0 {
			return 0, status.Error(codes.InvalidArgument, "内部调用必须指定 user_id")
		}
		return uint(reqUserID), nil
	}
	uid, err := commonmiddleware.RequireGRPCAuth(ctx)
	if err != nil {
		return 0, err
	}
	if reqUserID == 0 || uint(reqUserID) == uid {
		return uid, nil
	}
	if err := requireGRPCAdmin(ctx); err != nil {
		return 0, err
	}
	return uint(reqUserID), nil
}

// errCode 领域错误 → proto 错误码
func errCode(err error) uint32 {
	switch {
	case errors.Is(err, service.ErrInsufficientPoints):
		return uint32(pb.PointErrorCode_POINT_INSUFFICIENT)
	case errors.Is(err, service.ErrAlreadyCheckedIn):
		return uint32(pb.PointErrorCode_POINT_ALREADY_CHECKED_IN)
	case errors.Is(err, service.ErrRuleNotFound):
		return uint32(pb.PointErrorCode_POINT_RULE_NOT_FOUND)
	case errors.Is(err, service.ErrBadRequest):
		return uint32(pb.PointErrorCode_POINT_BAD_REQUEST)
	default:
		return uint32(pb.PointErrorCode_POINT_INTERNAL_ERROR)
	}
}

// ============================================================================
// 用户侧
// ============================================================================

// CheckIn 每日签到（命中「签到」与「连续签到 N 天」等规则）
func (h *GrpcPointHandler) CheckIn(ctx context.Context, req *pb.CheckInRequest) (*pb.CheckInResponse, error) {
	uid, err := commonmiddleware.RequireGRPCAuth(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.Svc.CheckIn(ctx, uid)
	if err != nil {
		return &pb.CheckInResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.CheckInResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Result: &pb.CheckInResult{
			Streak: uint32(res.Streak),
			Points: res.Points,
			Rules:  res.Rules,
		},
	}, nil
}

// GetMyPoints 查询我的积分账户
func (h *GrpcPointHandler) GetMyPoints(ctx context.Context, req *pb.GetMyPointsRequest) (*pb.GetMyPointsResponse, error) {
	uid, err := commonmiddleware.RequireGRPCAuth(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.Svc.GetMyPoints(ctx, uid)
	if err != nil {
		return &pb.GetMyPointsResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.GetMyPointsResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Point:   convertToProtoUserPoint(p),
	}, nil
}

// GetPointLogs 我的积分流水
func (h *GrpcPointHandler) GetPointLogs(ctx context.Context, req *pb.GetPointLogsRequest) (*pb.GetPointLogsResponse, error) {
	uid, err := commonmiddleware.RequireGRPCAuth(ctx)
	if err != nil {
		return nil, err
	}
	logs, total, err := h.Svc.GetPointLogs(ctx, uid, int(req.Page), int(req.PageSize))
	if err != nil {
		return &pb.GetPointLogsResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	list := make([]*pb.PointLog, 0, len(logs))
	for _, l := range logs {
		list = append(list, convertToProtoPointLog(l))
	}
	return &pb.GetPointLogsResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Logs:    list,
		Total:   uint32(total),
	}, nil
}

// HasPurchased 查询是否已购买某物品（付费文章 / 背景）
func (h *GrpcPointHandler) HasPurchased(ctx context.Context, req *pb.HasPurchasedRequest) (*pb.HasPurchasedResponse, error) {
	target, err := resolveUser(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	ok, err := h.Svc.HasPurchased(ctx, target, req.ItemType, uint(req.ItemId))
	if err != nil {
		return &pb.HasPurchasedResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.HasPurchasedResponse{
		Code:      uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message:   "success",
		Purchased: ok,
	}, nil
}

// ============================================================================
// 事件加分 / 消费
// ============================================================================

// EarnPoints 事件加分（规则引擎入口）。
// 未指定 user_id 时为当前登录用户加分；指定他人需管理员权限（防冒名加分）。
func (h *GrpcPointHandler) EarnPoints(ctx context.Context, req *pb.EarnPointsRequest) (*pb.EarnPointsResponse, error) {
	target, err := resolveUser(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	// 上下文 JSON → map，供规则 condition 匹配
	var ctxMap map[string]interface{}
	if req.Context != "" {
		if err := json.Unmarshal([]byte(req.Context), &ctxMap); err != nil {
			return &pb.EarnPointsResponse{
				Code:    uint32(pb.PointErrorCode_POINT_BAD_REQUEST),
				Message: "context 不是合法 JSON",
			}, nil
		}
	}
	res, err := h.Svc.EarnPoints(ctx, &model.EarnPointsRequest{
		UserID:      target,
		EventType:   req.EventType,
		Context:     ctxMap,
		RelatedType: req.RelatedType,
		RelatedID:   uint(req.RelatedId),
	})
	if err != nil {
		return &pb.EarnPointsResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.EarnPointsResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Points:  res.Points,
		Rules:   res.RuleCodes,
	}, nil
}

// SpendPoints 消费积分（购买付费文章 / 背景），只能为自己消费
func (h *GrpcPointHandler) SpendPoints(ctx context.Context, req *pb.SpendPointsRequest) (*pb.SpendPointsResponse, error) {
	target, err := resolveUser(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	balance, err := h.Svc.SpendPoints(ctx, &model.SpendPointsRequest{
		UserID:   target,
		Amount:   req.Amount,
		ItemType: req.ItemType,
		ItemID:   uint(req.ItemId),
		Remark:   req.Remark,
	})
	if err != nil {
		return &pb.SpendPointsResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.SpendPointsResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Balance: balance,
	}, nil
}

// ============================================================================
// 后台管理
// ============================================================================

func (h *GrpcPointHandler) AdminListRules(ctx context.Context, req *pb.AdminListRulesRequest) (*pb.AdminListRulesResponse, error) {
	if err := requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	rules, err := h.Svc.AdminListRules(ctx)
	if err != nil {
		return &pb.AdminListRulesResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	list := make([]*pb.PointRule, 0, len(rules))
	for _, r := range rules {
		list = append(list, convertToProtoRule(r))
	}
	return &pb.AdminListRulesResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Rules:   list,
	}, nil
}

func (h *GrpcPointHandler) AdminCreateRule(ctx context.Context, req *pb.AdminCreateRuleRequest) (*pb.AdminCreateRuleResponse, error) {
	if err := requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	rule := &model.PointRule{
		Code:       req.Code,
		Name:       req.Name,
		EventType:  req.EventType,
		Condition:  req.Condition,
		Points:     req.Points,
		LimitType:  req.LimitType,
		LimitCount: int(req.LimitCount),
		Sort:       int(req.Sort),
	}
	if err := h.Svc.AdminCreateRule(ctx, rule); err != nil {
		return &pb.AdminCreateRuleResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.AdminCreateRuleResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Rule:    convertToProtoRule(rule),
	}, nil
}

func (h *GrpcPointHandler) AdminUpdateRule(ctx context.Context, req *pb.AdminUpdateRuleRequest) (*pb.AdminUpdateRuleResponse, error) {
	if err := requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	rule := &model.PointRule{
		ID:         uint(req.RuleId),
		Name:       req.Name,
		Condition:  req.Condition,
		Points:     req.Points,
		LimitType:  req.LimitType,
		LimitCount: int(req.LimitCount),
		Status:     uint(req.Status),
		Sort:       int(req.Sort),
	}
	if err := h.Svc.AdminUpdateRule(ctx, rule); err != nil {
		return &pb.AdminUpdateRuleResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.AdminUpdateRuleResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Rule:    convertToProtoRule(rule),
	}, nil
}

func (h *GrpcPointHandler) AdminDeleteRule(ctx context.Context, req *pb.AdminDeleteRuleRequest) (*pb.AdminDeleteRuleResponse, error) {
	if err := requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := h.Svc.AdminDeleteRule(ctx, uint(req.RuleId)); err != nil {
		return &pb.AdminDeleteRuleResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.AdminDeleteRuleResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
	}, nil
}

// AdminAdjustPoints 管理员手动调整积分（纠错 / 运营发放）
func (h *GrpcPointHandler) AdminAdjustPoints(ctx context.Context, req *pb.AdminAdjustPointsRequest) (*pb.AdminAdjustPointsResponse, error) {
	if err := requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	balance, err := h.Svc.AdminAdjustPoints(ctx, uint(req.UserId), req.Amount, req.Remark)
	if err != nil {
		return &pb.AdminAdjustPointsResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.AdminAdjustPointsResponse{
		Code:    uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message: "success",
		Balance: balance,
	}, nil
}

// ============================================================================
// 类型转换
// ============================================================================

func convertToProtoUserPoint(p *model.UserPoint) *pb.UserPoint {
	if p == nil {
		return nil
	}
	return &pb.UserPoint{
		UserId:      uint32(p.UserID),
		Balance:     p.Balance,
		TotalEarned: p.TotalEarned,
		TotalSpent:  p.TotalSpent,
		UpdatedAt:   p.UpdatedAt.Format(constants.DateTimeFormat),
	}
}

func convertToProtoPointLog(l *model.PointLog) *pb.PointLog {
	if l == nil {
		return nil
	}
	return &pb.PointLog{
		Id:           uint32(l.ID),
		UserId:       uint32(l.UserID),
		Amount:       l.Amount,
		BalanceAfter: l.BalanceAfter,
		Type:         l.Type,
		RuleCode:     l.RuleCode,
		RelatedType:  l.RelatedType,
		RelatedId:    uint32(l.RelatedID),
		Remark:       l.Remark,
		CreatedAt:    l.CreatedAt.Format(constants.DateTimeFormat),
	}
}

func convertToProtoRule(r *model.PointRule) *pb.PointRule {
	if r == nil {
		return nil
	}
	return &pb.PointRule{
		Id:         uint32(r.ID),
		Code:       r.Code,
		Name:       r.Name,
		EventType:  r.EventType,
		Condition:  r.Condition,
		Points:     r.Points,
		LimitType:  r.LimitType,
		LimitCount: int32(r.LimitCount),
		Status:     uint32(r.Status),
		Sort:       int32(r.Sort),
		CreatedAt:  r.CreatedAt.Format(constants.DateTimeFormat),
		UpdatedAt:  r.UpdatedAt.Format(constants.DateTimeFormat),
	}
}
