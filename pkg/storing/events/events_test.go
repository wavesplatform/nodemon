package events

import (
	"errors"
	"log"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	zapLogger "go.uber.org/zap"
	"nodemon/pkg/entities"
)

type EventsStorageTestSuite struct {
	suite.Suite
	es *Storage
}

func (s *EventsStorageTestSuite) SetupTest() {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)
	es, err := NewStorage(time.Minute, zap)
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

func (d *dummyEvent) Height() int {
	return d.NodeStatement.Height
}

func (d *dummyEvent) Statement() entities.NodeStatement {
	return d.NodeStatement
}

func (d *dummyEvent) WithTimestamp(ts int64) entities.Event {
	cpy := *d
	cpy.NodeStatement.Timestamp = ts
	return &cpy
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
		err := s.es.ViewStatementsByNodeWithDescendKeys(test.node, func(statement *entities.NodeStatement) bool {
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

func TestEarliestHeight(t *testing.T) {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)

	for _, test := range []struct {
		node     string
		events   []entities.Event
		error    bool
		expected int
	}{
		{"A", events(she("A", 1, 100), she("A", 1, 200), she("A", 2, 300), she("A", 3, 400)), false, 1},
		{"A", events(she("B", 1, 100), she("B", 2, 200), she("B", 3, 300)), true, 0},
		{"A", events(she("B", 1, 110), she("A", 2, 200), she("B", 2, 210), she("B", 3, 300), she("A", 3, 310)), false, 2},
		{"A", events(he("A", 1, 100), he("A", 1, 200), he("A", 2, 300), he("A", 3, 400)), true, 0},
		{"A", events(he("A", 1, 100), she("B", 1, 110), she("A", 2, 200), she("B", 2, 210), she("B", 3, 300), he("A", 3, 300), she("A", 3, 310)), false, 2},
	} {
		storage, err := NewStorage(time.Minute, zap)
		require.NoError(t, err)
		putEvents(t, storage, test.events)
		h, err := storage.EarliestHeight(test.node)
		if test.error {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, test.expected, h)
		}
	}
}

func TestLatestHeight(t *testing.T) {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)

	for _, test := range []struct {
		node     string
		events   []entities.Event
		error    bool
		expected int
	}{
		{"A", events(she("A", 1, 100), she("A", 1, 200), she("A", 2, 300), she("A", 3, 400)), false, 3},
		{"A", events(she("B", 1, 100), she("B", 2, 200), she("B", 3, 300)), true, 0},
		{"A", events(she("B", 1, 110), she("A", 2, 200), she("B", 2, 210), she("B", 3, 300), she("A", 3, 310)), false, 3},
		{"A", events(he("A", 1, 100), he("A", 1, 200), he("A", 2, 300), he("A", 3, 400)), true, 0},
		{"A", events(he("A", 1, 100), she("B", 1, 110), she("A", 2, 200), she("B", 2, 210), she("B", 3, 300), he("A", 3, 300), she("A", 3, 310)), false, 3},
	} {
		storage, err := NewStorage(time.Minute, zap)
		require.NoError(t, err)
		putEvents(t, storage, test.events)
		h, err := storage.LatestHeight(test.node)
		if test.error {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, test.expected, h)
		}
	}
}

