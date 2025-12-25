// Package rpc provides a client for communicating with OpenSY nodes via JSON-RPC.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents circuit breaker state
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when circuit is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ClientConfig holds RPC client configuration
type ClientConfig struct {
	URL           string
	User          string
	Password      string
	Timeout       time.Duration
	RetryAttempts int
	RetryDelay    time.Duration
	// Circuit breaker
	CBEnabled      bool
	CBThreshold    int
	CBResetTimeout time.Duration
	Logger         *slog.Logger
}

// DefaultClientConfig returns default configuration
func DefaultClientConfig(url, user, password string) ClientConfig {
	return ClientConfig{
		URL:            url,
		User:           user,
		Password:       password,
		Timeout:        30 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     time.Second,
		CBEnabled:      true,
		CBThreshold:    5,
		CBResetTimeout: 30 * time.Second,
		Logger:         slog.Default(),
	}
}

// Client is a JSON-RPC client for the OpenSY node
type Client struct {
	url      string
	user     string
	password string
	client   *http.Client
	reqID    atomic.Uint64
	logger   *slog.Logger

	// Retry configuration
	retryAttempts int
	retryDelay    time.Duration

	// Circuit breaker
	cbEnabled      bool
	cbState        CircuitState
	cbFailures     int
	cbSuccesses    int
	cbThreshold    int
	cbResetTimeout time.Duration
	cbLastChange   time.Time
	cbMu           sync.Mutex
}

// NewClient creates a new RPC client
func NewClient(url, user, password string) *Client {
	cfg := DefaultClientConfig(url, user, password)
	return NewClientWithConfig(cfg)
}

// NewClientWithConfig creates a new RPC client with config
func NewClientWithConfig(cfg ClientConfig) *Client {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Client{
		url:            cfg.URL,
		user:           cfg.User,
		password:       cfg.Password,
		logger:         cfg.Logger.With("component", "rpc-client"),
		retryAttempts:  cfg.RetryAttempts,
		retryDelay:     cfg.RetryDelay,
		cbEnabled:      cfg.CBEnabled,
		cbState:        CircuitClosed,
		cbThreshold:    cfg.CBThreshold,
		cbResetTimeout: cfg.CBResetTimeout,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Request is a JSON-RPC request
type Request struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      uint64        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params,omitempty"`
}

// Response is a JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Call makes a JSON-RPC call with circuit breaker and retry
func (c *Client) Call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	// Check circuit breaker
	if c.cbEnabled && !c.cbAllow() {
		return ErrCircuitOpen
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryDelay * time.Duration(attempt)):
			}
		}

		err := c.doCall(ctx, method, params, result)
		if err == nil {
			c.cbRecordSuccess()
			return nil
		}

		lastErr = err
		c.logger.Warn("RPC call failed", "method", method, "attempt", attempt+1, "error", err)
	}

	c.cbRecordFailure()
	return lastErr
}

