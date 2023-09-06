package nodes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"nodemon/pkg/entities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestJSONStorage(t *testing.T, nodes []string) (*JSONStorage, string) {
	t.Helper()
	dbFilePath := filepath.Join(t.TempDir(), ".nodes")
	storage, err := NewJSONFileStorage(dbFilePath, nodes, zap.NewNop())
	require.NoError(t, err, "failed to create json nodes storage with file '%s'", dbFilePath)
	return storage, dbFilePath
}

func newTestJSONStorageWithDB(t *testing.T, db *dbStruct) (*JSONStorage, string) {
	t.Helper()
	storage, path := newTestJSONStorage(t, nil)
	storage.db = db
	err := storage.syncDB()
	require.NoError(t, err)
	return storage, path
}

func checkFileIsUpdated(t *testing.T, path string, expected *dbStruct) {
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	db := new(dbStruct)
	err = json.Unmarshal(data, &db)
	require.NoError(t, err)
	assert.Equal(t, expected, db)
}

func TestJSONStorage_Close(t *testing.T) {
	var nilStorage *JSONStorage
	require.NoError(t, nilStorage.Close()) // stub for future changes
}

func TestNewJSONStorage(t *testing.T) {
	tests := []struct {
		name  string
		nodes []string
		err   string
	}{
		{
			name:  "ok",
			nodes: []string{"test.test", "fest.fest"},
		},
		{
			name:  "duplicate",
			nodes: []string{"test.test", "fest.fest", "test.test"},
			err:   "failed to populate nodes storage: found duplicate for node 'test.test'",
		},
		{
			name:  "duplicate",
			nodes: []string{"file://fest.fest", "test.test"},
			err:   "failed to populate nodes storage: failed to validate node 'file://fest.fest': unsupported URL scheme 'file'",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".nodes")
			storage, err := NewJSONFileStorage(path, test.nodes, zap.NewNop())
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				canonicalNodesURLs := make(nodes, len(test.nodes))
				for i, node := range test.nodes {
					nodeURL, validateErr := entities.ValidateNodeURL(node)
					require.NoError(t, validateErr)
					canonicalNodesURLs[i] = entities.Node{URL: nodeURL, Enabled: true}
				}
				require.NoError(t, err)
				assert.Nil(t, storage.db.SpecificNodes)
				assert.ElementsMatch(t, canonicalNodesURLs, storage.db.CommonNodes)
				checkFileIsUpdated(t, path, storage.db)
			}
		})
	}
}

