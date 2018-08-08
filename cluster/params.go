package cluster

import (
	"dgateway/crypto"
	pb "dgateway/proto"
	"dgateway/util/assert"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

// TODO 配置化
var (
	ClusterSize      int
	TotalNodeCount   int
	QuorumN          int
	AccuseQuorumN    int
	MaxFaultyN       int
	NodeList         []NodeInfo
	DbCache          = 16
	DbFileHandles    = 16
	NodeSigners      map[int32]*crypto.SecureSigner
	MultiSigSnapshot []MultiSigInfo
	CurrMultiSig     MultiSigInfo
)

const (
	// BlockInterval 产出区块的频率
	BlockInterval = 10 * time.Second
)

// Init 初始化集群信息
func Init() {
	n := viper.GetInt("KEYSTORE.count")
	localPubkeyHash := viper.GetString("KEYSTORE.local_pubkey_hash")
	localId := viper.GetInt("DGW.local_id")
	NodeSigners = make(map[int32]*crypto.SecureSigner, n)
	NodeList = nil
	for i := 0; i < n; i++ {
		hash := ""
		if i == localId {
			hash = localPubkeyHash
		}
		key := "KEYSTORE.key_" + strconv.Itoa(i)
		pubkey := viper.GetString(key)
		NodeSigners[int32(i)] = crypto.NewSecureSigner(pubkey, hash)

		NodeList = append(NodeList, NodeInfo{
			Id:        int32(i),
			Name:      fmt.Sprintf("server%d", i),
			Url:       viper.GetString("DGW.host_" + strconv.Itoa(i)),
			PublicKey: NodeSigners[int32(i)].Pubkey,
			IsNormal:  viper.GetBool("DGW.status_" + strconv.Itoa(i)),
		})
	}
	ClusterSize = n
	TotalNodeCount = n
	MaxFaultyN = (ClusterSize - 1) / 3
	QuorumN = (ClusterSize+MaxFaultyN)/2 + 1
	AccuseQuorumN = MaxFaultyN + 1
}

// InitWithNodeList 根据参数而非配置文件来构建集群信息
func InitWithNodeList(nodeList *pb.NodeList) {
	NodeSigners = make(map[int32]*crypto.SecureSigner)
	for i, node := range nodeList.NodeList {
		assert.True(int32(i) == node.NodeId)
		NodeSigners[int32(i)] = crypto.NewSecureSigner(node.Pubkey, "")

		NodeList = append(NodeList, NodeInfo{
			Id:        node.NodeId,
			Name:      node.Name,
			Url:       node.Host,
			PublicKey: NodeSigners[int32(i)].Pubkey,
			IsNormal:  node.IsNormal,
		})
		if node.IsNormal {
			ClusterSize++
		}
	}
	TotalNodeCount = len(nodeList.NodeList)
	MaxFaultyN = (ClusterSize - 1) / 3
	QuorumN = (ClusterSize+MaxFaultyN)/2 + 1
	AccuseQuorumN = MaxFaultyN + 1
}

// LeaderNodeOfTerm 根据指定的term计算leader的ID并返回
func LeaderNodeOfTerm(term int64) int32 {
	idx := int32(term % int64(TotalNodeCount))
	for {
		if NodeList[idx].IsNormal {
			return idx
		}
		idx = int32((idx + 1) % int32(TotalNodeCount))
	}
}

// AddNode 增加节点到集群信息列表
func AddNode(host string, nodeId int32, pubkey string, pubkeyHash string) {
	if nodeId != int32(TotalNodeCount) {
		return
	}
	signer := crypto.NewSecureSigner(pubkey, pubkeyHash)
	NodeList = append(NodeList, NodeInfo{
		Id:        nodeId,
		Name:      fmt.Sprintf("server%d", nodeId),
		Url:       host,
		PublicKey: signer.Pubkey,
		IsNormal:  true,
	})
	NodeSigners[nodeId] = signer
	ClusterSize++
	TotalNodeCount++
	MaxFaultyN = (ClusterSize - 1) / 3
	QuorumN = (ClusterSize+MaxFaultyN)/2 + 1
	AccuseQuorumN = MaxFaultyN + 1
}

// DeleteNode 把相应ID的节点的状态标记为删除
func DeleteNode(nodeId int32) {
	if nodeId >= int32(TotalNodeCount) || nodeId < 0 {
		return
	}
	if NodeList[nodeId].IsNormal {
		NodeList[nodeId].IsNormal = false
		ClusterSize--
		MaxFaultyN = (ClusterSize - 1) / 3
		QuorumN = (ClusterSize+MaxFaultyN)/2 + 1
		AccuseQuorumN = MaxFaultyN + 1
	}
}

// RecoverNode 恢复节点的状态为正常
func RecoverNode(nodeId int32) {
	if nodeId >= int32(TotalNodeCount) || nodeId < 0 {
		return
	}
	NodeList[nodeId].IsNormal = true
	ClusterSize++
	MaxFaultyN = (ClusterSize - 1) / 3
	QuorumN = (ClusterSize+MaxFaultyN)/2 + 1
	AccuseQuorumN = MaxFaultyN + 1
}

// GetPubkeyList 获取所有正常状态节点的公钥
func GetPubkeyList() []string {
	var rst []string
	for _, nodeInfo := range NodeList {
		if nodeInfo.IsNormal {
			rst = append(rst, hex.EncodeToString(nodeInfo.PublicKey))
		}
	}
	return rst
}

// AddMultiSigInfo 保存当前多签地址到快照列表
func AddMultiSigInfo(multiSig MultiSigInfo) {
	if len(MultiSigSnapshot) > 0 && MultiSigSnapshot[len(MultiSigSnapshot)-1].Equal(multiSig) {
		return
	}
	MultiSigSnapshot = append(MultiSigSnapshot, multiSig)
}

// SetMultiSigSnapshot 设置快照
func SetMultiSigSnapshot(snapshot []MultiSigInfo) {
	MultiSigSnapshot = snapshot
}

// SetCurrMultiSig 设置当前的多签信息
func SetCurrMultiSig(multiSig MultiSigInfo) {
	CurrMultiSig = multiSig
}
