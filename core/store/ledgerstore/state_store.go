/*
 * Copyright (C) 2018 The dad-go Authors
 * This file is part of The dad-go library.
 *
 * The dad-go is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The dad-go is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The dad-go.  If not, see <http://www.gnu.org/licenses/>.
 */

package ledgerstore

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/common/serialization"
	"github.com/ontio/dad-go/core/payload"
	"github.com/ontio/dad-go/core/states"
	scom "github.com/ontio/dad-go/core/store/common"
	"github.com/ontio/dad-go/core/store/leveldbstore"
	"github.com/ontio/dad-go/core/store/overlaydb"
	"github.com/ontio/dad-go/core/store/statestore"
	"github.com/ontio/dad-go/merkle"
	"github.com/ontio/dad-go/smartcontract/service/native/ontid"
	"github.com/ontio/dad-go/smartcontract/service/native/utils"
)

var (
	BOOKKEEPER = []byte("Bookkeeper") //Bookkeeper store key
)

//StateStore saving the data of ledger states. Like balance of account, and the execution result of smart contract
type StateStore struct {
	dbDir           string                    //Store file path
	store           scom.PersistStore         //Store handler
	merklePath      string                    //Merkle tree store path
	merkleTree      *merkle.CompactMerkleTree //Merkle tree of block root
	merkleHashStore merkle.HashStore
}

//NewStateStore return state store instance
func NewStateStore(dbDir, merklePath string) (*StateStore, error) {
	var err error
	store, err := leveldbstore.NewLevelDBStore(dbDir)
	if err != nil {
		return nil, err
	}
	stateStore := &StateStore{
		dbDir:      dbDir,
		store:      store,
		merklePath: merklePath,
	}
	_, height, err := stateStore.GetCurrentBlock()
	if err != nil && err != scom.ErrNotFound {
		return nil, fmt.Errorf("GetCurrentBlock error %s", err)
	}
	err = stateStore.init(height)
	if err != nil {
		return nil, fmt.Errorf("init error %s", err)
	}
	return stateStore, nil
}

//NewBatch start new commit batch
func (self *StateStore) NewBatch() {
	self.store.NewBatch()
}

func (self *StateStore) init(currBlockHeight uint32) error {
	treeSize, hashes, err := self.GetMerkleTree()
	if err != nil && err != scom.ErrNotFound {
		return err
	}
	if treeSize > 0 && treeSize != currBlockHeight+1 {
		return fmt.Errorf("merkle tree size is inconsistent with blockheight: %d", currBlockHeight+1)
	}
	self.merkleHashStore, err = merkle.NewFileHashStore(self.merklePath, treeSize)
	if err != nil {
		return fmt.Errorf("merkle store is inconsistent with ChainStore. persistence will be disabled")
	}
	self.merkleTree = merkle.NewTree(treeSize, hashes, self.merkleHashStore)
	return nil
}

//GetMerkleTree return merkle tree size an tree node
func (self *StateStore) GetMerkleTree() (uint32, []common.Uint256, error) {
	key := self.getMerkleTreeKey()
	data, err := self.store.Get(key)
	if err != nil {
		return 0, nil, err
	}
	value := bytes.NewBuffer(data)
	treeSize, err := serialization.ReadUint32(value)
	if err != nil {
		return 0, nil, err
	}
	hashCount := (len(data) - 4) / common.UINT256_SIZE
	hashes := make([]common.Uint256, 0, hashCount)
	for i := 0; i < hashCount; i++ {
		var hash = new(common.Uint256)
		err = hash.Deserialize(value)
		if err != nil {
			return 0, nil, err
		}
		hashes = append(hashes, *hash)
	}
	return treeSize, hashes, nil
}

//AddMerkleTreeRoot add a new tree root
func (self *StateStore) AddMerkleTreeRoot(txRoot common.Uint256) error {
	key := self.getMerkleTreeKey()

	self.merkleTree.AppendHash(txRoot)
	err := self.merkleHashStore.Flush()
	if err != nil {
		return err
	}
	treeSize := self.merkleTree.TreeSize()
	hashes := self.merkleTree.Hashes()
	value := bytes.NewBuffer(make([]byte, 0, 4+len(hashes)*common.UINT256_SIZE))
	err = serialization.WriteUint32(value, treeSize)
	if err != nil {
		return err
	}
	for _, hash := range hashes {
		err = hash.Serialize(value)
		if err != nil {
			return err
		}
	}
	self.store.BatchPut(key, value.Bytes())
	return nil
}

//GetMerkleProof return merkle proof of block
func (self *StateStore) GetMerkleProof(proofHeight, rootHeight uint32) ([]common.Uint256, error) {
	return self.merkleTree.InclusionProof(proofHeight, rootHeight+1)
}

//NewStateBatch return state commit bathe. Usually using in smart contract execution
func (self *StateStore) NewStateBatch() *statestore.StateBatch {
	return statestore.NewStateStoreBatch(statestore.NewMemDatabase(), self.store)
}

func (self *StateStore) NewOverlayDB() *overlaydb.OverlayDB {
	return overlaydb.NewOverlayDB(self.store)
}

//CommitTo commit state batch to state store
func (self *StateStore) CommitTo() error {
	return self.store.BatchCommit()
}

//GetContractState return contract by contract address
func (self *StateStore) GetContractState(contractHash common.Address) (*payload.DeployCode, error) {
	key, err := self.getContractStateKey(contractHash)
	if err != nil {
		return nil, err
	}

	value, err := self.store.Get(key)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(value)
	contractState := new(payload.DeployCode)
	err = contractState.Deserialize(reader)
	if err != nil {
		return nil, err
	}
	return contractState, nil
}

