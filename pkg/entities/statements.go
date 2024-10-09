package entities

import (
	"math"
	"sort"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type NodeStatus string

const (
	OK            NodeStatus = "OK"
	Incomplete    NodeStatus = "incomplete"
	Unreachable   NodeStatus = "unreachable"
	InvalidHeight NodeStatus = "invalid_height"
)

type NodeStatement struct {
	Node       string              `json:"node"`
	Timestamp  int64               `json:"timestamp"`
	Status     NodeStatus          `json:"status"`
	Version    string              `json:"version,omitempty"`
	Height     uint64              `json:"height,omitempty"`
	StateHash  *proto.StateHash    `json:"state_hash,omitempty"`
	BaseTarget uint64              `json:"base_target,omitempty"`
	BlockID    *proto.BlockID      `json:"block_id,omitempty"`
	Generator  *proto.WavesAddress `json:"generator,omitempty"`
	Challenged bool                `json:"challenged,omitempty"`
}

type Nodes []string

func (n Nodes) Sort() Nodes {
	sort.Strings(n)
	return n
}

type NodeStatements []NodeStatement

type NodeStatementsWithMinHeight struct {
	Statements NodeStatements
	MinHeight  int
}

type (
	NodeStatementsSplitByStatus       map[NodeStatus]NodeStatements
	NodeStatementsSplitByVersion      map[string]NodeStatements
	NodeStatementsSplitByHeight       map[uint64]NodeStatements
	NodeStatementsSplitByStateHash    map[crypto.Digest]NodeStatements
	NodeStatementsSplitByHeightBucket map[uint64]NodeStatements // map[heightBucket]NodeStatements
)

func (s NodeStatements) Sort(less func(left, right *NodeStatement) bool) NodeStatements {
	sort.Slice(s, func(i, j int) bool { return less(&s[i], &s[j]) })
	return s
}

func (s NodeStatements) SortByNodeAsc() NodeStatements {
	return s.Sort(func(left, right *NodeStatement) bool { return left.Node < right.Node })
}

func (s NodeStatements) Nodes() Nodes {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, len(s))
	for i := range s {
		out[i] = s[i].Node
	}
	return out
}

func (s NodeStatements) SplitBySumStateHash() (NodeStatementsSplitByStateHash, NodeStatements) {
	var (
		split            = make(NodeStatementsSplitByStateHash)
		withoutStateHash NodeStatements
	)
	for _, statement := range s {
		if statement.Status == OK {
			sumHash := statement.StateHash.SumHash
			split[sumHash] = append(split[sumHash], statement)
		} else {
			withoutStateHash = append(withoutStateHash, statement)
		}
	}
	return split, withoutStateHash
}

func (s NodeStatements) SplitByNodeStatus() NodeStatementsSplitByStatus {
	split := make(NodeStatementsSplitByStatus)
	for _, statement := range s {
		status := statement.Status
		split[status] = append(split[status], statement)
	}
	return split
}

func (s NodeStatements) SplitByNodeHeight() NodeStatementsSplitByHeight {
	split := make(NodeStatementsSplitByHeight)
	for _, statement := range s {
		height := statement.Height
		split[height] = append(split[height], statement)
	}
	return split
}

func (s NodeStatements) SplitByNodeHeightBuckets(heightBucketSize uint64) NodeStatementsSplitByHeightBucket {
	split := make(NodeStatementsSplitByHeightBucket)
	for _, statement := range s {
		height := statement.Height
		bucketHeight := height - height%heightBucketSize
		split[bucketHeight] = append(split[bucketHeight], statement)
	}
	return split
}

func (s NodeStatements) SplitByNodeVersion() NodeStatementsSplitByVersion {
	split := make(NodeStatementsSplitByVersion)
	for _, statement := range s {
		version := statement.Version
		split[version] = append(split[version], statement)
	}
	return split
}

func (s NodeStatementsSplitByHeight) MinMaxHeight() (uint64, uint64) {
	if len(s) == 0 {
		return 0, 0
	}
	var (
		mih = uint64(math.MaxUint64)
		mah = uint64(0)
	)
	for height := range s {
		if mah < height {
			mah = height
		}
		if mih > height {
			mih = height
		}
	}
	return mih, mah
}