func TestJSONStorage_Nodes(t *testing.T) {
	tests := []struct {
		db       dbStruct
		specific bool
		expected []entities.Node
	}{
		{
			db: dbStruct{
				CommonNodes:   nodes{{URL: "kekpek", Enabled: true, Alias: "al"}},
				SpecificNodes: nodes{{URL: "pekkek", Enabled: true, Alias: "la"}},
			},
			specific: false,
			expected: []entities.Node{{URL: "kekpek", Enabled: true, Alias: "al"}},
		},
		{
			db: dbStruct{
				CommonNodes:   nodes{{URL: "lol", Enabled: true, Alias: "fff"}},
				SpecificNodes: nodes{{URL: "blah", Enabled: true, Alias: "xxx"}},
			},
			specific: true,
			expected: []entities.Node{{URL: "blah", Enabled: true, Alias: "xxx"}},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("specific=%t", test.specific), func(t *testing.T) {
			storage, _ := newTestJSONStorageWithDB(t, &test.db)
			actual, err := storage.Nodes(test.specific)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestJSONStorage_EnabledNodes(t *testing.T) {
	tests := []struct {
		name     string
		db       dbStruct
		expected []entities.Node
	}{
		{
			name:     "Empty",
			db:       dbStruct{},
			expected: nil,
		},
		{
			name: "HasEnabled",
			db: dbStruct{CommonNodes: nodes{
				{URL: "kekpek", Enabled: true, Alias: "al"},
				{URL: "pekkek", Enabled: false, Alias: "la"},
			}},
			expected: []entities.Node{{URL: "kekpek", Enabled: true, Alias: "al"}},
		},

		{
			name: "DoesNotHaveEnabled",
			db: dbStruct{CommonNodes: nodes{
				{URL: "lol", Enabled: false, Alias: "fff"},
				{URL: "blah", Enabled: false, Alias: "xxx"},
			}},
			expected: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, _ := newTestJSONStorageWithDB(t, &test.db)
			actual, err := storage.EnabledNodes()
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestJSONStorage_EnabledSpecificNodes(t *testing.T) {
	tests := []struct {
		name     string
		db       dbStruct
		expected []entities.Node
	}{
		{
			name:     "Empty",
			db:       dbStruct{},
			expected: nil,
		},
		{
			name: "HasEnabled",
			db: dbStruct{SpecificNodes: nodes{
				{URL: "kekpek", Enabled: true, Alias: "al"},
				{URL: "pekkek", Enabled: false, Alias: "la"},
			}},
			expected: []entities.Node{{URL: "kekpek", Enabled: true, Alias: "al"}},
		},

		{
			name: "DoesNotHaveEnabled",
			db: dbStruct{SpecificNodes: nodes{
				{URL: "lol", Enabled: false, Alias: "fff"},
				{URL: "blah", Enabled: false, Alias: "xxx"},
			}},
			expected: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, _ := newTestJSONStorageWithDB(t, &test.db)
			actual, err := storage.EnabledSpecificNodes()
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestJSONStorage_Update(t *testing.T) {
	tests := []struct {
		name      string
		db        dbStruct
		update    entities.Node
		updatedDB dbStruct
		err       string
	}{
		{
			name: "UpdateSpecific",
			db: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: false, Alias: "al"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
			update: entities.Node{URL: "kekpek", Enabled: true, Alias: "updated-specific"},
			updatedDB: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: true, Alias: "updated-specific"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
		},
		{
			name: "UpdateCommon",
			db: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: false, Alias: "fff"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
			update: entities.Node{URL: "lol", Enabled: true, Alias: "updated-common"},
			updatedDB: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: true, Alias: "updated-common"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
		},
		{
			name:   "Empty",
			db:     dbStruct{},
			update: entities.Node{URL: "test-node"},
			err:    "nodeRecord 'test-node' was not found in the storage",
		},
		{
			name: "NodeNotFound",
			db: dbStruct{
				CommonNodes:   nodes{{URL: "lol", Enabled: true, Alias: "fff"}},
				SpecificNodes: nodes{{URL: "blah", Enabled: false, Alias: "xxx"}},
			},
			update: entities.Node{URL: "kekpek"},
			err:    "nodeRecord 'kekpek' was not found in the storage",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, dbFilePath := newTestJSONStorageWithDB(t, &test.db)
			err := storage.Update(test.update)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, &test.updatedDB, storage.db)
				checkFileIsUpdated(t, dbFilePath, &test.updatedDB)
			}
		})
	}
}

func TestJSONStorage_InsertIfNew(t *testing.T) {
	tests := []struct {
		name      string
		db        dbStruct
		nodeURL   string
		specific  bool
		updatedDB dbStruct
	}{
		{
			name: "AppendSpecific",
			db: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: false, Alias: "al"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
			nodeURL:  "fek",
			specific: true,
			updatedDB: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: false, Alias: "al"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
					{URL: "fek", Enabled: true},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
		},
		{
			name: "AppendCommon",
			db: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: false, Alias: "fff"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
			nodeURL:  "fuuu",
			specific: false,
			updatedDB: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: false, Alias: "fff"},
					{URL: "fuuu", Enabled: true},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
		},
		{
			name: "NothingNewSpecific",
			db: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: false, Alias: "al"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
			nodeURL:  "kekpek",
			specific: true,
			updatedDB: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: false, Alias: "al"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
		},
		{
			name: "NothingNewSpecificCommon",
			db: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: false, Alias: "fff"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
			nodeURL:  "blah",
			specific: false,
			updatedDB: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: false, Alias: "fff"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, dbFilePath := newTestJSONStorageWithDB(t, &test.db)
			err := storage.InsertIfNew(test.nodeURL, test.specific)
			require.NoError(t, err)
			assert.Equal(t, &test.updatedDB, storage.db)
			checkFileIsUpdated(t, dbFilePath, &test.updatedDB)
		})
	}
}

func TestJSONStorage_Delete(t *testing.T) {
	tests := []struct {
		name         string
		nodeToDelete string
		db           dbStruct
		updatedDB    dbStruct
		err          string
	}{
		{
			name: "DeleteSpecific",
			db: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: true, Alias: "updated-specific"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
			nodeToDelete: "kekpek",
			updatedDB: dbStruct{
				SpecificNodes: nodes{{URL: "pekkek", Enabled: true, Alias: "la"}},
				CommonNodes:   nodes{{URL: "heh"}},
			},
		},
		{
			name: "DeleteCommon",
			db: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: true, Alias: "updated-common"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
			nodeToDelete: "lol",
			updatedDB: dbStruct{
				CommonNodes:   nodes{{URL: "blah", Enabled: false, Alias: "xxx"}},
				SpecificNodes: nodes{{URL: "lala"}},
			},
		},
		{
			name:         "Empty",
			db:           dbStruct{},
			nodeToDelete: "test-node",
			err:          "nodeRecord 'test-node' was not found in the storage",
		},
		{
			name: "DeleteNotFound",
			db: dbStruct{
				CommonNodes:   nodes{{URL: "lol", Enabled: true, Alias: "fff"}},
				SpecificNodes: nodes{{URL: "blah", Enabled: false, Alias: "xxx"}},
			},
			nodeToDelete: "kekpek",
			err:          "nodeRecord 'kekpek' was not found in the storage",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, dbFilePath := newTestJSONStorageWithDB(t, &test.db)
			err := storage.Delete(test.nodeToDelete)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, &test.db, storage.db)
				checkFileIsUpdated(t, dbFilePath, &test.db)
			}
		})
	}
}

func TestJSONStorage_FindAlias(t *testing.T) {
	tests := []struct {
		name          string
		db            dbStruct
		nodeURL       string
		expectedAlias string
		err           string
	}{
		{
			name: "AliasSpecific",
			db: dbStruct{
				SpecificNodes: nodes{
					{URL: "kekpek", Enabled: true, Alias: "specific"},
					{URL: "pekkek", Enabled: true, Alias: "la"},
				},
				CommonNodes: nodes{{URL: "heh"}},
			},
			nodeURL:       "kekpek",
			expectedAlias: "specific",
		},
		{
			name: "AliasCommon",
			db: dbStruct{
				CommonNodes: nodes{
					{URL: "blah", Enabled: false, Alias: "xxx"},
					{URL: "lol", Enabled: true, Alias: "common"},
				},
				SpecificNodes: nodes{{URL: "lala"}},
			},
			nodeURL:       "lol",
			expectedAlias: "common",
		},
		{
			name:    "Empty",
			db:      dbStruct{},
			nodeURL: "test-node",
			err:     "nodeRecord 'test-node' was not found in the storage",
		},
		{
			name: "AliasNotFound",
			db: dbStruct{
				CommonNodes:   nodes{{URL: "lol", Enabled: true, Alias: "fff"}},
				SpecificNodes: nodes{{URL: "blah", Enabled: false, Alias: "xxx"}},
			},
			nodeURL: "kekpek",
			err:     "nodeRecord 'kekpek' was not found in the storage",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			storage, _ := newTestJSONStorageWithDB(t, &test.db)
			alias, err := storage.FindAlias(test.nodeURL)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedAlias, alias)
			}
		})
	}
}
