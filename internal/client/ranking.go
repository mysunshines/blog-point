package client

import (
	"context"
	"fmt"

	v0pb "github.com/mysunshines/blog-ranking/proto/pb/v0"
	pb "github.com/mysunshines/blog-ranking/proto/pb/v1"
	"github.com/mysunshines/gocommon/grpcclient"
)

// 积分榜 key（member = 用户 ID，榜单分数 = 用户当前积分余额）
const BoardUserPoints = "board:user:points"

// RegisterUserPointsBoard 声明用户积分榜。
// 装饰器指向 user-service：榜单成员是用户 ID，由 user-service 回填用户名/头像。
// 幂等，可重复调用（服务启动时注册 + 周期重注册）。
func RegisterUserPointsBoard(ctx context.Context) error {
	cfg := &pb.BoardConfig{
		Board:            BoardUserPoints,
		DecoratorType:    "remote",
		DecoratorService: "user-service",
		LinkTemplate:     "/user/{member}",
		CacheTtlSec:      60,
	}
	var resp pb.RegisterBoardResponse
	if err := grpcclient.SendRequest(ctx, v0pb.RankingService_RegisterBoard_FullMethodName,
		&pb.RegisterBoardRequest{Config: cfg}, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("register board %s failed: code=%d message=%s", BoardUserPoints, resp.Code, resp.Message)
	}
	return nil
}

// SyncUserPoints 把用户积分余额同步到积分榜（绝对值覆盖，ZADD）。
// best-effort：榜单是展示侧派生数据，失败只告警，不影响积分主流程。
func SyncUserPoints(ctx context.Context, userID uint, balance int64) error {
	var resp pb.RecordScoreResponse
	if err := grpcclient.SendRequest(ctx, v0pb.RankingService_RecordScore_FullMethodName, &pb.RecordScoreRequest{
		Board:  BoardUserPoints,
		Member: fmt.Sprintf("%d", userID),
		Op:     pb.ScoreOp_SCORE_OP_SET,
		Score:  float64(balance),
	}, &resp); err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("sync user points failed: user=%d code=%d message=%s", userID, resp.Code, resp.Message)
	}
	return nil
}
