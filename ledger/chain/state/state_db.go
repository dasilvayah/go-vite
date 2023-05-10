package chain_state

import (
	"encoding/binary"
	"math/big"
	"path"
	"sync/atomic"

	"github.com/patrickmn/go-cache"

	"github.com/vitelabs/go-vite/v2/common/config"
	leveldb "github.com/vitelabs/go-vite/v2/common/db/xleveldb"
	"github.com/vitelabs/go-vite/v2/common/db/xleveldb/util"
	"github.com/vitelabs/go-vite/v2/common/types"
	"github.com/vitelabs/go-vite/v2/interfaces"
	ledger "github.com/vitelabs/go-vite/v2/interfaces/core"
	chain_db "github.com/vitelabs/go-vite/v2/ledger/chain/db"
	chain_utils "github.com/vitelabs/go-vite/v2/ledger/chain/utils"
	"github.com/vitelabs/go-vite/v2/log15"
)

const (
	ConsensusNoCache   = 0
	ConsensusReadCache = 1
)

type StateDB struct {
	chain Chain

	chainCfg *config.Chain

	// contract address white list which save VM logs
	vmLogWhiteListSet map[types.Address]struct{}
	// save all VM logs
	vmLogAll bool

	store *chain_db.Store
	cache *cache.Cache

	log log15.Logger

	redo     *Redo
	useCache bool

	consensusCacheLevel uint32
	roundCache          *RoundCache
}

func NewStateDB(chain Chain, chainCfg *config.Chain, chainDir string) (*StateDB, error) {

	store, err := chain_db.NewStore(path.Join(chainDir, "state"), "stateDb")

	if err != nil {
		return nil, err
	}

	redoStore, err := chain_db.NewStore(path.Join(chainDir, "state_redo"), "stateDbRedo")
	if err != nil {
		return nil, err
	}

	return NewStateDBWithStore(chain, chainCfg, store, redoStore)
}

func NewStateDBWithStore(chain Chain, chainCfg *config.Chain, store *chain_db.Store, redoStore *chain_db.Store) (*StateDB, error) {
	storageRedo, err := NewStorageRedoWithStore(chain, redoStore)
	if err != nil {
		return nil, err
	}

	stateDb := &StateDB{
		chain:               chain,
		chainCfg:            chainCfg,
		vmLogWhiteListSet:   parseVmLogWhiteList(chainCfg.VmLogWhiteList),
		vmLogAll:            chainCfg.VmLogAll,
		log:                 log15.New("module", "stateDB"),
		store:               store,
		useCache:            false,
		consensusCacheLevel: ConsensusNoCache,
		redo:                storageRedo,
	}

	if err := stateDb.newCache(); err != nil {
		return nil, err
	}
	stateDb.roundCache = NewRoundCache(chain, stateDb, 3)
	return stateDb, nil
}

func (sDB *StateDB) Init() error {
	defer sDB.enableCache()

	if err := sDB.initCache(); err != nil {
		return err
	}
	//if err := sDB.roundCache.Init(); err != nil {
	//	return err
	//}
	return nil
}

func (sDB *StateDB) Close() error {
	sDB.cache.Flush()

	sDB.cache = nil

	if err := sDB.store.Close(); err != nil {
		return err
	}

	sDB.store = nil

	if err := sDB.redo.Close(); err != nil {
		return err
	}
	sDB.redo = nil
	sDB.roundCache = nil
	return nil
}

func (sDB *StateDB) SetTimeIndex(periodTimeIndex interfaces.TimeIndex) error {
	if err := sDB.roundCache.Init(periodTimeIndex); err != nil {
		return err
	}
	return nil
}

func (sDB *StateDB) GetStorageValue(addr *types.Address, key []byte) ([]byte, error) {
	value, err := sDB.store.Get(chain_utils.CreateStorageValueKey(addr, key).Bytes())
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (sDB *StateDB) GetBalance(addr types.Address, tokenTypeId types.TokenTypeId) (*big.Int, error) {
	value, err := sDB.getValue(chain_utils.CreateBalanceKey(addr, tokenTypeId).Bytes(), balancePrefix)

	if err != nil {
		return nil, err
	}

	balance := big.NewInt(0).SetBytes(value)
	return balance, nil
}

func (sDB *StateDB) GetBalanceMap(addr types.Address) (map[types.TokenTypeId]*big.Int, error) {
	balanceMap := make(map[types.TokenTypeId]*big.Int)
	iter := sDB.store.NewIterator(util.BytesPrefix(chain_utils.CreateBalanceKeyPrefix(addr)))
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		tokenTypeIdBytes := key[1+types.AddressSize : 1+types.AddressSize+types.TokenTypeIdSize]
		tokenTypeId, err := types.BytesToTokenTypeId(tokenTypeIdBytes)
		if err != nil {
			return nil, err
		}
		balanceMap[tokenTypeId] = big.NewInt(0).SetBytes(iter.Value())
	}
	if err := iter.Error(); err != nil && err != leveldb.ErrNotFound {
		return nil, err
	}

	return balanceMap, nil
}

