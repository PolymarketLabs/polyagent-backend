package executor

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	_ "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/shopspring/decimal"
)

// PolymarketClient Polymarket API客户端
type PolymarketClient struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	passphrase string
	httpClient *http.Client
	privateKey *ecdsa.PrivateKey
}

// NewPolymarketClient 创建客户端
func NewPolymarketClient(baseURL, apiKey, apiSecret, passphrase, privateKeyHex string) (*PolymarketClient, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	return &PolymarketClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		passphrase: passphrase,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		privateKey: privateKey,
	}, nil
}

// Market 市场信息
type Market struct {
	ID          string          `json:"id"`
	Question    string          `json:"question"`
	Description string          `json:"description"`
	EndDate     time.Time       `json:"end_date"`
	Active      bool            `json:"active"`
	Closed      bool            `json:"closed"`
	Outcomes    []Outcome       `json:"outcomes"`
	BestBid     decimal.Decimal `json:"best_bid"`
	BestAsk     decimal.Decimal `json:"best_ask"`
	LastPrice   decimal.Decimal `json:"last_price"`
	Volume      decimal.Decimal `json:"volume"`
	Liquidity   decimal.Decimal `json:"liquidity"`
}

// Outcome 预测结果
type Outcome struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Price decimal.Decimal `json:"price"`
}

// OrderRequest 下单请求
type OrderRequest struct {
	MarketID   string          `json:"market_id"`
	OutcomeID  string          `json:"outcome_id"`
	Side       string          `json:"side"` // BUY or SELL
	Size       decimal.Decimal `json:"size"`
	Price      decimal.Decimal `json:"price"` // 0表示市价单
	OrderType  string          `json:"order_type"`
	Nonce      int64           `json:"nonce"`
	Expiration int64           `json:"expiration"`
}

// OrderResponse 下单响应
type OrderResponse struct {
	OrderID       string          `json:"order_id"`
	Status        string          `json:"status"`
	FilledSize    decimal.Decimal `json:"filled_size"`
	AvgFillPrice  decimal.Decimal `json:"avg_fill_price"`
	RemainingSize decimal.Decimal `json:"remaining_size"`
	TransactionID string          `json:"transaction_id"`
	Error         string          `json:"error,omitempty"`
}

// GetMarket 获取市场信息
func (c *PolymarketClient) GetMarket(ctx context.Context, marketID string) (*Market, error) {
	url := fmt.Sprintf("%s/markets/%s", c.baseURL, marketID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.generateAuthHeader("GET", "/markets/"+marketID, ""))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API错误: %s", string(body))
	}

	var market Market
	if err := json.NewDecoder(resp.Body).Decode(&market); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &market, nil
}

// PlaceOrder 下单
func (c *PolymarketClient) PlaceOrder(ctx context.Context, req OrderRequest) (*OrderResponse, error) {
	// 签名订单
	signature, err := c.signOrder(req)
	if err != nil {
		return nil, fmt.Errorf("签名订单失败: %w", err)
	}

	// 构建请求体
	body := map[string]interface{}{
		"market_id":  req.MarketID,
		"outcome_id": req.OutcomeID,
		"side":       req.Side,
		"size":       req.Size.String(),
		"price":      req.Price.String(),
		"order_type": req.OrderType,
		"nonce":      req.Nonce,
		"expiration": req.Expiration,
		"signature":  signature,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/orders", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.generateAuthHeader("POST", "/orders", string(jsonBody)))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("下单请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("下单失败: %s", string(respBody))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &orderResp, nil
}

// CancelOrder 撤单
func (c *PolymarketClient) CancelOrder(ctx context.Context, orderID string) error {
	url := fmt.Sprintf("%s/orders/%s", c.baseURL, orderID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", c.generateAuthHeader("DELETE", "/orders/"+orderID, ""))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("撤单请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("撤单失败: %s", string(body))
	}

	return nil
}

// GetPositions 获取持仓
func (c *PolymarketClient) GetPositions(ctx context.Context, address string) ([]Position, error) {
	url := fmt.Sprintf("%s/positions?address=%s", c.baseURL, address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.generateAuthHeader("GET", "/positions?address="+address, ""))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API错误: %s", string(body))
	}

	var positions []Position
	if err := json.NewDecoder(resp.Body).Decode(&positions); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return positions, nil
}

// Position 持仓信息
type Position struct {
	MarketID      string          `json:"market_id"`
	OutcomeID     string          `json:"outcome_id"`
	Size          decimal.Decimal `json:"size"`
	AvgPrice      decimal.Decimal `json:"avg_price"`
	UnrealizedPnL decimal.Decimal `json:"unrealized_pnl"`
}

// signOrder EIP-712签名订单
func (c *PolymarketClient) signOrder(req OrderRequest) (string, error) {
	// 构建EIP-712类型数据
	domain := apitypes.TypedDataDomain{
		Name:              "Polymarket",
		Version:           "1",
		ChainId:           (*math.HexOrDecimal256)(big.NewInt(137)), // Polygon主网
		VerifyingContract: "0x...",
	}

	types := apitypes.Types{
		"Order": []apitypes.Type{
			{Name: "market", Type: "address"},
			{Name: "outcome", Type: "uint256"},
			{Name: "side", Type: "uint8"},
			{Name: "size", Type: "uint256"},
			{Name: "price", Type: "uint256"},
			{Name: "nonce", Type: "uint256"},
			{Name: "expiration", Type: "uint256"},
		},
	}

	message := map[string]interface{}{
		"market":     req.MarketID,
		"outcome":    req.OutcomeID,
		"side":       map[string]uint8{"BUY": 0, "SELL": 1}[req.Side],
		"size":       req.Size.Shift(6).BigInt(), // 6位小数
		"price":      req.Price.Shift(6).BigInt(),
		"nonce":      big.NewInt(req.Nonce),
		"expiration": big.NewInt(req.Expiration),
	}

	typedData := apitypes.TypedData{
		Types:       types,
		PrimaryType: "Order",
		Domain:      domain,
		Message:     message,
	}

	// 签名
	signature, err := c.signTypedData(typedData)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(signature), nil
}

// signTypedData 签名类型数据
func (c *PolymarketClient) signTypedData(typedData apitypes.TypedData) ([]byte, error) {
	// 简化实现，实际应使用完整的EIP-712签名流程
	// 这里使用ethers.js或go-ethereum的完整实现
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, err
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return nil, err
	}

	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	hash := crypto.Keccak256(rawData)

	return crypto.Sign(hash, c.privateKey)
}

// generateAuthHeader 生成认证头
func (c *PolymarketClient) generateAuthHeader(method, path, body string) string {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	//message := timestamp + method + path + body
	// HMAC-SHA256签名
	// signature := hmacSha256(c.apiSecret, message)
	return fmt.Sprintf("PFX-HMAC-SHA256 %s:%s:%s", c.apiKey, timestamp, "signature")
}
