package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"polyagent-backend/internal/models"
	"polyagent-backend/internal/pkg/logger"
	"polyagent-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// Executor 交易执行器
type Executor struct {
	repo     repository.Repository
	pmClient *PolymarketClient
	logger   *logger.Logger

	// 执行配置
	maxRetries    int
	retryInterval time.Duration

	// 异步任务队列
	taskQueue chan *ExecutionTask
	workers   int
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

// ExecutionTask 执行任务
type ExecutionTask struct {
	IntentID uuid.UUID
	Retries  int
}

// NewExecutor 创建执行器
func NewExecutor(repo repository.Repository, pmClient *PolymarketClient,
	logger *logger.Logger, workers int) *Executor {
	return &Executor{
		repo:          repo,
		pmClient:      pmClient,
		logger:        logger,
		maxRetries:    3,
		retryInterval: 5 * time.Second,
		taskQueue:     make(chan *ExecutionTask, 1000),
		workers:       workers,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动执行器
func (e *Executor) Start(ctx context.Context) {
	e.logger.Info("启动交易执行器", zap.Int("workers", e.workers))

	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}
}

// Stop 停止执行器
func (e *Executor) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	e.logger.Info("交易执行器已停止")
}

// SubmitTask 提交执行任务
func (e *Executor) SubmitTask(intentID uuid.UUID) {
	select {
	case e.taskQueue <- &ExecutionTask{IntentID: intentID}:
		e.logger.Debug("任务已加入队列", zap.String("intent_id", intentID.String()))
	default:
		e.logger.Error("任务队列已满", zap.String("intent_id", intentID.String()))
	}
}

// worker 工作协程
func (e *Executor) worker(ctx context.Context, id int) {
	defer e.wg.Done()
	e.logger.Info("执行器工作协程启动", zap.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case task := <-e.taskQueue:
			if err := e.executeTask(ctx, task); err != nil {
				e.logger.Error("任务执行失败",
					zap.String("intent_id", task.IntentID.String()),
					zap.Error(err))

				// 重试逻辑
				if task.Retries < e.maxRetries {
					task.Retries++
					time.Sleep(e.retryInterval)
					e.SubmitTask(task.IntentID)
				} else {
					e.failIntent(ctx, task.IntentID, fmt.Sprintf("重试%d次后失败", e.maxRetries))
				}
			}
		}
	}
}

// executeTask 执行任务
func (e *Executor) executeTask(ctx context.Context, task *ExecutionTask) error {
	// 获取意图
	intent, err := e.repo.GetTradeIntent(ctx, task.IntentID)
	if err != nil {
		return fmt.Errorf("获取交易意图失败: %w", err)
	}

	// 检查状态
	if intent.Status != models.IntentStatusApproved {
		return fmt.Errorf("意图状态不正确: %s", intent.Status)
	}

	// 更新为执行中
	intent.Status = models.IntentStatusExecuting
	if err := e.repo.UpdateTradeIntent(ctx, intent); err != nil {
		return fmt.Errorf("更新状态失败: %w", err)
	}

	// 获取当前市场价格
	market, err := e.pmClient.GetMarket(ctx, intent.MarketID)
	if err != nil {
		return fmt.Errorf("获取市场信息失败: %w", err)
	}

	// 确定执行价格
	executionPrice := intent.Price
	if executionPrice.IsZero() {
		// 市价单使用当前最优价格
		if intent.Side == models.TradeSideBuy {
			executionPrice = market.BestAsk
		} else {
			executionPrice = market.BestBid
		}
	}

	// 构建订单请求
	orderReq := OrderRequest{
		MarketID:   intent.MarketID,
		OutcomeID:  intent.OutcomeID,
		Side:       string(intent.Side),
		Size:       intent.Size,
		Price:      executionPrice,
		OrderType:  intent.OrderType,
		Nonce:      time.Now().UnixNano(),
		Expiration: time.Now().Add(5 * time.Minute).Unix(),
	}

	// 执行下单
	e.logger.Info("执行交易",
		zap.String("intent_id", intent.ID.String()),
		zap.String("market_id", intent.MarketID),
		zap.String("side", string(intent.Side)),
		zap.String("size", intent.Size.String()),
		zap.String("price", executionPrice.String()))

	orderResp, err := e.pmClient.PlaceOrder(ctx, orderReq)
	if err != nil {
		return fmt.Errorf("下单失败: %w", err)
	}

	// 检查订单结果
	if orderResp.Error != "" {
		return fmt.Errorf("订单错误: %s", orderResp.Error)
	}

	// 更新意图状态为完成
	now := time.Now()
	intent.Status = models.IntentStatusCompleted
	intent.ExecutedTx = orderResp.TransactionID
	intent.ExecutedPrice = orderResp.AvgFillPrice
	intent.ExecutedAt = &now

	if err := e.repo.UpdateTradeIntent(ctx, intent); err != nil {
		e.logger.Error("更新意图完成状态失败", zap.Error(err))
	}

	// 更新持仓
	if err := e.updatePosition(ctx, intent, orderResp); err != nil {
		e.logger.Error("更新持仓失败", zap.Error(err))
	}

	e.logger.Info("交易执行完成",
		zap.String("intent_id", intent.ID.String()),
		zap.String("tx_id", orderResp.TransactionID),
		zap.String("avg_price", orderResp.AvgFillPrice.String()))

	return nil
}

