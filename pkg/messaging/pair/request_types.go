package pair

type RequestPairType byte

const (
	RequestNodeListType RequestPairType = iota + 1
	RequestSpecificNodeListType
	RequestInsertNewNodeType
	RequestInsertSpecificNewNodeType
	RequestUpdateNodeType
	RequestDeleteNodeType
	RequestNodesStatusType
	RequestNodeStatementType
)
