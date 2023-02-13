package nodes

import (
	"log"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
	zapLogger "go.uber.org/zap"
)

const nodesFilePath = "nodes.json"
const specificNodesFilePath = "specific_nodes.json"

type NodesStorageTestSuite struct {
	zap *zapLogger.Logger
	suite.Suite
}

func (s *NodesStorageTestSuite) SetupTest() {
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

	s.zap = zap
}

func (s *NodesStorageTestSuite) TearDownTest() {
	os.Remove(nodesFilePath)
	os.Remove(specificNodesFilePath)
}

func CreateFile(content []byte, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	if err != nil {
		return err
	}
	return nil
}

func (s *NodesStorageTestSuite) TestGetListOfNodes() {

	fileContentNodes := `{"id":1,"url":"node1","enabled":true}
{"id":2,"url":"node2","enabled":true}
XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
{"id":4,"url":"node3","enabled":true}
{"id":5,"url":"node4","enabled":true}
{"id":28,"url":"node5","enabled":true}
XXXXXXXXXXXXXXXXX
{"id":7,"url":"node6","enabled":true}
{"id":8,"url":"node7","enabled":true}
{"id":9,"url":"node8","enabled":true}
{"id":10,"url":"node9","enabled":true}
{"id":31,"url":"node10","enabled":true}
XXXX
{"id":12,"url":"node11","enabled":true}
{"id":13,"url":"node12","enabled":true}
{"id":20,"url":"node13","enabled":true}
XXXXXXX
{"id":22,"url":"node14","enabled":true}

XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXXXXXXXX
{"id":24,"url":"node15","enabled":true}

{"id":26,"url":"node16","enabled":true}
{"id":29,"url":"node17","enabled":true}
{"id":30,"url":"node18","enabled":true}
{"id":32,"url":"node19","enabled":true}
XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
{"id":36,"url":"node20","enabled":true}
{"id":40,"url":"node21","enabled":true}
{"id":37,"url":"node22","enabled":true,"alias":"DN"}
`
	fileContentSpecificNodes := `{"id":2,"url":"node1","enabled":true}
{"id":1,"url":"node2","enabled":true}
`

	err := CreateFile([]byte(fileContentNodes), nodesFilePath)
	s.Require().NoError(err)

	err = CreateFile([]byte(fileContentSpecificNodes), specificNodesFilePath)
	s.Require().NoError(err)

	es, err := NewStorage(".", "", s.zap)
	s.Require().NoError(err)

	nodes, err := es.Nodes(false)
	s.Require().NoError(err)

	specificNodes, err := es.Nodes(true)
	s.Require().NoError(err)

	expectedNodes := []string{
		"node1",
		"node2",
		"node3",
		"node4",
		"node5",
		"node6",
		"node7",
		"node8",
		"node9",
		"node10",
		"node11",
		"node12",
		"node13",
		"node14",
		"node15",
		"node16",
		"node17",
		"node18",
		"node19",
		"node20",
		"node21",
		"node22",
	}

	expectedSpecificNodes := []string{
		"node1",
		"node2",
	}

	var nodesString []string
	for _, n := range nodes {
		nodesString = append(nodesString, n.URL)
	}

	var specificNodesString []string
	for _, n := range specificNodes {
		specificNodesString = append(specificNodesString, n.URL)
	}

	sort.Strings(expectedNodes)
	sort.Strings(nodesString)
	s.Equal(expectedNodes, nodesString)

	sort.Strings(expectedSpecificNodes)
	sort.Strings(specificNodesString)
	s.Equal(expectedSpecificNodes, specificNodesString)

	for i := 0; i < 10; i++ {
		_, err = es.Nodes(false)
		s.Require().NoError(err)
		_, err = es.Nodes(true)
		s.Require().NoError(err)
	}

	es.Close()
}

func TestNodesStorage(t *testing.T) {
	suite.Run(t, new(NodesStorageTestSuite))
}
