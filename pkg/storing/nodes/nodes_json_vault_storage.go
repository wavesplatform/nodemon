package nodes

import (
	"context"

	vault "github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

const (
	vaultDefaultNodesDataKey = "nodemon.nodes.json"
)

type nodesJSONVaultStorage struct {
	client     *vault.Client
	mountPath  string
	secretPath string
	dataKey    string
}

func newNodesJSONVaultStorage(client *vault.Client, mountPath, secretPath string) *nodesJSONVaultStorage {
	return &nodesJSONVaultStorage{
		client:     client,
		mountPath:  mountPath,
		secretPath: secretPath,
		dataKey:    vaultDefaultNodesDataKey,
	}
}

func (s *nodesJSONVaultStorage) putNodes(ctx context.Context, db *dbStruct) error {
	value, err := db.marshalJSON()
	if err != nil {
		return err
	}
	_, err = s.client.KVv2(s.mountPath).Put(ctx, s.secretPath, map[string]interface{}{s.dataKey: string(value)})
	return err
}

func (s *nodesJSONVaultStorage) getNodes(ctx context.Context) (*dbStruct, error) {
	secret, err := s.client.KVv2(s.mountPath).Get(ctx, s.secretPath)
	if err != nil {
		if errors.Is(err, vault.ErrSecretNotFound) {
			return new(dbStruct), nil
		}
		return nil, err
	}
	v, ok := secret.Data[s.dataKey]
	if !ok {
		return new(dbStruct), nil
	}
	data, ok := v.(string)
	if !ok {
		return nil, errors.Errorf("invalid type by nodes vault key: exptected (string), got (%T)", v)
	}
	db := new(dbStruct)
	if unmErr := db.unmarshalJSON([]byte(data)); unmErr != nil {
		return nil, unmErr
	}
	return db, nil
}
