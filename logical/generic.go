package logical

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/vault/vault"
)

func init() {
	// Register the generic backend
	vault.BuiltinBackends["generic"] = newGenericBackend
}

// GenericBackend is used for the storing generic secrets. These are not
// materialized in any way. The value that is written to this backend
// is the same value that is always returned. Leasing can be configured on
// a per-key basis.
type GenericBackend struct{}

// newGenericBackend is a factory constructor for the generic backend
func newGenericBackend(map[string]string) (vault.LogicalBackend, error) {
	b := &GenericBackend{}
	return b, nil
}

// HandleRequest is used to handle a request and generate a response.
// The backends must check the operation type and handle appropriately.
func (g *GenericBackend) HandleRequest(req *vault.Request) (*vault.Response, error) {
	switch req.Operation {
	case vault.ReadOperation:
		return g.handleRead(req)
	case vault.WriteOperation:
		return g.handleWrite(req)
	case vault.ListOperation:
		return g.handleList(req)
	case vault.DeleteOperation:
		return g.handleDelete(req)
	case vault.HelpOperation:
		return g.handleHelp(req)
	default:
		return nil, vault.ErrUnsupportedOperation
	}
}

// RootPaths is a list of paths that require root level privileges,
// which do not exist for the geneirc backend.
func (g *GenericBackend) RootPaths() []string {
	return nil
}

func (g *GenericBackend) handleRead(req *vault.Request) (*vault.Response, error) {
	// Read the path
	out, err := req.View.Get(req.Path)
	if err != nil {
		return nil, fmt.Errorf("read failed: %v", err)
	}

	// Fast-path the no data case
	if out == nil {
		return nil, nil
	}

	// Decode the data
	var raw map[string]interface{}
	if err := json.Unmarshal(out.Value, &raw); err != nil {
		return nil, fmt.Errorf("json decoding failed: %v", err)
	}

	// Check if there is a lease key
	leaseVal, ok := raw["lease"].(string)
	var lease *vault.Lease
	if ok {
		leaseDuration, err := time.ParseDuration(leaseVal)
		if err == nil {
			lease = &vault.Lease{
				Renewable:    false,
				Revokable:    false,
				Duration:     leaseDuration,
				MaxDuration:  leaseDuration,
				MaxIncrement: 0,
			}
		}
	}

	// Generate the response
	resp := &vault.Response{
		IsSecret: true,
		Lease:    lease,
		Data:     raw,
	}
	return resp, nil
}

func (g *GenericBackend) handleWrite(req *vault.Request) (*vault.Response, error) {
	// Check that some fields are given
	if len(req.Data) == 0 {
		return nil, fmt.Errorf("missing data fields")
	}

	// JSON encode the data
	buf, err := json.Marshal(req.Data)
	if err != nil {
		return nil, fmt.Errorf("json encoding failed: %v", err)
	}

	// Write out a new key
	entry := &vault.Entry{
		Key:   req.Path,
		Value: buf,
	}
	if err := req.View.Put(entry); err != nil {
		return nil, fmt.Errorf("failed to write: %v", err)
	}
	return nil, nil
}

func (g *GenericBackend) handleDelete(req *vault.Request) (*vault.Response, error) {
	// Delete the key at the request path
	if err := req.View.Delete(req.Path); err != nil {
		return nil, err
	}
	return nil, nil
}

func (g *GenericBackend) handleList(req *vault.Request) (*vault.Response, error) {
	// List the keys at the prefix given by the request
	keys, err := req.View.List(req.Path)
	if err != nil {
		return nil, err
	}

	// Generate the response
	resp := &vault.Response{
		IsSecret: false,
		Lease:    nil,
		Data: map[string]interface{}{
			"keys": keys,
		},
	}
	return resp, nil
}

func (g *GenericBackend) handleHelp(req *vault.Request) (*vault.Response, error) {
	resp := &vault.Response{
		IsSecret: false,
		Lease:    nil,
		Data: map[string]interface{}{
			"help": genericHelpText,
		},
	}
	return resp, nil
}

// genericHelpText is the help information we return
const genericHelpText = "Generic backend for storing and retreiving raw keys with user-defined fields"
