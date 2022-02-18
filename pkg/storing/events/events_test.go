package events

import (
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"nodemon/pkg/entities"
)

type EventsStorageTestSuite struct {
	suite.Suite
	es *Storage
}

func (s *EventsStorageTestSuite) SetupTest() {
	es, err := NewStorage(time.Minute)
	s.Require().NoError(err)
	s.es = es
}

func (s *EventsStorageTestSuite) TearDownTest() {
	s.Require().NoError(s.es.Close())
}

func (s *EventsStorageTestSuite) TestPutAndGetStatementRoundtrip() {
	tests := []struct {
		event    entities.Event
		expected entities.NodeStatement
	}{
		{
			event: entities.NewUnreachableEvent("blah", 100500),
			expected: entities.NodeStatement{
				Node:      "blah",
				Timestamp: 100500,
				Status:    entities.Unreachable,
			},
		},
		{
			event: entities.NewVersionEvent("blah-blah", 500100, "v42.0.0"),
			expected: entities.NodeStatement{
				Node:      "blah-blah",
				Timestamp: 500100,
				Status:    entities.Incomplete,
				Version:   "v42.0.0",
			},
		},
		{
			event: entities.NewHeightEvent("blah-blah-blah", 8888, "21.0.0", 777),
			expected: entities.NodeStatement{
				Node:      "blah-blah-blah",
				Timestamp: 8888,
				Status:    entities.Incomplete,
				Version:   "21.0.0",
				Height:    777,
			},
		},
	}
	for i, test := range tests {
		testNum := i + 1

		err := s.es.PutEvent(test.event)
		s.Require().NoError(err, "testcase #%d", testNum)

		actual, err := s.es.GetStatement(test.expected.Node, test.expected.Timestamp)
		s.Require().NoError(err, "testcase #%d", testNum)

		s.Assert().Equal(test.expected, actual, "testcase #%d", testNum)
	}
}

func (s *EventsStorageTestSuite) TestGetStatementNotFound() {
	_, err := s.es.GetStatement("some-url", 100500)
	s.Assert().True(errors.Is(err, ErrNotFound))
}

type dummyEvent struct {
	entities.NodeStatement
}

func (d *dummyEvent) Node() string {
	return d.NodeStatement.Node
}

func (d *dummyEvent) Timestamp() int64 {
	return d.NodeStatement.Timestamp
}

func (d *dummyEvent) Statement() entities.NodeStatement {
	return d.NodeStatement
}

func (s *EventsStorageTestSuite) TestViewStatementsByNodeURLWithDescendKeys() {
	tests := []struct {
		node     string
		expected entities.NodeStatements
	}{
		{
			node: "blah",
			expected: entities.NodeStatements{
				{
					Node:      "blah",
					Timestamp: 534,
					Status:    entities.Unreachable,
				},
				{
					Node:      "blah",
					Timestamp: 6345,
					Status:    entities.Incomplete,
					Version:   "v42.0.0",
				},
				{
					Node:      "blah",
					Timestamp: 1235,
					Status:    entities.Incomplete,
					Version:   "21.0.0",
					Height:    777,
				},
			},
		},
		{
			node: "blah-blah",
			expected: entities.NodeStatements{
				{
					Node:      "blah-blah",
					Timestamp: 3456,
					Status:    entities.Unreachable,
				},
				{
					Node:      "blah-blah",
					Timestamp: 5345,
					Status:    entities.Incomplete,
					Version:   "v42.0.0",
				},
				{
					Node:      "blah-blah",
					Timestamp: 4234,
					Status:    entities.Incomplete,
					Version:   "21.0.0",
					Height:    777,
				},
			},
		},
	}
	for i, test := range tests {
		testNum := i + 1

		for _, statement := range test.expected {
			s.Require().NoError(s.es.PutEvent(&dummyEvent{statement}), "testcase #%d", testNum)
		}

		var actual entities.NodeStatements
		err := s.es.ViewStatementsByNodeURLWithDescendKeys(test.node, func(statement *entities.NodeStatement) bool {
			actual = append(actual, *statement)
			return true
		})
		s.Require().NoError(err, "testcase #%d", testNum)

		sort.Slice(test.expected, func(i, j int) bool {
			return test.expected[i].Timestamp > test.expected[j].Timestamp
		})
		sort.Slice(actual, func(i, j int) bool {
			return actual[i].Timestamp > actual[j].Timestamp
		})

		s.Assert().Equal(test.expected, actual, "testcase #%d", testNum)
	}
}