// updatePosition 更新持仓
func (e *Executor) updatePosition(ctx context.Context, intent *models.TradeIntent, resp *OrderResponse) error {
	// 查找现有持仓
	position, err := e.repo.GetPosition(ctx, intent.FundID, intent.MarketID, intent.OutcomeID)
	if err != nil {
		// 创建新持仓
		position = &models.Position{
			FundID:     intent.FundID,
			MarketID:   intent.MarketID,
			OutcomeID:  intent.OutcomeID,
			Size:       decimal.Zero,
			EntryPrice: decimal.Zero,
		}
	}

	// 计算新持仓
	if intent.Side == models.TradeSideBuy {
		position.Size = position.Size.Add(resp.FilledSize)
	} else {
		position.Size = position.Size.Sub(resp.FilledSize)
	}

	// 更新平均成本价
	if !position.Size.IsZero() {
		totalCost := position.EntryPrice.Mul(position.Size.Abs()).Add(
			resp.AvgFillPrice.Mul(resp.FilledSize))
		position.EntryPrice = totalCost.Div(position.Size.Abs())
	}

	position.CurrentPrice = resp.AvgFillPrice
	position.LastUpdated = time.Now()

	return e.repo.SavePosition(ctx, position)
}

// failIntent 标记意图失败
func (e *Executor) failIntent(ctx context.Context, intentID uuid.UUID, reason string) {
	intent, err := e.repo.GetTradeIntent(ctx, intentID)
	if err != nil {
		e.logger.Error("获取意图失败", zap.Error(err))
		return
	}

	intent.Status = models.IntentStatusFailed
	intent.RejectReason = reason
	if err := e.repo.UpdateTradeIntent(ctx, intent); err != nil {
		e.logger.Error("更新失败状态失败", zap.Error(err))
	}
}

// ExecuteStopLoss 执行止损平仓（供实时风控调用）
func (e *Executor) ExecuteStopLoss(ctx context.Context, position models.Position) error {
	e.logger.Warn("执行止损平仓",
		zap.String("fund_id", position.FundID.String()),
		zap.String("market_id", position.MarketID),
		zap.String("size", position.Size.String()))

	// 创建平仓意图
	closeIntent := &models.TradeIntent{
		FundID:    position.FundID,
		ManagerID: uuid.Nil, // 系统执行
		MarketID:  position.MarketID,
		OutcomeID: position.OutcomeID,
		Side:      e.getOppositeSide(position.Size),
		Size:      position.Size.Abs(),
		Price:     decimal.Zero, // 市价平仓
		OrderType: "MARKET",
		Status:    models.IntentStatusApproved, // 直接通过，跳过审计
	}

	if err := e.repo.CreateTradeIntent(ctx, closeIntent); err != nil {
		return fmt.Errorf("创建平仓意图失败: %w", err)
	}

	// 直接执行，不经过队列
	task := &ExecutionTask{IntentID: closeIntent.ID}
	return e.executeTask(ctx, task)
}

// getOppositeSide 获取相反方向
func (e *Executor) getOppositeSide(size decimal.Decimal) models.TradeSide {
	if size.GreaterThan(decimal.Zero) {
		return models.TradeSideSell
	}
	return models.TradeSideBuy
}
