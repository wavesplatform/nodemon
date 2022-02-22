package events

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	statementKeyNodePartPrefix      = "node:"
	statementKeyTimestampPartPrefix = "ts:"
	statementKeyPartSeparator       = "|"
)

type statementKey struct {
	node      string
	timestamp int64
}

func (s statementKey) String() string {
	return newStatementKey(s.node, strconv.FormatInt(s.timestamp, 10))
}

func newStatementKeyFromString(key string) (statementKey, error) {
	split := strings.Split(key, statementKeyPartSeparator)
	if len(split) != 2 {
		return statementKey{}, errors.Errorf("invalid statement key %q", key)
	}
	var (
		nodePart      = split[0]
		timestampPart = split[1]
	)
	if !strings.HasPrefix(nodePart, statementKeyNodePartPrefix) {
		return statementKey{}, errors.Errorf("statement node key part %q doesn't have required prefix %q",
			nodePart, statementKeyNodePartPrefix,
		)
	}
	if !strings.HasPrefix(timestampPart, statementKeyTimestampPartPrefix) {
		return statementKey{}, errors.Errorf("statement timestamp key part %q doesn't have required prefix %q",
			nodePart, statementKeyTimestampPartPrefix,
		)
	}
	var (
		nodeURL   = strings.TrimPrefix(nodePart, statementKeyNodePartPrefix)
		timestamp = strings.TrimPrefix(timestampPart, statementKeyTimestampPartPrefix)
	)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return statementKey{}, errors.Wrapf(err, "failed to parse timestamp value to int from statement key %q", key)
	}
	statementKey := statementKey{
		node:      nodeURL,
		timestamp: ts,
	}
	return statementKey, nil
}

func newStatementKey(node, timestamp string) string {
	var buf strings.Builder
	buf.Grow(
		len(statementKeyNodePartPrefix) + len(node) +
			len(statementKeyPartSeparator) +
			len(statementKeyTimestampPartPrefix) + len(timestamp),
	)
	buf.WriteString(statementKeyNodePartPrefix)
	buf.WriteString(node)
	buf.WriteString(statementKeyPartSeparator)
	buf.WriteString(statementKeyTimestampPartPrefix)
	buf.WriteString(timestamp)
	return buf.String()
}
