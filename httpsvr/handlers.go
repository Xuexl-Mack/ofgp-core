package httpsvr

import (
	"dgateway/crypto"
	"dgateway/node"
	pb "dgateway/proto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

type httpHandlerFunc func()

const (
	maxRequestContentLen = 1024 * 128
	paramErrCode         = 501
	sysErrCode           = 502
)

// HTTPHandler 提供了HTTP请求的处理函数
type HTTPHandler struct {
	node *node.BraftNode
}

type apiData struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func newData(code int, msg string, data interface{}) apiData {
	return apiData{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}
func newOKData(data interface{}) apiData {
	return apiData{
		Code: 200,
		Msg:  "ok",
		Data: data,
	}
}
func (d apiData) String() string {
	bytes, err := json.Marshal(d)
	if err != nil {
		return `{"code":502,"msg":"json encode err"}`
	}
	return fmt.Sprintf("%s", bytes)
}

// NewHTTPHandler 新建一个HTTPHandler对象并返回
func NewHTTPHandler(node *node.BraftNode) *HTTPHandler {
	return &HTTPHandler{node}
}

func (hd *HTTPHandler) root(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprintf(w, "just a test")
}

// GetBlockHeight 获取当前区块链的高度
func (hd *HTTPHandler) GetBlockHeight(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	height := hd.node.GetBlockHeight()
	res := &getBlockHeightResponse{height}
	fmt.Fprintf(w, "%s", newOKData(res))
}

// GetTxBySidechainTxId 根据公链的交易ID来查询网关对应的交易状态
func (hd *HTTPHandler) GetTxBySidechainTxId(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("txid")
	if len(id) == 0 {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "need param txid", nil))
		return
	}
	tx := hd.node.GetTxBySidechainTxId(id)
	if tx == nil {
		fmt.Fprintf(w, "%s", newData(sysErrCode, "transaction is not found", nil))
		return
	}
	var block *pb.BlockInfo
	if tx.Height > 0 {
		block = hd.node.GetBlockInfo(tx.Height)
	}
	res := toTxInfo(tx.Tx, block)
	writeResponse(&w, newOKData(res))
}

// getBlockByHeight 获取指定高度的区块信息
func (hd *HTTPHandler) getBlockByHeight(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	h := params.ByName("height")
	height, err := strconv.ParseInt(h, 10, 64)
	if err != nil || height < 0 {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "input is not correct", nil))
		return
	}
	block := hd.node.GetBlockInfo(height)
	res := toBlockInfo(block)
	writeResponse(&w, newOKData(res))
}

// getBlockByHeight 获取指定高度的区块信息
func (hd *HTTPHandler) getBlockViewByHeight(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	h := params.ByName("height")
	height, err := strconv.ParseInt(h, 10, 64)
	if err != nil || height < 0 {
		fmt.Fprintf(w, "%s", newData(paramErrCode, err.Error(), nil))
		return
	}
	block := hd.node.GetBlockInfo(height)
	res := toBlockInfo(block)
	writeResponse(&w, newOKData(res))
}

func (hd *HTTPHandler) getBlockTxsBySec(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	startStr := r.FormValue("start")
	endStr := r.FormValue("end")
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		fmt.Fprintf(w, "%s", newData(paramErrCode, err.Error(), nil))
		return
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil || end < 0 {
		fmt.Fprintf(w, "%s", newData(paramErrCode, err.Error(), nil))
		return
	}
	var res []txMap
	for ; start < end; start++ {
		block := hd.node.GetBlockInfo(start)
		for _, tx := range block.Block.Txs {
			res = append(res, txMap{
				FromTxId: tx.WatchedTx.Txid,
				ToTxId:   tx.NewlyTxId,
			})
		}
	}
	writeResponse(&w, newOKData(res))
}