//GetBookkeeperState return current book keeper states
func (self *StateStore) GetBookkeeperState() (*states.BookkeeperState, error) {
	key, err := self.getBookkeeperKey()
	if err != nil {
		return nil, err
	}

	value, err := self.store.Get(key)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(value)
	bookkeeperState := new(states.BookkeeperState)
	err = bookkeeperState.Deserialize(reader)
	if err != nil {
		return nil, err
	}
	return bookkeeperState, nil
}

//SaveBookkeeperState persist book keeper state to store
func (self *StateStore) SaveBookkeeperState(bookkeeperState *states.BookkeeperState) error {
	key, err := self.getBookkeeperKey()
	if err != nil {
		return err
	}
	value := bytes.NewBuffer(nil)
	err = bookkeeperState.Serialize(value)
	if err != nil {
		return err
	}

	return self.store.Put(key, value.Bytes())
}

//GetStorageItem return the storage value of the key in smart contract.
func (self *StateStore) GetStorageState(key *states.StorageKey) (*states.StorageItem, error) {
	storeKey, err := self.getStorageKey(key)
	if err != nil {
		return nil, err
	}

	data, err := self.store.Get(storeKey)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	storageState := new(states.StorageItem)
	err = storageState.Deserialize(reader)
	if err != nil {
		return nil, err
	}
	return storageState, nil
}

//GetCurrentBlock return current block height and current hash in state store
func (self *StateStore) GetCurrentBlock() (common.Uint256, uint32, error) {
	key := self.getCurrentBlockKey()
	data, err := self.store.Get(key)
	if err != nil {
		return common.Uint256{}, 0, err
	}
	reader := bytes.NewReader(data)
	blockHash := common.Uint256{}
	err = blockHash.Deserialize(reader)
	if err != nil {
		return common.Uint256{}, 0, err
	}
	height, err := serialization.ReadUint32(reader)
	if err != nil {
		return common.Uint256{}, 0, err
	}
	return blockHash, height, nil
}

//SaveCurrentBlock persist current block to state store
func (self *StateStore) SaveCurrentBlock(height uint32, blockHash common.Uint256) error {
	key := self.getCurrentBlockKey()
	value := bytes.NewBuffer(nil)
	blockHash.Serialize(value)
	serialization.WriteUint32(value, height)
	self.store.BatchPut(key, value.Bytes())
	return nil
}

func (self *StateStore) getCurrentBlockKey() []byte {
	return []byte{byte(scom.SYS_CURRENT_BLOCK)}
}

func (self *StateStore) getBookkeeperKey() ([]byte, error) {
	key := make([]byte, 1+len(BOOKKEEPER))
	key[0] = byte(scom.ST_BOOKKEEPER)
	copy(key[1:], []byte(BOOKKEEPER))
	return key, nil
}

func (self *StateStore) getContractStateKey(contractHash common.Address) ([]byte, error) {
	data := contractHash[:]
	key := make([]byte, 1+len(data))
	key[0] = byte(scom.ST_CONTRACT)
	copy(key[1:], []byte(data))
	return key, nil
}

func (self *StateStore) getStorageKey(key *states.StorageKey) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(byte(scom.ST_STORAGE))
	buf.Write(key.ContractAddress[:])
	buf.Write(key.Key)
	return buf.Bytes(), nil
}

func (self *StateStore) GetBlockRootWithNewTxRoot(txRoot common.Uint256) common.Uint256 {
	return self.merkleTree.GetRootWithNewLeaf(txRoot)
}

func (self *StateStore) getMerkleTreeKey() []byte {
	return []byte{byte(scom.SYS_BLOCK_MERKLE_TREE)}
}

//ClearAll clear all data in state store
func (self *StateStore) ClearAll() error {
	self.store.NewBatch()
	iter := self.store.NewIterator(nil)
	for iter.Next() {
		self.store.BatchDelete(iter.Key())
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		self.store.NewBatch() // reset the batch
		return err
	}
	return self.store.BatchCommit()
}

//Close state store
func (self *StateStore) Close() error {
	return self.store.Close()
}

func CheckStorage(dir string) error {
	path := dir + string(os.PathSeparator) + DBDirState
	db, err := leveldbstore.NewLevelDBStore(path)
	if err != nil {
		return err
	}
	defer db.Close()

	prefix := append([]byte{byte(scom.ST_STORAGE)}, utils.OntIDContractAddress[:]...) //prefix of new storage key
	flag := append(prefix, ontid.FIELD_VERSION)
	val, err := db.Get(flag)
	if err == nil {
		item := &states.StorageItem{}
		buf := bytes.NewBuffer(val)
		err := item.Deserialize(buf)
		if err == nil && item.Value[0] == ontid.FLAG_VERSION {
			return nil
		} else if err == nil {
			return errors.New("check ontid storage: invalid version flag")
		} else {
			return err
		}
	}

	prefix1 := []byte{byte(scom.ST_STORAGE), 0x2a, 0x64, 0x69, 0x64} //prefix of old storage key

	iter := db.NewIterator(prefix1)
	db.NewBatch()
	for ok := iter.First(); ok; ok = iter.Next() {
		key := append(prefix, iter.Key()[1:]...)
		db.BatchPut(key, iter.Value())
		db.BatchDelete(iter.Key())
	}
	iter.Release()
	err = iter.Error()
	if err != nil {
		return err
	}

	tag := states.StorageItem{}
	tag.Value = []byte{ontid.FLAG_VERSION}
	buf := bytes.NewBuffer(nil)
	tag.Serialize(buf)
	db.BatchPut(flag, buf.Bytes())
	err = db.BatchCommit()

	return err
}