func (sDB *StateDB) GetCode(addr types.Address) ([]byte, error) {
	code, err := sDB.store.Get(chain_utils.CreateCodeKey(addr).Bytes())
	if err != nil {
		return nil, err
	}

	return code, nil
}

//
func (sDB *StateDB) GetContractMeta(addr types.Address) (*ledger.ContractMeta, error) {
	value, err := sDB.getValueInCache(chain_utils.CreateContractMetaKey(addr).Bytes(), contractAddrPrefix)
	if err != nil {
		return nil, err
	}

	if len(value) <= 0 {
		return nil, nil
	}

	meta := &ledger.ContractMeta{}
	if err := meta.Deserialize(value); err != nil {
		return nil, err
	}
	return meta, nil
}

func (sDB *StateDB) IterateContracts(iterateFunc func(addr types.Address, meta *ledger.ContractMeta, err error) bool) {
	items := sDB.cache.Items()

	prefix := contractAddrPrefix[0]
	for keyStr, item := range items {

		if keyStr[0] == prefix {
			key := []byte(keyStr)
			addr, err := types.BytesToAddress(key[2:])
			if err != nil {
				iterateFunc(types.Address{}, nil, err)
				return
			}

			meta := &ledger.ContractMeta{}
			if err := meta.Deserialize(item.Object.([]byte)); err != nil {
				iterateFunc(types.Address{}, nil, err)
				return
			}

			if !iterateFunc(addr, meta, nil) {
				return
			}
		}
	}
}

func (sDB *StateDB) HasContractMeta(addr types.Address) (bool, error) {
	value, err := sDB.getValueInCache(chain_utils.CreateContractMetaKey(addr).Bytes(), contractAddrPrefix)
	if err != nil {
		return false, err
	}

	return len(value) > 0, nil
}

func (sDB *StateDB) GetContractList(gid *types.Gid) ([]types.Address, error) {
	iter := sDB.store.NewIterator(util.BytesPrefix(chain_utils.CreateGidContractPrefixKey(gid)))
	defer iter.Release()

	var contractList []types.Address
	for iter.Next() {
		key := iter.Key()

		addr, err := types.BytesToAddress(key[len(key)-types.AddressSize:])
		if err != nil {
			return nil, err
		}
		contractList = append(contractList, addr)
	}
	if err := iter.Error(); err != nil && err != leveldb.ErrNotFound {
		return nil, err
	}
	return contractList, nil
}

func (sDB *StateDB) GetVmLogList(logHash *types.Hash) (ledger.VmLogList, error) {
	value, err := sDB.store.Get(chain_utils.CreateVmLogListKey(logHash).Bytes())
	if err != nil {
		return nil, err
	}

	if len(value) <= 0 {
		return nil, nil
	}
	vmList := &ledger.VmLogList{}
	err = vmList.Deserialize(value)
	return *vmList, err
}

func (sDB *StateDB) GetCallDepth(sendBlockHash *types.Hash) (uint16, error) {
	value, err := sDB.store.Get(chain_utils.CreateCallDepthKey(*sendBlockHash).Bytes())
	if err != nil {
		return 0, err
	}

	if len(value) <= 0 {
		return 0, nil
	}

	return binary.BigEndian.Uint16(value), nil
}

func (sDB *StateDB) GetSnapshotBalanceList(balanceMap map[types.Address]*big.Int, snapshotBlockHash types.Hash, addrList []types.Address, tokenId types.TokenTypeId) error {
	// if consensusCacheLevel is ConsensusReadCache and tokenId is vite token id
	if sDB.consensusCacheLevel == ConsensusReadCache &&
		tokenId == ledger.ViteTokenId {
		// read from cache
		cacheBalanceMap, notFoundAddressList, err := sDB.roundCache.GetSnapshotViteBalanceList(snapshotBlockHash, addrList)
		if err != nil {
			return err
		}

		// hit the cache
		if cacheBalanceMap != nil {
			// if some address miss the cache, supplement through the database
			if len(notFoundAddressList) > 0 {
				if err := sDB.getSnapshotBalanceList(cacheBalanceMap, snapshotBlockHash, notFoundAddressList, tokenId); err != nil {
					return err
				}
			}

			// copy cache
			for addr, balance := range cacheBalanceMap {
				balanceMap[addr] = balance
			}

			// over
			return nil
		}
	}

	return sDB.getSnapshotBalanceList(balanceMap, snapshotBlockHash, addrList, tokenId)

}

