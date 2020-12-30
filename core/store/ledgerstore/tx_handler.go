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
	"fmt"
	"math"

	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/common/config"
	"github.com/ontio/dad-go/common/log"
	"github.com/ontio/dad-go/common/serialization"
	"github.com/ontio/dad-go/core/payload"
	"github.com/ontio/dad-go/core/states"
	"github.com/ontio/dad-go/core/store"
	scommon "github.com/ontio/dad-go/core/store/common"
	"github.com/ontio/dad-go/core/store/statestore"
	"github.com/ontio/dad-go/core/types"
	"github.com/ontio/dad-go/smartcontract"
	"github.com/ontio/dad-go/smartcontract/event"
	ninit "github.com/ontio/dad-go/smartcontract/service/native/init"
	"github.com/ontio/dad-go/smartcontract/service/native/ont"
	"github.com/ontio/dad-go/smartcontract/service/native/utils"
	"github.com/ontio/dad-go/smartcontract/service/neovm"
	"github.com/ontio/dad-go/smartcontract/storage"
)

//HandleDeployTransaction deal with smart contract deploy transaction
func (self *StateStore) HandleDeployTransaction(store store.LedgerStore, stateBatch *statestore.StateBatch,
	tx *types.Transaction, block *types.Block, eventStore scommon.EventStore) error {
	deploy := tx.Payload.(*payload.DeployCode)
	txHash := tx.Hash()
	address := types.AddressFromVmCode(deploy.Code)
	var (
		notifies []*event.NotifyEventInfo
		err      error
	)

	if tx.GasPrice != 0 {
		gas, overflow := common.SafeMul(tx.GasLimit, tx.GasPrice)
		if overflow {
			return fmt.Errorf("gaslimit:%d*gasprice:%d overflow!", tx.GasLimit, tx.GasPrice)
		}

		if err := isBalanceSufficient(tx.Payer, stateBatch, gas); err != nil {
			return err
		}

		cache := storage.NewCloneCache(stateBatch)

		// init smart contract configuration info
		config := &smartcontract.Config{
			Time:   block.Header.Timestamp,
			Height: block.Header.Height,
			Tx:     tx,
		}

		notifies, err = costGas(tx.Payer, gas, config, cache, store)
		if err != nil {
			return err
		}
		cache.Commit()
	}

	log.Infof("deploy contract address:%s", address.ToHexString())
	// store contract message
	err = stateBatch.TryGetOrAdd(scommon.ST_CONTRACT, address[:], deploy)
	if err != nil {
		return err
	}

	SaveNotify(eventStore, txHash, notifies, true)
	return nil
}

//HandleInvokeTransaction deal with smart contract invoke transaction
func (self *StateStore) HandleInvokeTransaction(store store.LedgerStore, stateBatch *statestore.StateBatch,
	tx *types.Transaction, block *types.Block, eventStore scommon.EventStore) error {
	invoke := tx.Payload.(*payload.InvokeCode)
	txHash := tx.Hash()
	code := invoke.Code
	sysTransFlag := bytes.Compare(code, ninit.COMMIT_DPOS_BYTES) == 0 || block.Header.Height == 0

	isCharge := !sysTransFlag && tx.GasPrice != 0

	gas, overflow := common.SafeMul(tx.GasLimit, tx.GasPrice)
	if overflow {
		return fmt.Errorf("gaslimit:%d*gasprice:%d overflow!", tx.GasLimit, tx.GasPrice)
	}
	if isCharge {
		if err := isBalanceSufficient(tx.Payer, stateBatch, gas); err != nil {
			return err
		}
	}

	// init smart contract configuration info
	config := &smartcontract.Config{
		Time:   block.Header.Timestamp,
		Height: block.Header.Height,
		Tx:     tx,
	}

	cache := storage.NewCloneCache(stateBatch)
	//init smart contract info
	sc := smartcontract.SmartContract{
		Config:     config,
		CloneCache: cache,
		Store:      store,
		Gas:        tx.GasLimit,
	}

	//start the smart contract executive function
	engine, err := sc.NewExecuteEngine(invoke.Code)
	if err != nil {
		return err
	}

	_, err = engine.Invoke()
	if err != nil {
		return err
	}

	var notifies []*event.NotifyEventInfo
	if isCharge {
		totalGas := tx.GasLimit - sc.Gas
		if totalGas < neovm.TRANSACTION_GAS {
			totalGas = neovm.TRANSACTION_GAS
		}
		notifies, err = costGas(tx.Payer, gas, config, sc.CloneCache, store)
		if err != nil {
			return err
		}
	}

	SaveNotify(eventStore, txHash, append(sc.Notifications, notifies...), true)
	sc.CloneCache.Commit()
	return nil
}

