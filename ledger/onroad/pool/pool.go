package onroad_pool

import (
	"github.com/vitelabs/go-vite/v2/common/types"
	"github.com/vitelabs/go-vite/v2/interfaces"
	ledger "github.com/vitelabs/go-vite/v2/interfaces/core"
)

type OnRoadPool interface {
	InsertAccountBlocks(orAddr types.Address, blocks []*ledger.AccountBlock) error
	DeleteAccountBlocks(orAddr types.Address, blocks []*ledger.AccountBlock) error

	GetOnRoadTotalNumByAddr(addr types.Address) (uint64, error)
	GetFrontOnRoadBlocksByAddr(addr types.Address) ([]*ledger.AccountBlock, error)

	IsFrontOnRoadOfCaller(orAddr, caller types.Address, hash types.Hash) (bool, error)
	Info() map[string]interface{}
}

type chainReader interface {
	LoadOnRoadRange(gid types.Gid, fn interfaces.LoadOnroadFn) error
	GetAccountBlockByHash(blockHash types.Hash) (*ledger.AccountBlock, error)
	GetCompleteBlockByHash(blockHash types.Hash) (*ledger.AccountBlock, error)
}