func (hd *HTTPHandler) createBlock(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body := io.LimitReader(r.Body, maxRequestContentLen)
	param := new(fakeBlockInfo)
	err := json.NewDecoder(body).Decode(param)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(paramErrCode, err.Error(), nil))
		return
	}
	currBlock := hd.node.GetBlockCurrent()
	digest, _ := hex.DecodeString(currBlock.ID)
	bp := toBlockPack(param, &crypto.Digest256{Data: digest}, currBlock.Height)
	hd.node.FakeCommitBlock(bp)
	res := fakeBlockResponse{
		Height: bp.Height(),
		Id:     hex.EncodeToString(bp.BlockId().Data),
	}
	writeResponse(&w, newOKData(res))
}

func (hd *HTTPHandler) createTx(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body := io.LimitReader(r.Body, maxRequestContentLen)
	param := new(createTxRequest)
	err := json.NewDecoder(body).Decode(param)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(sysErrCode, err.Error(), nil))
		return
	}
	tx := toWatchedTxInfo(param)
	hd.node.AddWatchedTx(tx)
	w.WriteHeader(200)
}

//获取各个节点
func (hd *HTTPHandler) GetNodes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	nodes := hd.node.GetNodes()
	writeResponse(&w, newOKData(nodes))
}

//获取当前区块
func (hd *HTTPHandler) GetCurrentBlock(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	block := hd.node.GetBlockCurrent()
	writeResponse(&w, newOKData(block))
}
func (hd *HTTPHandler) GetBlocksByHeightSec(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	startStr := req.FormValue("start")
	endStr := req.FormValue("end")
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "start is not num", nil))
		return
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "end is not num", nil))
		return
	}
	if start < 0 || end < 0 || start >= end {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "start,end must > 0 and start must < end", nil))
		return
	}
	blocks := hd.node.GetBlocks(start, end)
	writeResponse(&w, newOKData(blocks))
}

func (hd *HTTPHandler) GetBlockByHeight(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	heightStr := params.ByName("height")
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "height is not number", nil))
		return
	}
	if height < 0 {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "height < 0", nil))
		return
	}
	block := hd.node.GetBlockBytHeight(height)
	writeResponse(&w, newOKData(block))
}
func (hd *HTTPHandler) GetBlockByID(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	id := params.ByName("blockID")
	if id == "" {
		fmt.Fprintf(w, "%s", newData(paramErrCode, "blockID is empty", nil))
	}
	block, err := hd.node.GetBlockByID(id)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(sysErrCode, "get block err", nil))
		return
	}
	writeResponse(&w, newOKData(block))
}
func (hd *HTTPHandler) GetTransActionByID(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
	txID := params.ByName("txid")
	tx := hd.node.GetTransacitonByTxID(txID)
	writeResponse(&w, newOKData(tx))
}

func (hd *HTTPHandler) chainRegister(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	body := io.LimitReader(req.Body, maxRequestContentLen)
	param := new(node.ChainRegInfo)
	err := json.NewDecoder(body).Decode(param)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(paramErrCode, err.Error(), nil))
		return
	}
	hd.node.ChainRegister(param)
	writeResponse(&w, newOKData(nil))
}

func (hd *HTTPHandler) getChainRegisterID(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	newChain := req.FormValue("newchain")
	targetChain := req.FormValue("targetchain")
	chainID := hd.node.GetChainRegisterID(newChain, targetChain)
	writeResponse(&w, newOKData(chainID))
}

// tokenRegister token合约向网关注册
func (hd *HTTPHandler) tokenRegister(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	body := io.LimitReader(req.Body, maxRequestContentLen)
	param := new(node.TokenRegInfo)
	err := json.NewDecoder(body).Decode(param)
	if err != nil {
		fmt.Fprintf(w, "%s", newData(paramErrCode, err.Error(), nil))
		return
	}
	hd.node.TokenRegister(param)
	writeResponse(&w, newOKData(nil))
}

func (hd *HTTPHandler) getTokenRegisterID(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	chain := req.FormValue("chain")
	contractAddr := req.FormValue("contractaddr")
	regID := hd.node.GetTokenRegisterID(chain, contractAddr)
	writeResponse(&w, newOKData(regID))
}

func writeResponse(w *http.ResponseWriter, r interface{}) {
	rst, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(*w, "%s", newData(sysErrCode, err.Error(), nil))
	} else {
		fmt.Fprintf(*w, "%s", rst)
	}
}
