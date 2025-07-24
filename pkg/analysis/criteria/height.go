package criteria

import (
	"log/slog"

	"nodemon/pkg/entities"
	"nodemon/pkg/tools/logging/attrs"
)

const (
	defaultMaxHeightDiff = 3
)

type HeightCriterionOptions struct {
	MaxHeightDiff uint64
}

type HeightCriterion struct {
	opts   *HeightCriterionOptions
	logger *slog.Logger
}

func NewHeightCriterion(opts *HeightCriterionOptions, logger *slog.Logger) *HeightCriterion {
	if opts == nil { // default
		opts = &HeightCriterionOptions{
			MaxHeightDiff: defaultMaxHeightDiff,
		}
	}
	return &HeightCriterion{opts: opts, logger: logger}
}

func (c *HeightCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) {
	split := statements.SplitByNodeHeight()
	minHeight, maxHeight := split.MinMaxHeight()
	if minHeight == maxHeight { // all nodes on same height
		return
	}
	sortedMaxGroup := split[maxHeight].Nodes().Sort()
	for height, nodeStatements := range split {
		heightDiff := maxHeight - height
		if heightDiff > 0 {
			c.logger.Info("HeightCriterion: height difference detected",
				slog.Uint64("height difference", heightDiff),
				attrs.Strings("first group", sortedMaxGroup),
				slog.Uint64("first group height", maxHeight),
				attrs.Strings("second group", nodeStatements.Nodes().Sort()),
				slog.Uint64("second group height", height),
			)
		}
		if heightDiff > c.opts.MaxHeightDiff {
			alerts <- &entities.HeightAlert{
				Timestamp: timestamp,
				MaxHeightGroup: entities.HeightGroup{
					Height: maxHeight,
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