func SaveNotify(eventStore scommon.EventStore, txHash common.Uint256, notifies []*event.NotifyEventInfo, execSucc bool) error {
	if !config.DefConfig.Common.EnableEventLog {
		return nil
	}
	var notifyInfo *event.ExecuteNotify
	if execSucc {
		notifyInfo = &event.ExecuteNotify{TxHash: txHash,
			State: event.CONTRACT_STATE_SUCCESS, Notify: notifies}
	} else {
		notifyInfo = &event.ExecuteNotify{TxHash: txHash,
			State: event.CONTRACT_STATE_FAIL, Notify: notifies}
	}
	if err := eventStore.SaveEventNotifyByTx(txHash, notifyInfo); err != nil {
		return fmt.Errorf("SaveEventNotifyByTx error %s", err)
	}
	event.PushSmartCodeEvent(txHash, 0, event.EVENT_NOTIFY, notifyInfo)
	return nil
}

func genNativeTransferCode(from, to common.Address, value uint64) []byte {
	transfer := ont.Transfers{States: []*ont.State{{From: from, To: to, Value: value}}}
	tr := new(bytes.Buffer)
	transfer.Serialize(tr)
	return tr.Bytes()
}

// check whether payer ong balance sufficient
func isBalanceSufficient(payer common.Address, stateBatch *statestore.StateBatch, gas uint64) error {
	balance, err := getBalance(stateBatch, payer, utils.OngContractAddress)
	if err != nil {
		return err
	}
	if balance < gas {
		return fmt.Errorf("payer gas insufficient, need %d , only have %d", gas, balance)
	}
	return nil
}

func costGas(payer common.Address, gas uint64, config *smartcontract.Config,
	cache *storage.CloneCache, store store.LedgerStore) ([]*event.NotifyEventInfo, error) {

	params := genNativeTransferCode(payer, utils.GovernanceContractAddress, gas)

	sc := smartcontract.SmartContract{
		Config:     config,
		CloneCache: cache,
		Store:      store,
		Gas:        math.MaxUint64,
	}

	service, _ := sc.NewNativeService()
	_, err := service.NativeCall(utils.OngContractAddress, "transfer", params)
	if err != nil {
		return nil, err
	}
	return sc.Notifications, nil
}

func getBalance(stateBatch *statestore.StateBatch, address, contract common.Address) (uint64, error) {
	bl, err := stateBatch.TryGet(scommon.ST_STORAGE, append(contract[:], address[:]...))
	if err != nil {
		return 0, fmt.Errorf("get balance error:%s", err)
	}
	if bl == nil || bl.Value == nil {
		return 0, fmt.Errorf("get %s balance fail from %s", address.ToHexString(), contract.ToHexString())
	}
	item, ok := bl.Value.(*states.StorageItem)
	if !ok {
		return 0, fmt.Errorf("%s", "instance doesn't StorageItem!")
	}
	balance, err := serialization.ReadUint64(bytes.NewBuffer(item.Value))
	if err != nil {
		return 0, fmt.Errorf("read balance error:%s", err)
	}
	return balance, nil
}
