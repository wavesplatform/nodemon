package storing

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	statementKeyNodePartPrefix      = "node:"
	statementKeyTimestampPartPrefix = "ts:"
	statementKeyPartSeparator       = "|"
)

type StatementKey struct {
	NodeUrl   string
	Timestamp int64
}

func (s StatementKey) String() string {
	return fmt.Sprintf("%s%s%s%s%d",
		statementKeyNodePartPrefix, s.NodeUrl,
		statementKeyPartSeparator,
		statementKeyTimestampPartPrefix, s.Timestamp,
	)
}

func NewStatementKeyFromString(key string) (StatementKey, error) {
	split := strings.Split(key, statementKeyPartSeparator)
	if len(split) != 2 {
		return StatementKey{}, errors.Errorf("invalid statement key %q", key)
	}
	var (
		nodePart      = split[0]
		timestampPart = split[1]
	)
	if !strings.HasPrefix(nodePart, statementKeyNodePartPrefix) {
		return StatementKey{}, errors.Errorf("statement node key part %q doesn't have required prefix %q",
			nodePart, statementKeyNodePartPrefix,
		)
	}
	if !strings.HasPrefix(timestampPart, statementKeyTimestampPartPrefix) {
		return StatementKey{}, errors.Errorf("statement timestamp key part %q doesn't have required prefix %q",
			nodePart, statementKeyTimestampPartPrefix,
		)
	}
	var (
		nodeURL   = strings.TrimPrefix(nodePart, statementKeyNodePartPrefix)
		timestamp = strings.TrimPrefix(timestampPart, statementKeyTimestampPartPrefix)
	)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return StatementKey{}, errors.Wrapf(err, "failed to parse timestamp value to int from statement key %q", key)
	}
	statementKey := StatementKey{
		NodeUrl:   nodeURL,
		Timestamp: ts,
	}
	return statementKey, nil
}
