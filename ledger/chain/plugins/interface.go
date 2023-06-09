package chain_plugins

import (
	"github.com/vitelabs/go-vite/v2/common/db/xleveldb"
	"github.com/vitelabs/go-vite/v2/common/types"
	ledger "github.com/vitelabs/go-vite/v2/interfaces/core"
	"github.com/vitelabs/go-vite/v2/ledger/chain/db"
	"github.com/vitelabs/go-vite/v2/ledger/chain/flusher"
)

type Chain interface {
	Flusher() *chain_flusher.Flusher
	GetLatestSnapshotBlock() *ledger.SnapshotBlock
	GetSnapshotBlocksByHeight(height uint64, higher bool, count uint64) ([]*ledger.SnapshotBlock, error)
	GetSubLedgerAfterHeight(height uint64) ([]*ledger.SnapshotChunk, error)
	GetSubLedger(startHeight, endHeight uint64) ([]*ledger.SnapshotChunk, error)
	GetAccountBlockByHash(blockHash types.Hash) (*ledger.AccountBlock, error)

	IsAccountBlockExisted(hash types.Hash) (bool, error)
	IsGenesisAccountBlock(hash types.Hash) bool

	GetAllUnconfirmedBlocks() []*ledger.AccountBlock

	LoadAllOnRoad() (map[types.Address][]types.Hash, error)
}

type Plugin interface {
	SetStore(store *chain_db.Store)

	InsertAccountBlock(*leveldb.Batch, *ledger.AccountBlock) error

	InsertSnapshotBlock(*leveldb.Batch, *ledger.SnapshotBlock, []*ledger.AccountBlock) error

	DeleteAccountBlocks(*leveldb.Batch, []*ledger.AccountBlock) error

	DeleteSnapshotBlocks(*leveldb.Batch, []*ledger.SnapshotChunk) error

	RemoveNewUnconfirmed(*leveldb.Batch, []*ledger.AccountBlock) error
}