func TestLastStateHashAtHeight(t *testing.T) {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)

	d1 := crypto.Digest([32]byte{0x01})
	d2 := crypto.Digest([32]byte{0x02})
	d3 := crypto.Digest([32]byte{0x03})
	b1 := proto.NewBlockIDFromDigest(d1)
	b2 := proto.NewBlockIDFromDigest(d2)
	b3 := proto.NewBlockIDFromDigest(d3)
	sh1 := &proto.StateHash{BlockID: b1, SumHash: d1}
	sh2 := &proto.StateHash{BlockID: b2, SumHash: d2}
	sh3 := &proto.StateHash{BlockID: b3, SumHash: d3}
	for _, test := range []struct {
		node     string
		events   []entities.Event
		height   int
		error    bool
		expected proto.StateHash
	}{
		{"A", events(fshe("A", 1, 100, sh1), fshe("A", 2, 200, sh2), fshe("A", 3, 300, sh3)), 3, false, *sh3},
		{"A", events(fshe("A", 1, 100, sh1), fshe("A", 2, 200, sh2), fshe("A", 2, 300, sh3)), 2, false, *sh3},
		{"A", events(fshe("A", 1, 100, sh1), fshe("A", 1, 110, sh2), fshe("A", 2, 200, sh3)), 1, false, *sh2},
		{"A", events(he("A", 1, 100), he("A", 1, 110), he("A", 2, 200)), 1, true, proto.StateHash{}},
		{"A", events(he("A", 1, 100), he("A", 1, 110), he("A", 2, 200)), 2, true, proto.StateHash{}},
		{"A", events(fshe("A", 1, 100, sh1), he("A", 1, 110), he("A", 2, 200)), 1, false, *sh1},
	} {
		storage, err := NewStorage(time.Minute, zap)
		require.NoError(t, err)
		putEvents(t, storage, test.events)
		sh, err := storage.StateHashAtHeight(test.node, test.height)
		if test.error {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, test.expected, sh)
		}
	}
}

func events(es ...entities.Event) []entities.Event {
	return es
}

func she(n string, h int, ts int64) entities.Event {
	return fshe(n, h, ts, &proto.StateHash{})
}

func fshe(n string, h int, ts int64, sh *proto.StateHash) entities.Event {
	return entities.NewStateHashEvent(n, ts, "", h, sh, 1)
}

func he(n string, h int, ts int64) entities.Event {
	return entities.NewStateHashEvent(n, ts, "", h, nil, 1)
}

func putEvents(t *testing.T, st *Storage, events []entities.Event) {
	for _, ev := range events {
		err := st.PutEvent(ev)
		require.NoError(t, err)
	}
}

func TestStatusSameHeightInStorage(t *testing.T) {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)

	d1 := crypto.Digest([32]byte{0x01})
	d2 := crypto.Digest([32]byte{0x02})
	d3 := crypto.Digest([32]byte{0x02})
	b1 := proto.NewBlockIDFromDigest(d1)
	b2 := proto.NewBlockIDFromDigest(d2)
	b3 := proto.NewBlockIDFromDigest(d3)
	sh1 := &proto.StateHash{BlockID: b1, SumHash: d1}
	sh2 := &proto.StateHash{BlockID: b2, SumHash: d2}
	sh3 := &proto.StateHash{BlockID: b3, SumHash: d3}
	for _, test := range []struct {
		testcase       string
		events         []entities.Event
		expectedError  bool
		expectedHeight int
		expectedSH     *proto.StateHash
	}{
		// node 1 - 1000; node 2 - 1000; node 3 - 1000, 1001. Expected height 1000
		{"testcase 1", events(fshe("1", 1000, 100, sh1), fshe("2", 1000, 100, sh1), fshe("3", 1000, 100, sh1), fshe("3", 1001, 101, sh2)), false, 1000, sh1},

		// node 1 - 999,1000,1001; node 2 - 999, 1001; node 3 - 999, 1000. Expected height 999
		{"testcase 2", events(fshe("1", 999, 99, sh1), fshe("1", 1000, 100, sh2), fshe("1", 1001, 101, sh3), fshe("2", 999, 99, sh1), fshe("2", 1001, 101, sh3), fshe("3", 999, 99, sh1), fshe("3", 1000, 100, sh2)), false, 999, sh1},
	} {
		storage, err := NewStorage(time.Minute, zap)
		require.NoError(t, err)
		putEvents(t, storage, test.events)
		statements, err := storage.FindAllStateHashesOnCommonHeight([]string{"1", "2", "3"})
		if test.expectedError {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			for _, st := range statements {
				require.Equal(t, test.expectedHeight, st.Height)
				require.Equal(t, test.expectedSH, st.StateHash)
			}
		}
	}
}