func (s *EventsStorageTestSuite) TestViewStatementsByTimestamp() {
	tests := []struct {
		timestamp int64
		expected  entities.NodeStatements
	}{
		{
			timestamp: 100500,
			expected: entities.NodeStatements{
				{
					Node:      "blah",
					Timestamp: 100500,
					Status:    entities.Unreachable,
				},
				{
					Node:      "blah-blah",
					Timestamp: 100500,
					Status:    entities.Incomplete,
					Version:   "v42.0.0",
				},
				{
					Node:      "blah-blah-blah",
					Timestamp: 100500,
					Status:    entities.Incomplete,
					Version:   "21.0.0",
					Height:    777,
				},
			},
		},
		{
			timestamp: 500100,
			expected: entities.NodeStatements{
				{
					Node:      "blah",
					Timestamp: 500100,
					Status:    entities.Unreachable,
				},
				{
					Node:      "blah-blah",
					Timestamp: 500100,
					Status:    entities.Incomplete,
					Version:   "v42.0.0",
				},
				{
					Node:      "blah-blah-blah",
					Timestamp: 500100,
					Status:    entities.Incomplete,
					Version:   "21.0.0",
					Height:    777,
				},
			},
		},
	}
	for i, test := range tests {
		testNum := i + 1

		for _, statement := range test.expected {
			s.Require().NoError(s.es.PutEvent(&dummyEvent{statement}), "testcase #%d", testNum)
		}

		var actual entities.NodeStatements
		err := s.es.ViewStatementsByTimestamp(test.timestamp, func(statement *entities.NodeStatement) bool {
			actual = append(actual, *statement)
			return true
		})
		s.Require().NoError(err, "testcase #%d", testNum)

		sort.Slice(test.expected, func(i, j int) bool {
			return test.expected[i].Node > test.expected[j].Node
		})
		sort.Slice(actual, func(i, j int) bool {
			return actual[i].Node > actual[j].Node
		})

		s.Assert().Equal(test.expected, actual, "testcase #%d", testNum)
	}
}

func (s *EventsStorageTestSuite) TestStatementsCount() {
	statements := entities.NodeStatements{
		{
			Node:      "blah",
			Timestamp: 100500,
			Status:    entities.Unreachable,
		},
		{
			Node:      "blah-blah",
			Timestamp: 100500,
			Status:    entities.Incomplete,
			Version:   "v42.0.0",
		},
		{
			Node:      "blah-blah-blah",
			Timestamp: 100500,
			Status:    entities.Incomplete,
			Version:   "21.0.0",
			Height:    777,
		},
	}
	for i, statement := range statements {
		testNum := i + 1

		cnt, err := s.es.StatementsCount()
		s.Require().NoError(err, "testcase #%d", testNum)
		s.Assert().Equal(i, cnt, "testcase #%d", testNum)

		s.Require().NoError(s.es.PutEvent(&dummyEvent{statement}), "testcase #%d", testNum)

		cnt, err = s.es.StatementsCount()
		s.Require().NoError(err, "testcase #%d", testNum)
		s.Assert().Equal(i+1, cnt, "testcase #%d", testNum)
	}
}

func TestEventsStorage(t *testing.T) {
	suite.Run(t, new(EventsStorageTestSuite))
}