// doCall performs the actual RPC call
func (c *Client) doCall(ctx context.Context, method string, params []interface{}, result interface{}) error {
	req := Request{
		JSONRPC: "2.0",
		ID:      c.reqID.Add(1),
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.user != "" {
		httpReq.SetBasicAuth(c.user, c.password)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rpcResp Response
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return rpcResp.Error
	}

	if result != nil && rpcResp.Result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// Circuit breaker methods
func (c *Client) cbAllow() bool {
	c.cbMu.Lock()
	defer c.cbMu.Unlock()

	switch c.cbState {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(c.cbLastChange) >= c.cbResetTimeout {
			c.cbState = CircuitHalfOpen
			c.logger.Info("Circuit breaker half-open")
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

func (c *Client) cbRecordSuccess() {
	if !c.cbEnabled {
		return
	}
	c.cbMu.Lock()
	defer c.cbMu.Unlock()

	switch c.cbState {
	case CircuitHalfOpen:
		c.cbSuccesses++
		if c.cbSuccesses >= c.cbThreshold {
			c.cbState = CircuitClosed
			c.cbFailures = 0
			c.cbSuccesses = 0
			c.logger.Info("Circuit breaker closed")
		}
	case CircuitClosed:
		c.cbFailures = 0
	}
}

func (c *Client) cbRecordFailure() {
	if !c.cbEnabled {
		return
	}
	c.cbMu.Lock()
	defer c.cbMu.Unlock()

	switch c.cbState {
	case CircuitHalfOpen:
		c.cbState = CircuitOpen
		c.cbLastChange = time.Now()
		c.logger.Warn("Circuit breaker opened (half-open failed)")
	case CircuitClosed:
		c.cbFailures++
		if c.cbFailures >= c.cbThreshold {
			c.cbState = CircuitOpen
			c.cbLastChange = time.Now()
			c.logger.Warn("Circuit breaker opened", "failures", c.cbFailures)
		}
	}
}

// CircuitState returns current circuit breaker state
func (c *Client) CircuitState() CircuitState {
	c.cbMu.Lock()
	defer c.cbMu.Unlock()
	return c.cbState
}

// BlockTemplate represents a block template from getblocktemplate
type BlockTemplate struct {
	Version              int          `json:"version"`
	PreviousBlockHash    string       `json:"previousblockhash"`
	Target               string       `json:"target"`
	Bits                 string       `json:"bits"`
	Height               int64        `json:"height"`
	CurTime              int64        `json:"curtime"`
	SigOpLimit           int          `json:"sigoplimit"`
	SizeLimit            int          `json:"sizelimit"`
	WeightLimit          int          `json:"weightlimit"`
	Transactions         []TxTemplate `json:"transactions"`
	CoinbaseAux          CoinbaseAux  `json:"coinbaseaux"`
	CoinbaseValue        int64        `json:"coinbasevalue"`
	LongPollID           string       `json:"longpollid,omitempty"`
	Mutable              []string     `json:"mutable"`
	NonceRange           string       `json:"noncerange"`
	SeedHash             string       `json:"seedhash"`               // RandomX seed hash
	NextSeedHash         string       `json:"nextseedhash,omitempty"` // Next seed hash if changing soon
	MinTime              int64        `json:"mintime"`
	DefaultWitnessCommit string       `json:"default_witness_commitment,omitempty"`
}

// TxTemplate is a transaction in the block template
type TxTemplate struct {
	Data    string `json:"data"`
	TxID    string `json:"txid"`
	Hash    string `json:"hash"`
	Depends []int  `json:"depends"`
	Fee     int64  `json:"fee"`
	SigOps  int    `json:"sigops"`
	Weight  int    `json:"weight"`
}

// CoinbaseAux contains auxiliary data for coinbase
type CoinbaseAux struct {
	Flags string `json:"flags"`
}

// GetBlockTemplate fetches a block template from the node
func (c *Client) GetBlockTemplate(ctx context.Context) (*BlockTemplate, error) {
	params := []interface{}{
		map[string]interface{}{
			"rules": []string{"segwit"},
		},
	}

	var template BlockTemplate
	if err := c.Call(ctx, "getblocktemplate", params, &template); err != nil {
		return nil, err
	}

	return &template, nil
}

// SubmitBlock submits a solved block to the network
func (c *Client) SubmitBlock(ctx context.Context, blockHex string) error {
	var result interface{}
	if err := c.Call(ctx, "submitblock", []interface{}{blockHex}, &result); err != nil {
		return err
	}

	// submitblock returns null on success, or a string on failure
	if result != nil {
		if errStr, ok := result.(string); ok && errStr != "" {
			return fmt.Errorf("block rejected: %s", errStr)
		}
	}

	return nil
}

// BlockchainInfo contains blockchain information
type BlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	BestBlockHash        string  `json:"bestblockhash"`
	Difficulty           float64 `json:"difficulty"`
	MedianTime           int64   `json:"mediantime"`
	VerificationProgress float64 `json:"verificationprogress"`
	InitialBlockDownload bool    `json:"initialblockdownload"`
	Chainwork            string  `json:"chainwork"`
	SizeOnDisk           int64   `json:"size_on_disk"`
	Pruned               bool    `json:"pruned"`
	Warnings             string  `json:"warnings"`
}

// GetBlockchainInfo returns blockchain state information
func (c *Client) GetBlockchainInfo(ctx context.Context) (*BlockchainInfo, error) {
	var info BlockchainInfo
	if err := c.Call(ctx, "getblockchaininfo", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetBestBlockHash returns the hash of the best (tip) block
func (c *Client) GetBestBlockHash(ctx context.Context) (string, error) {
	var hash string
	if err := c.Call(ctx, "getbestblockhash", nil, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

// GetBlockHash returns the hash of the block at the given height
func (c *Client) GetBlockHash(ctx context.Context, height int64) (string, error) {
	var hash string
	if err := c.Call(ctx, "getblockhash", []interface{}{height}, &hash); err != nil {
		return "", err
	}
	return hash, nil
}

// Block represents a block from getblock
type Block struct {
	Hash              string   `json:"hash"`
	Confirmations     int      `json:"confirmations"`
	Height            int64    `json:"height"`
	Version           int      `json:"version"`
	VersionHex        string   `json:"versionHex"`
	Merkleroot        string   `json:"merkleroot"`
	Time              int64    `json:"time"`
	MedianTime        int64    `json:"mediantime"`
	Nonce             uint64   `json:"nonce"`
	Bits              string   `json:"bits"`
	Difficulty        float64  `json:"difficulty"`
	Chainwork         string   `json:"chainwork"`
	NTx               int      `json:"nTx"`
	PreviousBlockHash string   `json:"previousblockhash,omitempty"`
	NextBlockHash     string   `json:"nextblockhash,omitempty"`
	Tx                []string `json:"tx"`
}

// GetBlock returns block information (verbosity 1)
func (c *Client) GetBlock(ctx context.Context, hash string) (*Block, error) {
	var block Block
	if err := c.Call(ctx, "getblock", []interface{}{hash, 1}, &block); err != nil {
		return nil, err
	}
	return &block, nil
}

// GetBlockByHeight returns block information for a given height
func (c *Client) GetBlockByHeight(ctx context.Context, height int64) (*Block, error) {
	hash, err := c.GetBlockHash(ctx, height)
	if err != nil {
		return nil, err
	}
	return c.GetBlock(ctx, hash)
}

// NetworkInfo contains network information
type NetworkInfo struct {
	Version         int    `json:"version"`
	Subversion      string `json:"subversion"`
	ProtocolVersion int    `json:"protocolversion"`
	LocalServices   string `json:"localservices"`
	LocalRelay      bool   `json:"localrelay"`
	TimeOffset      int    `json:"timeoffset"`
	Connections     int    `json:"connections"`
	NetworkActive   bool   `json:"networkactive"`
	Warnings        string `json:"warnings"`
}

// GetNetworkInfo returns network information
func (c *Client) GetNetworkInfo(ctx context.Context) (*NetworkInfo, error) {
	var info NetworkInfo
	if err := c.Call(ctx, "getnetworkinfo", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// MiningInfo contains mining information
type MiningInfo struct {
	Blocks             int64   `json:"blocks"`
	CurrentBlockWeight int64   `json:"currentblockweight"`
	CurrentBlockTx     int64   `json:"currentblocktx"`
	Difficulty         float64 `json:"difficulty"`
	NetworkHashPS      float64 `json:"networkhashps"`
	PooledTx           int     `json:"pooledtx"`
	Chain              string  `json:"chain"`
	Warnings           string  `json:"warnings"`
}

// GetMiningInfo returns mining information
func (c *Client) GetMiningInfo(ctx context.Context) (*MiningInfo, error) {
	var info MiningInfo
	if err := c.Call(ctx, "getmininginfo", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetDifficulty returns the current mining difficulty
func (c *Client) GetDifficulty(ctx context.Context) (float64, error) {
	var difficulty float64
	if err := c.Call(ctx, "getdifficulty", nil, &difficulty); err != nil {
		return 0, err
	}
	return difficulty, nil
}

// ValidateAddress validates an address
type AddressInfo struct {
	IsValid      bool   `json:"isvalid"`
	Address      string `json:"address"`
	ScriptPubKey string `json:"scriptPubKey"`
	IsScript     bool   `json:"isscript"`
	IsWitness    bool   `json:"iswitness"`
}

// ValidateAddress validates a wallet address
func (c *Client) ValidateAddress(ctx context.Context, address string) (*AddressInfo, error) {
	var info AddressInfo
	if err := c.Call(ctx, "validateaddress", []interface{}{address}, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// GetConnectionCount returns the number of peers
func (c *Client) GetConnectionCount(ctx context.Context) (int, error) {
	var count int
	if err := c.Call(ctx, "getconnectioncount", nil, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// Ping checks if the node is responsive
func (c *Client) Ping(ctx context.Context) error {
	return c.Call(ctx, "ping", nil, nil)
}

// GetBlockCount returns the current block height
func (c *Client) GetBlockCount(ctx context.Context) (int64, error) {
	var count int64
	if err := c.Call(ctx, "getblockcount", nil, &count); err != nil {
		return 0, err
	}
	return count, nil
}
