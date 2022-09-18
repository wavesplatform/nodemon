package pair

type RequestPairType byte

const (
	RequestNodeListT RequestPairType = iota + 1
	RequestSpecificNodeListT
	RequestInsertNewNodeT
	RequestInsertSpecificNewNodeT
	RequestDeleteNodeT
	RequestNodesStatus
)
