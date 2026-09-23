package v0

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mysunshines/blog-point/internal/model"
	"github.com/mysunshines/blog-point/internal/service"
	v0pb "github.com/mysunshines/blog-point/proto/pb/v0"
	pb "github.com/mysunshines/blog-point/proto/pb/v1"
)

// 摄入接口（EarnPoints / SpendPoints / HasPurchased）仅由内网受信服务经 Consul
// 直连 gRPC 调用。已决定完全信任内网：不做令牌 / JWT 鉴权，仅依赖网络隔离
// （point 的 gRPC 端口不暴露公网），故 handler 内不再做任何调用方校验。
//
// 对外暴露为 point.v0.PointIngestService，与 C 端 v1 接口在命名与部署上隔离。
// 公网网关 DeriveGRPCService 仅硬编码 v1（prefix.v1.PrefixService），因此本 v0 服务
// 天然不会经 Gateway 反射代理暴露，仅由内网业务服务直连（纵深防御）。

// GrpcPointHandler gRPC 摄入接口处理器（point.v0.PointIngestService）。
type GrpcPointHandler struct {
	v0pb.UnimplementedPointIngestServiceServer
	Svc service.PointService
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

// EarnPoints 内部摄入接口：推送积分事件（完全信任内网，不做调用方校验）。
func (h *GrpcPointHandler) EarnPoints(ctx context.Context, req *pb.EarnPointsRequest) (*pb.EarnPointsResponse, error) {
	if req == nil || req.UserId == 0 || req.EventType == "" {
		return &pb.EarnPointsResponse{Code: uint32(pb.PointErrorCode_POINT_BAD_REQUEST), Message: "user_id and event_type required"}, nil
	}
	var ctxMap map[string]interface{}
	if req.Context != "" {
		if err := json.Unmarshal([]byte(req.Context), &ctxMap); err != nil {
			return &pb.EarnPointsResponse{Code: uint32(pb.PointErrorCode_POINT_BAD_REQUEST), Message: "context 不是合法 JSON"}, nil
		}
	}
	res, err := h.Svc.EarnPoints(ctx, &model.EarnPointsRequest{
		UserID:         uint(req.UserId),
		EventType:      req.EventType,
		Context:        ctxMap,
		RelatedType:    req.RelatedType,
		RelatedID:      uint(req.RelatedId),
		IdempotencyKey: req.IdempotencyKey,
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

// SpendPoints 内部摄入接口：消费积分（完全信任内网，不做调用方校验）。
func (h *GrpcPointHandler) SpendPoints(ctx context.Context, req *pb.SpendPointsRequest) (*pb.SpendPointsResponse, error) {
	if req == nil || req.UserId == 0 || req.Amount <= 0 || req.ItemType == "" || req.ItemId == 0 {
		return &pb.SpendPointsResponse{Code: uint32(pb.PointErrorCode_POINT_BAD_REQUEST), Message: "invalid request"}, nil
	}
	balance, err := h.Svc.SpendPoints(ctx, &model.SpendPointsRequest{
		UserID:         uint(req.UserId),
		Amount:         req.Amount,
		ItemType:       req.ItemType,
		ItemID:         uint(req.ItemId),
		Remark:         req.Remark,
		IdempotencyKey: req.IdempotencyKey,
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

// HasPurchased 内部摄入接口：查询购买状态（完全信任内网，不做调用方校验）。
func (h *GrpcPointHandler) HasPurchased(ctx context.Context, req *pb.HasPurchasedRequest) (*pb.HasPurchasedResponse, error) {
	if req == nil || req.UserId == 0 || req.ItemType == "" || req.ItemId == 0 {
		return &pb.HasPurchasedResponse{Code: uint32(pb.PointErrorCode_POINT_BAD_REQUEST), Message: "invalid request"}, nil
	}
	ok, err := h.Svc.HasPurchased(ctx, uint(req.UserId), req.ItemType, uint(req.ItemId))
	if err != nil {
		return &pb.HasPurchasedResponse{Code: errCode(err), Message: err.Error()}, nil
	}
	return &pb.HasPurchasedResponse{
		Code:      uint32(pb.PointErrorCode_POINT_SUCCESS),
		Message:   "success",
		Purchased: ok,
	}, nil
}
