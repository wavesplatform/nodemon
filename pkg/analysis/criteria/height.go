package criteria

import (
	"strings"

	"nodemon/pkg/entities"

	"go.uber.org/zap"
)

const (
	defaultMaxHeightDiff = 3
)

type HeightCriterionOptions struct {
	MaxHeightDiff int
}

type HeightCriterion struct {
	opts   *HeightCriterionOptions
	logger *zap.Logger
}

func NewHeightCriterion(opts *HeightCriterionOptions, logger *zap.Logger) *HeightCriterion {
	if opts == nil { // default
		opts = &HeightCriterionOptions{
			MaxHeightDiff: defaultMaxHeightDiff,
		}
	}
	return &HeightCriterion{opts: opts, logger: logger}
}

func (c *HeightCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) {
	split := statements.SplitByNodeHeight()
	min, max := split.MinMaxHeight()
	if min == max { // all nodes on same height
		return
	}
	sortedMaxGroup := split[max].Nodes().Sort()
	for height, nodeStatements := range split {
		heightDiff := max - height
		if heightDiff > 0 {
			c.logger.Info("HeightCriterion: height difference detected",
				zap.Int("height difference", heightDiff),
				zap.String("first group", strings.Join(sortedMaxGroup, ", ")),
				zap.Int("first group height", max),
				zap.String("second group", strings.Join(nodeStatements.Nodes().Sort(), ", ")),
				zap.Int("second group height", height),
			)
		}
		if heightDiff > c.opts.MaxHeightDiff {
			alerts <- &entities.HeightAlert{
				Timestamp: timestamp,
				MaxHeightGroup: entities.HeightGroup{
					Height: max,
					Nodes:  sortedMaxGroup,
				},
				OtherHeightGroup: entities.HeightGroup{
					Height: height,
					Nodes:  nodeStatements.Nodes().Sort(),
				},
			}
		}
	}
}