func (sDB *StateDB) GetSnapshotValue(snapshotBlockHeight uint64, addr types.Address, key []byte) ([]byte, error) {

	if sDB.useCache && sDB.shouldCacheContractData(addr) && snapshotBlockHeight == sDB.chain.GetLatestSnapshotBlock().Height {
		return sDB.getValueInCache(append(addr.Bytes(), key...), snapshotValuePrefix)
	}

	startHistoryStorageKey := chain_utils.CreateHistoryStorageValueKey(&addr, key, 0)
	endHistoryStorageKey := chain_utils.CreateHistoryStorageValueKey(&addr, key, snapshotBlockHeight+1)

	iter := sDB.store.NewIterator(&util.Range{Start: startHistoryStorageKey.Bytes(), Limit: endHistoryStorageKey.Bytes()})
	defer iter.Release()

	if iter.Last() {
		return iter.Value(), nil
	}

	if err := iter.Error(); err != nil && err != leveldb.ErrNotFound {
		return nil, err
	}

	return nil, nil
}

func (sDB *StateDB) SetCacheLevelForConsensus(level uint32) {
	atomic.StoreUint32(&sDB.consensusCacheLevel, level)
}

func (sDB *StateDB) Store() *chain_db.Store {
	return sDB.store
}

func (sDB *StateDB) RedoStore() *chain_db.Store {
	return sDB.redo.store
}

func (sDB *StateDB) Redo() RedoInterface {
	return sDB.redo
}

func (sDB *StateDB) GetStatus() []interfaces.DBStatus {
	statusList := sDB.store.GetStatus()
	return []interfaces.DBStatus{{
		Name:   "stateDB.cache",
		Count:  uint64(sDB.cache.ItemCount()),
		Size:   uint64(sDB.cache.ItemCount() * 65),
		Status: "",
	}, {
		Name:   "stateDB.store.mem",
		Count:  uint64(statusList[0].Count),
		Size:   uint64(statusList[0].Size),
		Status: statusList[0].Status,
	}, {
		Name:   "stateDB.store.levelDB",
		Count:  uint64(statusList[1].Count),
		Size:   uint64(statusList[1].Size),
		Status: statusList[1].Status,
	}}
}

func (sDB *StateDB) shouldCacheContractData(addr types.Address) bool {
	return addr == types.AddressQuota || addr == types.AddressGovernance || addr == types.AddressAsset
}

func (sDB *StateDB) getSnapshotBalanceList(balanceMap map[types.Address]*big.Int, snapshotBlockHash types.Hash, addrList []types.Address, tokenId types.TokenTypeId) error {
	// get snapshot height
	snapshotHeight, err := sDB.chain.GetSnapshotHeightByHash(snapshotBlockHash)
	if err != nil {
		return err
	}
	if snapshotHeight <= 0 {
		return nil
	}

	// prepare iterator
	prefix := chain_utils.BalanceHistoryKeyPrefix
	iter := sDB.store.NewIterator(util.BytesPrefix([]byte{prefix}))
	defer iter.Release()

	//seekKey := make([]byte, 1+types.AddressSize+types.TokenTypeIdSize+8)
	//seekKey[0] = prefix
	//
	//copy(seekKey[1+types.AddressSize:], tokenId.Bytes())
	//binary.BigEndian.PutUint64(seekKey[1+types.AddressSize+types.TokenTypeIdSize:], snapshotHeight+1)

	seekKey := chain_utils.CreateHistoryBalanceKey(types.Address{}, tokenId, snapshotHeight+1)

	// iterate address list
	for _, addr := range addrList {
		//copy(seekKey[1:1+types.AddressSize], addr.Bytes())
		seekKey.AddressRefill(addr)

		iter.Seek(seekKey.Bytes())

		ok := iter.Prev()
		if !ok {
			continue
		}

		key := chain_utils.BalanceHistoryKey{}.Construct(iter.Key())

		if key != nil && key.EqualAddressAndTokenId(addr, tokenId) {
			// set balance
			balanceMap[addr] = big.NewInt(0).SetBytes(iter.Value())
			// FOR DEBUG
			// fmt.Println("query", addr, balanceMap[addr], binary.BigEndian.Uint64(key[len(key)-8:]))
		}

	}
	return nil
}

func parseVmLogWhiteList(list []types.Address) map[types.Address]struct{} {
	set := make(map[types.Address]struct{}, len(list))
	for _, item := range list {
		set[item] = struct{}{}
	}
	return set

}
